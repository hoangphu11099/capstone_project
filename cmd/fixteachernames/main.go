// Rename only unchanged demo teacher names, without resetting school data.
package main

import (
	"fmt"
	driver "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"student-management/models"
	"student-management/seed"
)

func main() {
	_ = godotenv.Load(".env")
	cfg := driver.NewConfig()
	cfg.User = os.Getenv("DB_USER")
	if cfg.User == "" {
		cfg.User = "root"
	}
	cfg.Passwd = os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "3306"
	}
	cfg.Net = "tcp"
	cfg.Addr = host + ":" + port
	cfg.DBName = os.Getenv("DB_NAME")
	if cfg.DBName == "" {
		cfg.DBName = "student_manager_db"
	}
	cfg.ParseTime = true
	db, err := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		panic("Database connection failed")
	}
	old := []string{"Nguyễn Minh Khang", "Trần Hoàng Anh", "Lê Thu Hà", "Phạm Quốc Bảo", "Võ Ngọc Linh"}
	changed := int64(0)
	err = db.Transaction(func(tx *gorm.DB) error {
		for i, name := range seed.DemoTeacherNames {
			result := tx.Model(&models.User{}).Where("username = ? AND full_name = ? AND id IN (SELECT user_id FROM teachers)", fmt.Sprintf("teacher%02d", i+1), old[i%5]).Update("full_name", name)
			if result.Error != nil {
				return result.Error
			}
			changed += result.RowsAffected
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Updated %d demo teacher names.\n", changed)
	var rows []struct{ TeacherCode, FullName, Username, Major string }
	err = db.Raw(`SELECT t.teacher_code,u.full_name,u.username,m.name AS major FROM teachers t JOIN users u ON u.id=t.user_id LEFT JOIN majors m ON m.id=t.major_id ORDER BY t.teacher_code`).Scan(&rows).Error
	if err != nil {
		panic(err)
	}
	for _, r := range rows {
		fmt.Printf("%s | %s | %s | %s\n", r.TeacherCode, r.FullName, r.Username, r.Major)
	}
	var wrong, conflicts int64
	db.Raw(`SELECT COUNT(*) FROM course_offerings o JOIN classes c ON c.id=o.class_id JOIN teachers t ON t.id=o.teacher_id WHERE t.major_id<>c.major_id`).Scan(&wrong)
	db.Raw(`SELECT COUNT(*) FROM schedules a JOIN schedules b ON a.id<b.id AND a.full_date=b.full_date AND a.start_time<b.end_time AND b.start_time<a.end_time JOIN course_offerings x ON x.id=a.course_offering_id JOIN course_offerings y ON y.id=b.course_offering_id WHERE a.full_date='2026-11-11' AND x.teacher_id=y.teacher_id`).Scan(&conflicts)
	fmt.Printf("Wrong-major offerings: %d; same-teacher overlaps on 2026-11-11: %d\n", wrong, conflicts)
}
