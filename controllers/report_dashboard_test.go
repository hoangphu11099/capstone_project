package controllers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http/httptest"
	"os"
	"strings"
	"student-management/config"
	"student-management/middleware"
	"student-management/models"
	"testing"
	"time"
)

func TestReportFilterValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, query := range []string{"classId=-1", "semesterId=x", "majorId=0", "cohort=1%20OR%201=1", "from=2026-02-30", "from=2026-12-01&to=2026-11-01"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/statistical-report?"+query, nil)
		if _, err := parseReportFilter(c); err == nil {
			t.Fatalf("accepted invalid query: %s", query)
		}
	}
	for _, role := range []string{"student", ""} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(middleware.ContextRoleKey, role)
		ViewStatisticalReport(c)
		if w.Code != 403 {
			t.Fatalf("role %s was not rejected", role)
		}
	}
}

func TestReportDashboardIntegration(t *testing.T) {
	if os.Getenv("RUN_REPORT_INTEGRATION") != "1" {
		t.Skip("set RUN_REPORT_INTEGRATION=1 for isolated MySQL report tests")
	}
	env, err := godotenv.Read("../.env")
	if err != nil {
		t.Fatal(err)
	}
	if env["DB_HOST"] != "localhost" && env["DB_HOST"] != "127.0.0.1" {
		t.Fatal("test requires local MySQL")
	}
	cfg := mysqlDriver.NewConfig()
	cfg.User = env["DB_USER"]
	cfg.Passwd = env["DB_PASSWORD"]
	cfg.Net = "tcp"
	cfg.Addr = env["DB_HOST"] + ":" + env["DB_PORT"]
	cfg.ParseTime = true
	cfg.Loc = attendanceLocation()
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	name := fmt.Sprintf("pp_report_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(name, "pp_report_test_") {
			if _, err := admin.Exec("DROP DATABASE `" + name + "`"); err != nil {
				t.Error(err)
			}
		}
	})
	cfg.DBName = name
	db, err := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	old := config.DB
	config.DB = db
	t.Cleanup(func() { config.DB = old; conn, _ := db.DB(); conn.Close() })
	config.AutoMigrateDB()
	create := func(value interface{}) {
		t.Helper()
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	date := func(value string) time.Time {
		d, err := time.ParseInLocation("2006-01-02", value, cfg.Loc)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	role := models.Role{Name: "teacher"}
	create(&role)
	user := func(name string) models.User {
		u := models.User{Username: name, Email: name + "@gmail.com", FullName: name, Password: "not-a-login-fixture", RoleID: role.ID}
		create(&u)
		return u
	}
	u1, u2 := user("teacher-one"), user("teacher-two")
	major := models.Major{Code: "WEB", Name: "Web"}
	create(&major)
	t1 := models.Teacher{UserID: u1.ID, TeacherCode: "T1", MajorID: major.ID, HireDate: date("2024-01-01")}
	t2 := models.Teacher{UserID: u2.ID, TeacherCode: "T2", MajorID: major.ID, HireDate: date("2024-01-01")}
	create(&t1)
	create(&t2)
	year := models.AcademicYear{Name: "2026–2027", StartDate: date("2026-09-01"), EndDate: date("2027-08-31")}
	create(&year)
	term1 := models.Semester{Name: "HK1 - K26", AcademicYearID: &year.ID, StartDate: date("2026-09-01"), EndDate: date("2027-02-28"), Status: "active"}
	term2 := models.Semester{Name: "HK2 - K26", AcademicYearID: &year.ID, StartDate: date("2027-03-01"), EndDate: date("2027-08-31"), Status: "upcoming"}
	create(&term1)
	create(&term2)
	cl1 := models.Class{ClassCode: "K26_WEB_01", MajorID: major.ID, CohortYear: 2026, MaxStudents: 20}
	cl2 := models.Class{ClassCode: "PRIVATE_CLASS", MajorID: major.ID, CohortYear: 2026, MaxStudents: 20}
	create(&cl1)
	create(&cl2)
	room := models.Room{Name: "A101", Capacity: 30}
	create(&room)
	c1 := models.Course{Code: "WEB001", Name: "Web", MajorID: major.ID}
	c2 := models.Course{Code: "WEB002", Name: "Private course", MajorID: major.ID}
	create(&c1)
	create(&c2)
	o1 := models.CourseOffering{ClassID: cl1.ID, CourseID: c1.ID, TeacherID: t1.ID, SemesterID: term1.ID, RoomID: room.ID}
	o2 := models.CourseOffering{ClassID: cl2.ID, CourseID: c2.ID, TeacherID: t2.ID, SemesterID: term1.ID, RoomID: room.ID}
	o3 := models.CourseOffering{ClassID: cl1.ID, CourseID: c1.ID, TeacherID: t1.ID, SemesterID: term2.ID, RoomID: room.ID}
	create(&o1)
	create(&o2)
	create(&o3)
	d1, d2 := date("2026-10-01"), date("2026-11-01")
	s1 := models.Schedule{ClassID: cl1.ID, CourseOfferingID: &o1.ID, FullDate: &d1, StartTime: "07:30", EndTime: "10:30"}
	s2 := models.Schedule{ClassID: cl1.ID, CourseOfferingID: &o1.ID, FullDate: &d2, StartTime: "07:30", EndTime: "10:30"}
	create(&s1)
	create(&s2)
	s3 := models.Schedule{ClassID: cl2.ID, CourseOfferingID: &o2.ID, FullDate: &d1, StartTime: "13:30", EndTime: "16:30"}
	create(&s3)
	confirmed := models.AttendanceSession{ClassID: cl1.ID, CourseID: c1.ID, TeacherID: t1.ID, CourseOfferingID: &o1.ID, ScheduleID: &s1.ID, ClassDate: d1, ExpiresAt: d1.Add(10 * time.Hour), Code: "confirmed", Status: "confirmed"}
	pending := models.AttendanceSession{ClassID: cl1.ID, CourseID: c1.ID, TeacherID: t1.ID, CourseOfferingID: &o1.ID, ClassDate: d2, ExpiresAt: d2.Add(10 * time.Hour), Code: "draft", Status: "pending"}
	create(&confirmed)
	create(&pending)
	enrollment := func(username string, cl models.Class, o models.CourseOffering, score float64, status string, cancelled bool) models.Enrollment {
		u := user(username)
		s := models.Student{StudentCode: username, UserID: u.ID, ClassID: cl.ID, DateOfBirth: date("2008-01-01"), EnrollmentDate: date("2026-09-01")}
		create(&s)
		e := models.Enrollment{StudentID: s.ID, ClassID: cl.ID, CourseID: o.CourseID, CourseOfferingID: &o.ID, EnrollDate: date("2026-09-01"), Status: "enrolled"}
		if cancelled {
			e.Status = "cancelled"
		}
		create(&e)
		g := models.Grade{EnrollmentID: e.ID, TotalScore: score, Score: 99, Status: status}
		create(&g)
		return e
	}
	e1 := enrollment("student-a", cl1, o1, 0, "approved", false)
	e2 := enrollment("student-b", cl1, o1, 80, "Approved", false)
	e3 := enrollment("student-c", cl1, o1, 100, "draft", false)
	create(&models.AcademicWarning{StudentID: e1.StudentID, SemesterID: term1.ID, GPA: 0, FailedCourses: 1, FailedCredits: 3, Reason: "Approved failing grade", Status: "active"})
	create(&models.Room{Name: "A102", Capacity: 30})
	planJSON, err := json.Marshal(map[string]interface{}{"schedules": []models.Schedule{s3}, "urgentException": true, "exceptionReason": "Urgent fixture"})
	if err != nil {
		t.Fatal(err)
	}
	create(&models.ClassOffer{ClassID: cl2.ID, CourseID: &c2.ID, SemesterID: &term1.ID, TeacherID: t2.ID, RoomID: &room.ID, OfferedByUserID: u2.ID, Status: "pending", PlanJSON: string(planJSON), OfferedAt: date("2026-09-29")})
	enrollment("cancelled", cl1, o1, 100, "approved", true)
	enrollment("PRIVATE_STUDENT", cl2, o2, 99, "approved", false)
	// Same student enrolled in the following semester: unique student count must remain 3.
	create(&models.Enrollment{StudentID: e1.StudentID, ClassID: cl1.ID, CourseID: c1.ID, CourseOfferingID: &o3.ID, EnrollDate: date("2027-02-01"), Status: "enrolled"})
	for _, record := range []models.Attendance{{EnrollmentID: e1.ID, AttendanceSessionID: &confirmed.ID, ClassDate: d1, Status: "absent"}, {EnrollmentID: e2.ID, AttendanceSessionID: &confirmed.ID, ClassDate: d1, Status: "present"}, {EnrollmentID: e3.ID, AttendanceSessionID: &confirmed.ID, ClassDate: d1, Status: "late"}, {EnrollmentID: e2.ID, AttendanceSessionID: &pending.ID, ClassDate: d2, Status: "absent"}} {
		v := record
		create(&v)
	}
	filter := reportFilter{IDs: map[string]uint{}}
	report, err := buildDashboardReport(db, filter, t1.ID)
	if err != nil {
		t.Fatal(err)
	}
	grades := report["grades"].(gin.H)
	if grades["approved"] != 2 || grades["draft"] != 1 || grades["averageScore"] != float64(40) || grades["passed"] != 1 {
		t.Fatalf("draft/cancelled/zero grade handling: %+v", grades)
	}
	if report["totalStudents"] != 3 || report["totalClasses"] != 1 || report["totalEnrollments"] != 4 {
		t.Fatalf("teacher scope or distinct counts: %+v", report)
	}
	attendance := report["attendance"].(map[string]int)
	if attendance["absent"] != 1 || attendance["present"] != 1 || attendance["late"] != 1 {
		t.Fatalf("unconfirmed attendance leaked: %+v", attendance)
	}
	if _, ok := report["assignments"]; ok {
		t.Fatal("teacher received admin assignment data")
	}
	if _, ok := report["academicWarnings"]; ok {
		t.Fatal("teacher received school-wide academic warnings")
	}
	if report["scheduleSummary"].(gin.H)["confirmed"] != 1 {
		t.Fatal("confirmed lessons were not counted separately")
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"PRIVATE_CLASS", "PRIVATE_STUDENT", "Private course", "teacher-two", "not-a-login-fixture"} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("teacher response leaked %s", private)
		}
	}
	wrongScope := reportFilter{IDs: map[string]uint{"classId": cl2.ID}}
	empty, err := buildDashboardReport(db, wrongScope, t1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if empty["totalEnrollments"] != 0 {
		t.Fatal("teacher accessed another teacher's class")
	}
	month := reportFilter{IDs: map[string]uint{}, From: "2026-11-01", To: "2026-11-01"}
	nov, err := buildDashboardReport(db, month, t1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if nov["attendance"].(map[string]int)["absent"] != 0 || nov["scheduleSummary"].(gin.H)["lessons"] != 1 {
		t.Fatal("date filter not applied or pending absence included")
	}
	comparison, err := buildDashboardReport(db, reportFilter{IDs: map[string]uint{"semesterId": term2.ID}}, t1.ID)
	if err != nil {
		t.Fatal(err)
	}
	cmp := comparison["comparison"].(gin.H)
	if cmp["available"] != true || cmp["enrollments"] != int64(3) || cmp["enrollmentChange"] != -2 {
		t.Fatalf("comparison wrong: %+v", cmp)
	}
	adminReport, err := buildDashboardReport(db, filter, 0)
	if err != nil {
		t.Fatal(err)
	}
	if adminReport["totalTeachers"] != int64(2) {
		t.Fatal("admin totals missing teacher")
	}
	if adminReport["totalWarnings"] != 1 {
		t.Fatalf("official academic warnings mixed with risk flags: %v", adminReport["totalWarnings"])
	}
	assignments := adminReport["assignments"].(gin.H)
	if len(assignments["pending"].([]gin.H)) != 1 || len(assignments["shortNotice"].([]gin.H)) != 1 {
		t.Fatalf("pending/urgent assignment missing: %+v", assignments)
	}
	rooms := adminReport["rooms"].([]gin.H)
	if len(rooms) != 2 {
		t.Fatal("unused room omitted")
	}
	for _, r := range rooms {
		if r["room"] == "A102" && r["lessons"] != 0 {
			t.Fatal("unused room has imaginary usage")
		}
	}

}
