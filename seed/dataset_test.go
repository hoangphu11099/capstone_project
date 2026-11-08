package seed

import (
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"strings"
	"student-management/config"
	"testing"
	"time"
)

func TestCalendarPlan(t *testing.T) {
	for _, value := range []string{"2026-11-02", "2027-01-31", "2027-02-01", "2027-09-01", "2028-02-29"} {
		date, err := time.ParseInLocation("2006-01-02", value, time.FixedZone("Asia/Ho_Chi_Minh", 7*3600))
		if err != nil {
			t.Fatal(err)
		}
		start, end, year, _ := termFor(date)
		if date.Before(start) || date.After(end) || year > date.Year() {
			t.Fatalf("invalid term for %s", value)
		}
		dates := weeklyDates(start, end, time.Monday, 8)
		for i, d := range dates {
			if d.Weekday() != time.Monday || d.Before(start) || d.After(end) || (i > 0 && d.Sub(dates[i-1]) != 7*24*time.Hour) {
				t.Fatal("invalid weekly plan")
			}
		}
	}
}

func TestDatasetIntegration(t *testing.T) {
	if os.Getenv("SEED_INTEGRATION") != "1" {
		t.Skip("set SEED_INTEGRATION=1 to test in an isolated MySQL schema")
	}
	env, err := godotenv.Read("../.env")
	if err != nil {
		t.Fatal(err)
	}
	cfg := mysql.NewConfig()
	cfg.User = env["DB_USER"]
	cfg.Passwd = env["DB_PASSWORD"]
	cfg.Net = "tcp"
	cfg.Addr = env["DB_HOST"] + ":" + env["DB_PORT"]
	cfg.ParseTime = true
	cfg.Loc = time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
	cfg.Params = map[string]string{"charset": "utf8mb4"}
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("__pp_seed_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer admin.Close()
		if strings.HasPrefix(name, "__pp_seed_test_") {
			if _, err := admin.Exec("DROP DATABASE `" + name + "`"); err != nil {
				t.Error(err)
			}
		}
	})
	cfg.DBName = name
	db, err := gorm.Open(gormmysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	defer conn.Close()
	previous := config.DB
	config.DB = db
	defer func() { config.DB = previous }()
	config.AutoMigrateDB()
	date := time.Date(2026, 11, 2, 0, 0, 0, 0, cfg.Loc)
	if err := SeedDatabase(db, Options{Now: date, Password: "TestOnly-SeedPass2026"}); err != nil {
		t.Fatal(err)
	}
	summary, err := Validate(db, date)
	if err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int64{"majors": 4, "classes": 24, "teachers": 20, "students": 480, "courses": 120, "auth_sessions": 0, "semesters": 12} {
		if summary.Counts[table] != want {
			t.Fatalf("%s: want %d got %d", table, want, summary.Counts[table])
		}
	}
	checks := []string{
		"SELECT COUNT(*) FROM students s JOIN users u ON u.id=s.user_id JOIN majors m ON m.id=s.major_id WHERE s.student_code<>CONCAT(YEAR(s.enrollment_date),s.training_system,m.code,LPAD(CAST(SUBSTRING(u.username,8) AS UNSIGNED),5,'0')) OR u.email NOT LIKE '%@ppacademy.edu.vn'",
		"SELECT COUNT(*) FROM semesters WHERE name NOT REGEXP '^HK[1-4] - K2[456]$'",
		"SELECT COUNT(*) FROM course_offerings o JOIN classes c ON c.id=o.class_id JOIN semesters s ON s.id=o.semester_id WHERE RIGHT(s.name,3)<>CONCAT('K',MOD(c.cohort_year,100))",
		"SELECT COUNT(*) FROM (SELECT c.id FROM classes c LEFT JOIN students s ON s.class_id=c.id GROUP BY c.id HAVING COUNT(s.id)<>20) x",
		"SELECT COUNT(*) FROM (SELECT major_id FROM courses GROUP BY major_id HAVING COUNT(*)<>30) x",
		"SELECT COUNT(*) FROM (SELECT cohort_year,major_id FROM classes GROUP BY cohort_year,major_id HAVING COUNT(*)<>2) x",
		"SELECT COUNT(*) FROM students s WHERE NOT EXISTS (SELECT 1 FROM enrollments e JOIN grades g ON g.enrollment_id=e.id WHERE e.student_id=s.id AND g.status='approved')",
		"SELECT COUNT(*) FROM schedules s JOIN classes c ON c.id=s.class_id WHERE c.cohort_year=2026 AND s.full_date>='2026-11-02' AND s.full_date<='2028-08-31' AND (DAYOFWEEK(s.full_date) NOT BETWEEN 2 AND 7 OR s.start_time<>'13:30' OR s.end_time<>'16:30')",
		"SELECT COUNT(*) FROM (SELECT c.id FROM classes c LEFT JOIN course_offerings o ON o.class_id=c.id LEFT JOIN class_offers f ON f.class_id=c.id AND f.status='pending' GROUP BY c.id HAVING COUNT(DISTINCT o.course_id)+COUNT(DISTINCT f.course_id)<>30) x",
	}
	for _, query := range checks {
		var count int64
		if err := db.Raw(query).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("school requirements failed (%d): %s", count, query)
		}
	}
	var demoLessons int64
	if err := db.Table("schedules s").Joins("JOIN classes c ON c.id=s.class_id").Where("c.cohort_year=2026 AND s.full_date>=? AND s.full_date<=?", date, "2028-08-31").Count(&demoLessons).Error; err != nil {
		t.Fatal(err)
	}
	end := time.Date(2028, 8, 31, 0, 0, 0, 0, date.Location())
	wantLessons := int64(0)
	from := date.AddDate(0, 0, 2)
	for index := 0; index < 8; index++ {
		for _, dates := range k26Dates(from, end, from, index) {
			wantLessons += int64(len(dates))
		}
	}
	if demoLessons != wantLessons {
		t.Fatalf("demo coverage: want %d lessons got %d", wantLessons, demoLessons)
	}
	if err := SeedDatabase(db, Options{Now: date, Password: "TestOnly-SeedPass2026"}); err == nil {
		t.Fatal("a second seed must not overwrite existing accounts")
	}
}
