package controllers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http/httptest"
	"os"
	"strings"
	"student-management/config"
	"student-management/models"
	"testing"
	"time"
)

func TestStudentRegisteredMajorIntegration(t *testing.T) {
	if os.Getenv("RUN_STUDENT_MAJOR_INTEGRATION") != "1" {
		t.Skip("set RUN_STUDENT_MAJOR_INTEGRATION=1 for isolated MySQL tests")
	}
	env, err := godotenv.Read("../.env")
	if err != nil {
		t.Fatal(err)
	}
	if env["DB_HOST"] != "localhost" && env["DB_HOST"] != "127.0.0.1" {
		t.Fatal("local MySQL required")
	}
	cfg := mysqlDriver.NewConfig()
	cfg.User = env["DB_USER"]
	cfg.Passwd = env["DB_PASSWORD"]
	cfg.Net = "tcp"
	cfg.Addr = env["DB_HOST"] + ":" + env["DB_PORT"]
	cfg.ParseTime = true
	cfg.Loc = attendanceLocation()
	conn, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	name := fmt.Sprintf("pp_student_major_test_%d", time.Now().UnixNano())
	if _, err := conn.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(name, "pp_student_major_test_") {
			if _, err := conn.Exec("DROP DATABASE `" + name + "`"); err != nil {
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
	t.Cleanup(func() { config.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	config.AutoMigrateDB()
	create := func(value interface{}) {
		t.Helper()
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	web, game := models.Major{Code: "WEB", Name: "Web"}, models.Major{Code: "GAME", Name: "Game"}
	create(&web)
	create(&game)
	webClass := models.Class{ClassCode: "WEB_01", MajorID: web.ID, MaxStudents: 10}
	gameClass := models.Class{ClassCode: "GAME_01", MajorID: game.ID, MaxStudents: 10}
	target := models.Class{ClassCode: "WEB_02", MajorID: web.ID, MaxStudents: 10}
	create(&webClass)
	create(&gameClass)
	create(&target)
	role := models.Role{Name: "student"}
	create(&role)
	student := func(name string, classID uint) models.Student {
		u := models.User{Username: name, Email: name + "@gmail.com", FullName: name, Password: "test-only", RoleID: role.ID}
		create(&u)
		s := models.Student{UserID: u.ID, StudentCode: name, ClassID: classID, EnrollmentDate: time.Now()}
		create(&s)
		return s
	}
	a, b := student("WEB_STUDENT", webClass.ID), student("GAME_STUDENT", gameClass.ID)
	if a.MajorID != web.ID || b.MajorID != game.ID {
		t.Fatal("admission did not persist registered major")
	}
	call := func(handler gin.HandlerFunc, url, id string, body interface{}, status int) *httptest.ResponseRecorder {
		t.Helper()
		encoded, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", url, bytes.NewReader(encoded))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: id}}
		handler(c)
		if w.Code != status {
			t.Fatalf("%s: expected %d got %d: %s", url, status, w.Code, w.Body.String())
		}
		return w
	}
	url := fmt.Sprintf("/students?majorId=%d", web.ID)
	result := call(ListStudents, url, "", nil, 200)
	if strings.Contains(result.Body.String(), "GAME_STUDENT") {
		t.Fatal("another major leaked into list")
	}
	call(ListStudents, "/students?majorId=bad", "", nil, 400)
	call(AddStudentsToClass, "/classes/students", fmt.Sprint(target.ID), AddStudentsToClassRequest{StudentIDs: []uint{a.ID, b.ID}}, 400)
	var fresh models.Student
	if err := db.First(&fresh, a.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.ClassID != webClass.ID {
		t.Fatal("mixed-major batch partially changed students")
	}
	call(AddStudentsToClass, "/classes/students", fmt.Sprint(target.ID), AddStudentsToClassRequest{StudentIDs: []uint{a.ID}}, 200)
	if err := db.First(&fresh, a.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.ClassID != target.ID || fresh.MajorID != web.ID {
		t.Fatal("same-major transfer changed registered major")
	}
	call(UpdateStudent, "/students/update", fmt.Sprint(a.ID), CreateStudentRequest{Username: "WEB_STUDENT", FullName: "Web student", Email: "WEB_STUDENT@gmail.com", ClassID: gameClass.ID}, 400)
	if err := db.Model(&models.Student{}).Where("id=?", a.ID).Update("class_id", nil).Error; err != nil {
		t.Fatal(err)
	}
	result = call(ListStudents, url, "", nil, 200)
	if !strings.Contains(result.Body.String(), "WEB_STUDENT") {
		t.Fatal("unassigned student lost registered major filter")
	}
	if err := db.Model(&models.Student{}).Where("id=?", b.ID).Update("major_id", nil).Error; err != nil {
		t.Fatal(err)
	}
	config.AutoMigrateDB()
	var backfill models.Student
	if err := db.First(&backfill, b.ID).Error; err != nil {
		t.Fatal(err)
	}
	if backfill.MajorID != game.ID {
		t.Fatal("legacy student's major was not initialized")
	}
	create(&models.User{Username: "student480", Email: "student480@gmail.com", FullName: "Previous student", Password: "test-only", RoleID: role.ID})
	call(GenerateStudentCode, "/students/generate-code", "", nil, 400)
	preview := call(GenerateStudentCode, fmt.Sprintf("/students/generate-code?majorId=%d&year=2026&trainingSystem=CQ&fullName=Nguyen+Minh+Anh", web.ID), "", nil, 200)
	var codeResult struct {
		Data struct {
			StudentCode string `json:"studentCode"`
			Username    string `json:"username"`
			Email       string `json:"email"`
		} `json:"data"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &codeResult); err != nil {
		t.Fatal(err)
	}
	code := codeResult.Data.StudentCode
	if code != "2026CQWEB00481" || codeResult.Data.Username != "student481" || codeResult.Data.Email != "nma.web00481@ppacademy.edu.vn" || !studentCodePattern.MatchString(code) {
		t.Fatalf("invalid random code: %s", code)
	}
	request := CreateStudentRequest{Username: codeResult.Data.Username, FullName: "Nguyen Minh Anh", Email: codeResult.Data.Email, ClassID: target.ID, MajorID: web.ID, StudentCode: code, Gender: "Nữ", TrainingSystem: "CQ", AutoIdentity: true, EnrollmentDate: "2026-11-08"}
	createdResponse := call(CreateStudent, "/students", "", request, 201)
	var created struct {
		Data            models.Student `json:"data"`
		DefaultPassword string         `json:"defaultPassword"`
	}
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Data.StudentCode != code || created.Data.MajorID != web.ID || created.Data.Gender != "Nữ" || len(created.DefaultPassword) < 12 {
		t.Fatalf("preview or admission data changed: %+v", created.Data)
	}
	var account models.User
	if err := db.First(&account, created.Data.UserID).Error; err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(created.DefaultPassword)) != nil || !account.FirstLogin {
		t.Fatal("default password or first-login behavior wrong")
	}
	var userCount int64
	db.Model(&models.User{}).Count(&userCount)
	call(CreateStudent, "/students", "", request, 409)
	var orphaned int64
	db.Model(&models.User{}).Count(&orphaned)
	if orphaned != userCount {
		t.Fatal("duplicate code left an orphaned account")
	}
	request.StudentCode = "<script>"
	call(CreateStudent, "/students", "", request, 400)
	request.StudentCode = ""
	request.MajorID = game.ID
	call(CreateStudent, "/students", "", request, 400)
	request.MajorID = web.ID
	request.Gender = "invalid"
	call(CreateStudent, "/students", "", request, 400)
	request.Gender = "Nam"
	request.AutoIdentity = false
	request.Username = "capacity-student"
	request.Email = "capacity-student@gmail.com"
	request.StudentCode = "SV001122334455"
	if err := db.Model(&target).Update("max_students", 1).Error; err != nil {
		t.Fatal(err)
	}
	call(CreateStudent, "/students", "", request, 400)
	request.ClassID = 0
	request.Username = "unassigned-admission"
	request.Email = "unassigned-admission@example.test"
	request.StudentCode = "2026CQWEB00999"
	response := call(CreateStudent, "/students", "", request, 201)
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Data.ClassID != 0 || created.Data.MajorID != web.ID {
		t.Fatal("admission must preserve the registered major without assigning a class")
	}
	var enrollments int64
	db.Model(&models.Enrollment{}).Where("student_id = ?", created.Data.ID).Count(&enrollments)
	if enrollments != 0 {
		t.Fatal("unassigned admission unexpectedly created enrollments")
	}
	call(UpdateStudent, "/students/"+fmt.Sprint(created.Data.ID), fmt.Sprint(created.Data.ID), request, 200)
	registered, err := registrationMajor(db, created.Data)
	if err != nil || registered.ID != web.ID {
		t.Fatal("student without a homeroom must register using the admission major", err)
	}
}

func TestStudentGenderOptions(t *testing.T) {
	for input, want := range map[string]string{"Nam": "Nam", "male": "Nam", "Nữ": "Nữ", "female": "Nữ", "nu": "Nữ", "": ""} {
		got, err := normalizeStudentGender(input)
		if err != nil || got != want {
			t.Fatalf("gender %q: got %q, %v", input, got, err)
		}
	}
	if _, err := normalizeStudentGender("other-value"); err == nil {
		t.Fatal("free-text gender accepted")
	}
}
