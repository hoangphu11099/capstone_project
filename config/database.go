package config

import (
	"fmt"
	"log"
	"os"

	"student-management/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB() {
	username := GetEnv("DB_USER", "root")
	password := GetEnv("DB_PASSWORD", "Ho@ngphu199")
	databaseName := GetEnv("DB_NAME", "student_manager_db")
	host := GetEnv("DB_HOST", "localhost")
	port := GetEnv("DB_PORT", "3306")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		username, password, host, port, databaseName)

	db, err := gorm.Open(mysql.New(mysql.Config{
		DriverName: "mysql",
		DSN:        dsn,
	}), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect database:", err)
	}

	DB = db
	fmt.Println("Database connected successfully!")
}

func AutoMigrateDB() {
	if DB == nil {
		log.Fatal("Database is not connected")
	}

	if err := DB.AutoMigrate(
		&models.Role{},
		&models.User{},
		&models.Teacher{},
		&models.Student{},
		&models.Major{},
		&models.Semester{},
		&models.Course{},
		&models.Room{},
		&models.Class{},
		&models.CourseOffering{},
		&models.Schedule{},
		&models.Enrollment{},
		&models.Grade{},
		&models.Transcript{},
		&models.AttendanceSession{},
		&models.Attendance{},
		&models.RoomRegister{},
		&models.CourseRegistration{},
		&models.ClassTeacher{},
		&models.Notification{},
		&models.ClassOffer{},
		&models.AdminDashboard{},
		&models.Dashboard{},
		&models.ExamSchedule{},
		&models.Exercise{},
		&models.Submission{},
		&models.AcademicWarning{},
		&models.GradeApproval{},
	); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}
}

func GetEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
