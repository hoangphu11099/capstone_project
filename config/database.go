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

	// Replace the old one-schedule-per-offering constraint without removing rows.
	if DB.Migrator().HasTable(&models.Schedule{}) {
		indexes, err := DB.Migrator().GetIndexes(&models.Schedule{})
		if err != nil {
			log.Fatal("Failed to inspect schedule indexes:", err)
		}
		for _, index := range indexes {
			unique, _ := index.Unique()
			columns := index.Columns()
			if unique && len(columns) == 1 && columns[0] == "course_offering_id" {
				// MySQL requires an index for the foreign key while the unique index is removed.
				if !DB.Migrator().HasIndex(&models.Schedule{}, "idx_schedule_offering_lookup") {
					if err := DB.Exec("CREATE INDEX idx_schedule_offering_lookup ON schedules (course_offering_id)").Error; err != nil {
						log.Fatal(err)
					}
				}
				if err := DB.Migrator().DropIndex(&models.Schedule{}, index.Name()); err != nil {
					log.Fatal(err)
				}
			}
		}
	}
	if err := DB.AutoMigrate(
		&models.Role{},
		&models.User{},
		&models.AuthSession{},
		&models.Teacher{},
		&models.Student{},
		&models.Major{},
		&models.AcademicYear{},
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
		&models.SiteSetting{},
		&models.SchoolPost{},
		&models.Consultation{},
		&models.ProgramImage{},
		&models.PublicProgram{},
	); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}
	// Backfill once for legacy students; never overwrite an already registered major.
	if err := DB.Exec("UPDATE students s JOIN classes c ON c.id=s.class_id SET s.major_id=c.major_id WHERE (s.major_id IS NULL OR s.major_id=0) AND c.major_id>0").Error; err != nil {
		log.Fatal("Failed to initialize student registered majors:", err)
	}
}

func GetEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
