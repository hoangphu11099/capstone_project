package seed

import (
	"fmt"
	"math/rand"
	"time"

	"student-management/config"
	"student-management/models"

	"golang.org/x/crypto/bcrypt"
)

func SeedData() {
	db := config.DB
	rand.Seed(time.Now().UnixNano())

	// ----------------------
	// 1. Tạo bảng
	// ----------------------
	db.AutoMigrate(
		&models.User{},
		&models.Teacher{},
		&models.Student{},
		&models.Class{},
		&models.Course{},
		&models.Grade{},
	)

	// ----------------------
	// 2. Seed ADMIN
	// ----------------------

	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	db.Create(&models.User{
		Username:   "admin",
		Password:   string(hash),
		Role:       "admin",
		FirstLogin: true,
	})
	// ----------------------
	// 3. Seed 20 giáo viên
	// ----------------------
	teachers := []models.Teacher{}
	for i := 1; i <= 20; i++ {
		t := models.Teacher{
			TeacherCode: fmt.Sprintf("GV%03d", i),
			FullName:    fmt.Sprintf("Teacher %d", i),
			Department:  "General",
		}
		db.Create(&t)
		teachers = append(teachers, t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
		db.Create(&models.User{
			Username:   "teacher" + fmt.Sprintf("%02d", i),
			Password:   string(hash),
			Role:       "teacher",
			TeacherID:  &t.ID,
			FirstLogin: true,
		})
	}

	// ----------------------
	// 4. Seed 24 lớp + 480 học sinh K24/K25/K26
	// ----------------------
	khoas := []string{"K24", "K25", "K26"}
	classes := []models.Class{}
	for _, khoa := range khoas {
		for i := 1; i <= 8; i++ {
			class := models.Class{
				ClassName: fmt.Sprintf("%s_L%02d", khoa, i),
				Major:     "",
				TeacherID: teachers[rand.Intn(len(teachers))].ID,
			}
			db.Create(&class)
			classes = append(classes, class)

			// 20 học sinh mỗi lớp
			for j := 1; j <= 20; j++ {
				student := models.Student{
					StudentCode: fmt.Sprintf("%s_S%03d%02d", khoa, i, j),
					FullName:    fmt.Sprintf("Student %s-%d-%d", khoa, i, j),
					Gender:      "Nam",
					ClassID:     class.ID,
				}
				db.Create(&student)

				hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
				db.Create(&models.User{
					Username:   fmt.Sprintf("%s_user%03d%02d", khoa, i, j),
					Password:   string(hashedPassword),
					Role:       "student",
					StudentID:  &student.ID,
					FirstLogin: true,
				})
			}
		}
	}

	// ----------------------
	// 5. Seed 4 chuyên ngành + 30 môn mỗi ngành
	// ----------------------
	majorCodeMap := map[string]string{
		"Lập trình web":   "LWC",
		"Lập trình game":  "LGC",
		"Marketing":       "MKC",
		"Thiết kế đồ họa": "TDH",
	}
	majors := []string{"Lập trình web", "Lập trình game", "Marketing", "Thiết kế đồ họa"}
	courses := []models.Course{}

	for _, major := range majors {
		for i := 1; i <= 30; i++ {
			c := models.Course{
				CourseCode: fmt.Sprintf("%s%02d", majorCodeMap[major], i),
				CourseName: fmt.Sprintf("%s Course %02d", major, i),
				Credits:    rand.Intn(4) + 1,
				TeacherID:  teachers[rand.Intn(len(teachers))].ID,
			}
			db.Create(&c)
			courses = append(courses, c)
		}
	}

	// ----------------------
	// 6. Chia điểm cho học sinh
	// ----------------------
	var allStudents []models.Student
	db.Find(&allStudents)
	for _, student := range allStudents {
		for _, course := range courses {
			grade := models.Grade{
				StudentID: student.ID,
				CourseID:  course.ID,
				Score:     5 + rand.Float64()*5,
			}
			db.Create(&grade)
		}
	}

	// ----------------------
	// 7. Gán học kỳ, buổi, thứ cho lớp
	// ----------------------
	semesters := []string{"HK1", "HK2", "HK3", "HK4"}
	timeslots := []string{"Sáng", "Chiều", "Tối"}
	weekdays := []string{"Thứ 2", "Thứ 3", "Thứ 4", "Thứ 5", "Thứ 6"}

	for i, class := range classes {
		class.Major = majors[i%len(majors)]
		class.Semester = semesters[i%len(semesters)]
		class.TimeSlot = timeslots[i%len(timeslots)]
		class.Weekday = weekdays[i%len(weekdays)]
		db.Save(&class)
	}

	fmt.Println("Seeding final completed: 20 teachers, 480 students, 24 classes (K24/K25/K26), 4 majors, 30 courses each, grades assigned, classes scheduled!")
}
