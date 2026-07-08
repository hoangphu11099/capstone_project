package seed

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"student-management/config"
	"student-management/models"

	"golang.org/x/crypto/bcrypt"
)

func SeedData() {
	db := config.DB
	rand.Seed(time.Now().UnixNano())

	db.AutoMigrate(
		&models.Role{},
		&models.User{},
		&models.Teacher{},
		&models.Student{},
		&models.Major{},
		&models.Semester{},
		&models.Course{},
		&models.Room{},
		&models.Class{},
		&models.Schedule{},
		&models.Enrollment{},
		&models.Grade{},
		&models.Transcript{},
		&models.AttendanceSession{},
		&models.Attendance{},
		&models.RoomRegister{},
		&models.ExamSchedule{},
		&models.Exercise{},
		&models.Submission{},
		&models.AcademicWarning{},
		&models.GradeApproval{},
	)

	// =====================
	// 1. ROLES
	// =====================

	roles := []models.Role{
		{Name: "admin", Description: "Quản trị hệ thống"},
		{Name: "teacher", Description: "Giáo viên"},
		{Name: "student", Description: "Học sinh"},
	}

	for _, role := range roles {
		db.FirstOrCreate(&role, models.Role{Name: role.Name})
	}

	// =====================
	// 2. ADMIN USER
	// =====================

	admin := models.User{
		Username: "admin",
		Password: hashPassword("123456"),
		Email:    "admin@gmail.com",
		FullName: "Quản trị viên",
		RoleID:   1,
		IsActive: true,
	}

	db.FirstOrCreate(&admin, models.User{Username: "admin"})

	// =====================
	// 3. MAJORS
	// =====================

	majors := []models.Major{
		{Name: "Lập trình web", Code: "WEB"},
		{Name: "Lập trình game", Code: "GAME"},
		{Name: "Marketing", Code: "MKT"},
		{Name: "Thiết kế đồ họa", Code: "DESIGN"},
	}

	for _, major := range majors {
		db.FirstOrCreate(&major, models.Major{Code: major.Code})
	}

	// =====================
	// 4. SEMESTERS
	// =====================

	semesters := []models.Semester{
		{
			Name:      "HK1",
			StartDate: time.Date(2024, 9, 1, 0, 0, 0, 0, time.Local),
			EndDate:   time.Date(2025, 2, 28, 0, 0, 0, 0, time.Local),
			Status:    "closed",
		},
		{
			Name:      "HK2",
			StartDate: time.Date(2025, 3, 1, 0, 0, 0, 0, time.Local),
			EndDate:   time.Date(2025, 8, 31, 0, 0, 0, 0, time.Local),
			Status:    "closed",
		},
		{
			Name:      "HK3",
			StartDate: time.Date(2025, 9, 1, 0, 0, 0, 0, time.Local),
			EndDate:   time.Date(2026, 2, 28, 0, 0, 0, 0, time.Local),
			Status:    "closed",
		},
		{
			Name:      "HK4",
			StartDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local),
			EndDate:   time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local),
			Status:    "active",
		},
	}

	for _, semester := range semesters {
		db.Where("name = ?", semester.Name).
			Assign(models.Semester{
				StartDate: semester.StartDate,
				EndDate:   semester.EndDate,
				Status:    semester.Status,
			}).
			FirstOrCreate(&semester)
	}

	// =====================
	// 5. TEACHERS + USERS
	// 20 GIÁO VIÊN
	// =====================

	for i := 1; i <= 20; i++ {
		username := fmt.Sprintf("teacher%02d", i)

		user := models.User{
			Username: username,
			Password: hashPassword("123456"),
			Email:    fmt.Sprintf("%s@gmail.com", username),
			FullName: fmt.Sprintf("Giáo viên %02d", i),
			RoleID:   2,
			IsActive: true,
		}

		db.FirstOrCreate(&user, models.User{Username: username})

		teacher := models.Teacher{
			TeacherCode:   fmt.Sprintf("TCH%03d", i),
			UserID:        user.ID,
			Phone:         fmt.Sprintf("090000%04d", i),
			Address:       "TP.HCM",
			Qualification: "Cử nhân",
			HireDate:      time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local),
		}

		db.FirstOrCreate(&teacher, models.Teacher{TeacherCode: teacher.TeacherCode})
	}

	// =====================
	// 6. ROOMS
	// 10 PHÒNG
	// =====================

	rooms := []models.Room{
		{Name: "A101", Building: "Tòa A", Capacity: 40, Description: "Phòng học lý thuyết"},
		{Name: "A102", Building: "Tòa A", Capacity: 40, Description: "Phòng học lý thuyết"},
		{Name: "A201", Building: "Tòa A", Capacity: 35, Description: "Phòng máy tính thực hành"},
		{Name: "A202", Building: "Tòa A", Capacity: 35, Description: "Phòng máy tính thực hành"},

		{Name: "B101", Building: "Tòa B", Capacity: 45, Description: "Phòng học Marketing"},
		{Name: "B102", Building: "Tòa B", Capacity: 45, Description: "Phòng học Marketing"},
		{Name: "B201", Building: "Tòa B", Capacity: 30, Description: "Phòng thiết kế đồ họa"},
		{Name: "B202", Building: "Tòa B", Capacity: 30, Description: "Phòng thiết kế đồ họa"},

		{Name: "C101", Building: "Tòa C", Capacity: 50, Description: "Phòng học chung"},
		{Name: "C201", Building: "Tòa C", Capacity: 60, Description: "Hội trường / phòng học lớn"},
	}

	for _, room := range rooms {
		room.IsActive = true
		db.FirstOrCreate(&room, models.Room{Name: room.Name})
	}

	// =====================
	// 7. COURSES
	// 4 CHUYÊN NGÀNH × 30 MÔN
	// =====================

	courseGroups := map[uint][]string{
		1: {
			"Lập trình căn bản", "C Programming", "Cấu trúc dữ liệu", "Giải thuật",
			"Lập trình hướng đối tượng", "Cơ sở dữ liệu", "SQL Server", "MySQL",
			"HTML CSS căn bản", "JavaScript căn bản", "Lập trình Web Frontend",
			"Lập trình Web Backend", "ReactJS", "NodeJS", "PHP Laravel",
			"Java Spring Boot", "RESTful API", "Git và GitHub", "Linux căn bản",
			"Mạng máy tính", "An toàn thông tin", "Kiểm thử phần mềm",
			"Phân tích thiết kế hệ thống", "UML", "Quản lý dự án phần mềm",
			"Điện toán đám mây", "DevOps căn bản", "Docker căn bản",
			"Đồ án Web 1", "Đồ án Web 2",
		},
		2: {
			"Nhập môn phát triển Game", "Game Design căn bản", "C# Programming",
			"Unity căn bản", "Unity nâng cao", "Lập trình Gameplay",
			"Lập trình Game 2D", "Lập trình Game 3D", "Thiết kế nhân vật Game",
			"Thiết kế màn chơi", "Game Physics", "Game AI", "Animation trong Game",
			"Âm thanh trong Game", "UI UX cho Game", "Blender căn bản",
			"3D Modeling", "Texture và Material", "Lighting trong Game",
			"Mobile Game Development", "Multiplayer Game", "VR AR Game",
			"Game Testing", "Game Optimization", "Game Publishing",
			"Storytelling trong Game", "Monetization trong Game",
			"Đồ án Game 1", "Đồ án Game 2", "Đồ án tốt nghiệp Game",
		},
		3: {
			"Marketing căn bản", "Hành vi khách hàng", "Nghiên cứu thị trường",
			"Digital Marketing", "Content Marketing", "SEO căn bản",
			"Google Ads", "Facebook Ads", "TikTok Marketing", "Email Marketing",
			"Social Media Marketing", "Brand Management", "Marketing Strategy",
			"Trade Marketing", "PR và Truyền thông", "Copywriting",
			"Quản trị bán hàng", "Chăm sóc khách hàng", "CRM", "E-commerce",
			"Phân tích dữ liệu Marketing", "Marketing Automation",
			"Thiết kế chiến dịch quảng cáo", "Kỹ năng thuyết trình",
			"Kỹ năng đàm phán", "Quản trị sự kiện", "Kênh phân phối",
			"Đồ án Marketing 1", "Đồ án Marketing 2", "Đồ án tốt nghiệp Marketing",
		},
		4: {
			"Nguyên lý thiết kế", "Mỹ thuật cơ bản", "Hình họa", "Màu sắc học",
			"Bố cục trong thiết kế", "Typography", "Adobe Photoshop",
			"Adobe Illustrator", "Adobe InDesign", "Thiết kế nhận diện thương hiệu",
			"Thiết kế Logo", "Thiết kế Poster", "Thiết kế Bao bì",
			"Thiết kế Ấn phẩm truyền thông", "Nhiếp ảnh căn bản",
			"Chỉnh sửa ảnh", "Digital Painting", "UI Design", "UX Design",
			"Figma căn bản", "Thiết kế Web Layout", "Motion Graphic",
			"After Effects", "Premiere Pro", "Thiết kế 3D căn bản",
			"Blender căn bản", "Portfolio Design", "Đồ án Thiết kế 1",
			"Đồ án Thiết kế 2", "Đồ án tốt nghiệp Thiết kế",
		},
	}

	courseIndex := 1

	for majorID, courseNames := range courseGroups {
		for i, courseName := range courseNames {
			course := models.Course{
				Code:       fmt.Sprintf("C%03d", courseIndex),
				Name:       courseName,
				Credits:    rand.Intn(2) + 2,
				MajorID:    majorID,
				SemesterID: uint(i%4 + 1),
				IsActive:   true,
			}

			db.FirstOrCreate(&course, models.Course{Code: course.Code})
			courseIndex++
		}
	}
	// =====================
	// 8. CLASSES
	// 24 LỚP
	// K24, K25, K26
	// Mỗi khóa 8 lớp
	// Mỗi chuyên ngành 2 lớp
	// =====================

	khoas := []string{"K24", "K25", "K26"}
	majorCodes := []string{"WEB", "GAME", "MKT", "DESIGN"}

	classCount := 1

	for _, khoa := range khoas {
		for majorIndex, majorCode := range majorCodes {
			for lop := 1; lop <= 2; lop++ {
				classCode := fmt.Sprintf("%s_%s_%02d", khoa, majorCode, lop)

				class := models.Class{
					ClassCode:   classCode,
					MajorID:     uint(majorIndex + 1),
					TeacherID:   uint((classCount-1)%20 + 1),
					SemesterID:  uint((classCount-1)%4 + 1),
					RoomID:      uint((classCount-1)%10 + 1),
					MaxStudents: 20,
					Status:      "open",
				}

				db.FirstOrCreate(&class, models.Class{ClassCode: classCode})

				classCount++
			}
		}
	}

	// =====================
	// 9. STUDENTS + USERS
	// 480 HỌC SINH
	// 20 HỌC SINH / 1 LỚP
	// =====================

	studentIndex := 1

	for classID := 1; classID <= 24; classID++ {
		for j := 1; j <= 20; j++ {
			username := fmt.Sprintf("student%03d", studentIndex)

			user := models.User{
				Username: username,
				Password: hashPassword("123456"),
				Email:    fmt.Sprintf("%s@gmail.com", username),
				FullName: fmt.Sprintf("Học sinh %03d", studentIndex),
				RoleID:   3,
				IsActive: true,
			}

			db.FirstOrCreate(&user, models.User{Username: username})

			student := models.Student{
				StudentCode:    fmt.Sprintf("STD%03d", studentIndex),
				UserID:         user.ID,
				ClassID:        uint(classID),
				DateOfBirth:    time.Date(2005, time.Month(rand.Intn(12)+1), rand.Intn(28)+1, 0, 0, 0, 0, time.Local),
				Gender:         randomGender(),
				Phone:          fmt.Sprintf("091%07d", studentIndex),
				Address:        "Việt Nam",
				EnrollmentDate: time.Date(2024, 9, 1, 0, 0, 0, 0, time.Local),
				Status:         "active",
			}

			db.FirstOrCreate(&student, models.Student{StudentCode: student.StudentCode})

			studentIndex++
		}
	}

	// =====================
	// 10. SCHEDULES
	// 24 LỚP CHIA ĐỀU SÁNG / CHIỀU / TỐI
	// =====================

	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	sessions := []struct {
		Name      string
		StartTime string
		EndTime   string
	}{
		{"Sáng", "07:30", "09:30"},
		{"Chiều", "13:30", "15:30"},
		{"Tối", "18:00", "20:00"},
	}

	for classID := 1; classID <= 24; classID++ {
		session := sessions[(classID-1)%3]

		schedule := models.Schedule{
			ClassID:   uint(classID),
			DayOfWeek: days[(classID-1)%len(days)],
			Session:   session.Name,
			StartTime: session.StartTime,
			EndTime:   session.EndTime,
		}

		db.Create(&schedule)
	}

	// =====================
	// 11. ENROLLMENTS
	// Mỗi học sinh học các môn theo chuyên ngành + học kỳ
	// =====================

	var students []models.Student
	db.Find(&students)

	for _, student := range students {
		var class models.Class
		db.First(&class, student.ClassID)

		var courses []models.Course
		db.Where("major_id = ? AND semester_id = ?", class.MajorID, class.SemesterID).Find(&courses)

		for _, course := range courses {
			enrollment := models.Enrollment{
				StudentID:  uint(student.ID),
				ClassID:    uint(class.ID),
				CourseID:   uint(course.ID),
				EnrollDate: time.Now(),
				Status:     "enrolled",
			}

			db.Create(&enrollment)

			assignmentScore := randomScore()
			midtermScore := randomScore()
			finalScore := randomScore()
			totalScore := roundScore(assignmentScore*0.3 + midtermScore*0.3 + finalScore*0.4)

			gradeLetter := convertLetterGrade(totalScore)

			grade := models.Grade{
				EnrollmentID:    enrollment.ID,
				AssignmentScore: assignmentScore,
				MidtermScore:    midtermScore,
				FinalScore:      finalScore,
				TotalScore:      totalScore,
				GradeLetter:     gradeLetter,
				Status:          "Approved",
				Remark:          "Đã có điểm",
				Score:           totalScore,
				LetterGrade:     gradeLetter,
			}

			db.Create(&grade)
		}
	}
	// =====================
	// 12. ROOM REGISTERS
	// Mỗi lớp đăng ký phòng học theo lịch học
	// =====================

	var classes []models.Class
	db.Find(&classes)

	for _, class := range classes {
		roomRegister := models.RoomRegister{
			ClassID: class.ID,
			RoomID:  class.RoomID,
			Date:    time.Now(),
			Time:    "Theo lịch học của lớp",
			Note:    "Đăng ký phòng học cho lớp " + class.ClassCode,
		}

		db.Create(&roomRegister)
	}
	// =====================
	// =====================
	// ATTENDANCE
	// =====================

	var enrollments []models.Enrollment
	db.Find(&enrollments)

	statuses := []string{"present", "present", "present", "absent", "late", "excused"}

	for _, enrollment := range enrollments {
		for i := 0; i < 5; i++ {
			status := statuses[rand.Intn(len(statuses))]

			note := "Đi học đầy đủ"
			if status == "absent" {
				note = "Vắng không phép"
			} else if status == "late" {
				note = "Đi trễ"
			} else if status == "excused" {
				note = "Vắng có phép"
			}

			attendance := models.Attendance{
				EnrollmentID: enrollment.ID,
				ClassDate:    time.Now().AddDate(0, 0, -i*7),
				Status:       status,
				Note:         note,
			}

			db.Create(&attendance)
		}
	}

	fmt.Println("Seed data completed successfully!")
}

func randomGender() string {
	genders := []string{"Male", "Female"}
	return genders[rand.Intn(len(genders))]
}

func randomScore() float64 {
	return float64(rand.Intn(51) + 50)
}

func roundScore(score float64) float64 {
	return math.Round(score*100) / 100
}

func convertLetterGrade(score float64) string {
	if score >= 85 {
		return "A"
	}

	if score >= 80 {
		return "B+"
	}

	if score >= 70 {
		return "B"
	}

	if score >= 65 {
		return "C+"
	}

	if score >= 55 {
		return "C"
	}

	if score >= 50 {
		return "D+"
	}

	if score >= 40 {
		return "D"
	}

	return "F"
}
func hashPassword(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(hash)
}
