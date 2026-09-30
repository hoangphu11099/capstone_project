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
	// Dùng seed cố định để dữ liệu mẫu ổn định giữa các lần khởi tạo.
	rand.Seed(20260825)

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
		&models.CourseOffering{},
		&models.Schedule{},
		&models.Enrollment{},
		&models.CourseRegistration{},
		&models.ClassTeacher{},
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

	roleIDs := make(map[string]uint, len(roles))
	for _, roleName := range []string{"admin", "teacher", "student"} {
		var role models.Role
		db.Where("name = ?", roleName).First(&role)
		roleIDs[roleName] = role.ID
	}

	// =====================
	// 2. ADMIN USER
	// =====================

	admin := models.User{
		Username: "admin",
		Password: hashPassword("123456"),
		Email:    "admin@gmail.com",
		FullName: "Quản trị viên",
		RoleID:   roleIDs["admin"],
		IsActive: true,
	}

	db.Where("username = ?", admin.Username).Assign(admin).FirstOrCreate(&admin)

	// =====================
	// 3. MAJORS
	// =====================

	majors := []models.Major{
		{Name: "Lập trình web", Code: "WEB", IsActive: true},
		{Name: "Lập trình game", Code: "GAME", IsActive: true},
		{Name: "Marketing", Code: "MKT", IsActive: true},
		{Name: "Thiết kế đồ họa", Code: "DESIGN", IsActive: true},
	}

	majorByCode := make(map[string]models.Major, len(majors))
	for _, majorData := range majors {
		major := majorData
		db.Where("code = ?", major.Code).Assign(majorData).FirstOrCreate(&major)
		majorByCode[major.Code] = major
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

	semesterByName := make(map[string]models.Semester, len(semesters))
	for _, semesterData := range semesters {
		var semester models.Semester
		db.Where("name = ?", semesterData.Name).First(&semester)
		semesterByName[semester.Name] = semester
	}

	// =====================
	// 5. TEACHERS + USERS
	// 20 GIÁO VIÊN
	// Mỗi chuyên ngành có 5 giáo viên, không gán chéo ngành.
	// =====================

	majorCodes := []string{"WEB", "GAME", "MKT", "DESIGN"}

	for i := 1; i <= 20; i++ {
		username := fmt.Sprintf("teacher%02d", i)
		majorCode := majorCodes[(i-1)/5]
		major := majorByCode[majorCode]

		user := models.User{
			Username: username,
			Password: hashPassword("123456"),
			Email:    fmt.Sprintf("%s@gmail.com", username),
			FullName: fmt.Sprintf("Giáo viên %s %02d", majorCode, (i-1)%5+1),
			RoleID:   roleIDs["teacher"],
			IsActive: true,
		}

		db.Where("username = ?", username).Assign(user).FirstOrCreate(&user)

		teacher := models.Teacher{
			TeacherCode:   fmt.Sprintf("TCH%03d", i),
			UserID:        user.ID,
			Phone:         fmt.Sprintf("090000%04d", i),
			Address:       "TP.HCM",
			Qualification: "Cử nhân",
			HireDate:      time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local),
			MajorID:       major.ID,
		}

		db.Where("teacher_code = ?", teacher.TeacherCode).Assign(teacher).FirstOrCreate(&teacher)
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
		db.Where("name = ?", room.Name).Assign(room).FirstOrCreate(&room)
	}

	// =====================
	// 7. COURSES
	// 4 CHUYÊN NGÀNH × 30 MÔN
	// =====================

	courseGroups := map[string][]string{
		"WEB": {
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
		"GAME": {
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
		"MKT": {
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
		"DESIGN": {
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

	for _, majorCode := range majorCodes {
		major := majorByCode[majorCode]
		courseNames := courseGroups[majorCode]
		for i, courseName := range courseNames {
			semester := semesterByName[semesterNameForCourseIndex(i)]
			course := models.Course{
				Code:       fmt.Sprintf("C%03d", courseIndex),
				Name:       courseName,
				Credits:    rand.Intn(2) + 2,
				MajorID:    major.ID,
				SemesterID: semester.ID,
				IsActive:   true,
			}

			db.Where("code = ?", course.Code).Assign(course).FirstOrCreate(&course)
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
	activeSemester := semesterByName["HK4"]
	classCount := 1
	seededClassCodes := make([]string, 0, 24)

	for _, khoa := range khoas {
		for _, majorCode := range majorCodes {
			major := majorByCode[majorCode]
			var majorTeachers []models.Teacher
			db.Where("major_id = ?", major.ID).Order("teacher_code").Find(&majorTeachers)
			if len(majorTeachers) == 0 {
				panic("Không có giáo viên cho ngành " + majorCode)
			}

			for lop := 1; lop <= 2; lop++ {
				classCode := fmt.Sprintf("%s_%s_%02d", khoa, majorCode, lop)
				seededClassCodes = append(seededClassCodes, classCode)
				teacher := majorTeachers[(classCount-1)%len(majorTeachers)]
				room := rooms[(classCount-1)%len(rooms)]
				var savedRoom models.Room
				db.Where("name = ?", room.Name).First(&savedRoom)

				class := models.Class{
					ClassCode:   classCode,
					MajorID:     major.ID,
					TeacherID:   teacher.ID,
					SemesterID:  activeSemester.ID,
					RoomID:      savedRoom.ID,
					MaxStudents: 20,
					Status:      "open",
				}

				db.Where("class_code = ?", classCode).Assign(class).FirstOrCreate(&class)

				classTeacher := models.ClassTeacher{
					ClassID:          class.ID,
					TeacherID:        teacher.ID,
					AssignedByUserID: admin.ID,
					Status:           "active",
					AssignedAt:       time.Now(),
					Note:             "Cố vấn lớp thuộc ngành " + majorCode,
				}
				db.Where("class_id = ? AND teacher_id = ? AND status = ?", class.ID, teacher.ID, "active").
					Assign(classTeacher).FirstOrCreate(&classTeacher)

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
	var seededClasses []models.Class
	db.Where("class_code IN ?", seededClassCodes).Order("class_code").Find(&seededClasses)
	seededStudentCodes := make([]string, 0, len(seededClasses)*20)

	for _, class := range seededClasses {
		for j := 1; j <= 20; j++ {
			username := fmt.Sprintf("student%03d", studentIndex)
			studentCode := fmt.Sprintf("STD%03d", studentIndex)
			seededStudentCodes = append(seededStudentCodes, studentCode)

			user := models.User{
				Username: username,
				Password: hashPassword("123456"),
				Email:    fmt.Sprintf("%s@gmail.com", username),
				FullName: fmt.Sprintf("Học sinh %03d", studentIndex),
				RoleID:   roleIDs["student"],
				IsActive: true,
			}

			db.Where("username = ?", username).Assign(user).FirstOrCreate(&user)

			student := models.Student{
				StudentCode:    studentCode,
				UserID:         user.ID,
				ClassID:        class.ID,
				DateOfBirth:    time.Date(2005, time.Month(rand.Intn(12)+1), rand.Intn(28)+1, 0, 0, 0, 0, time.Local),
				Gender:         randomGender(),
				Phone:          fmt.Sprintf("091%07d", studentIndex),
				Address:        "Việt Nam",
				EnrollmentDate: enrollmentDateForClass(class.ClassCode),
				Status:         "active",
			}

			db.Where("student_code = ?", student.StudentCode).Assign(student).FirstOrCreate(&student)

			studentIndex++
		}
	}

	// =====================
	// 10. COURSE OFFERINGS + SCHEDULES
	// Mỗi học phần mở luôn xác định rõ môn, lớp, giáo viên, học kỳ và phòng.
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

	var savedRooms []models.Room
	db.Order("name").Find(&savedRooms)
	if len(savedRooms) == 0 {
		panic("Không có phòng học để tạo CourseOffering")
	}
	seededClassIDs := make([]uint, 0, len(seededClasses))
	for _, class := range seededClasses {
		seededClassIDs = append(seededClassIDs, class.ID)
	}

	// Xóa lịch kiểu cũ do chính seed cũ tạo ra. Lịch mới luôn thuộc CourseOffering.
	db.Where("course_offering_id IS NULL AND class_id IN ?", seededClassIDs).Delete(&models.Schedule{})

	for classIndex, class := range seededClasses {
		var courses []models.Course
		db.Where("major_id = ? AND semester_id = ? AND is_active = ?", class.MajorID, class.SemesterID, true).
			Order("code").Find(&courses)

		var majorTeachers []models.Teacher
		db.Where("major_id = ?", class.MajorID).Order("teacher_code").Find(&majorTeachers)
		if len(majorTeachers) == 0 {
			panic("Không có giáo viên đúng ngành cho lớp " + class.ClassCode)
		}

		for courseIndex, course := range courses {
			teacher := majorTeachers[(classIndex+courseIndex)%len(majorTeachers)]
			room := savedRooms[(classIndex+courseIndex)%len(savedRooms)]

			offering := models.CourseOffering{
				CourseID:   course.ID,
				ClassID:    class.ID,
				TeacherID:  teacher.ID,
				SemesterID: class.SemesterID,
				RoomID:     room.ID,
				Status:     "open",
			}
			db.Where("course_id = ? AND class_id = ? AND semester_id = ?", course.ID, class.ID, class.SemesterID).
				Assign(offering).FirstOrCreate(&offering)

			slot := (classIndex*len(courses) + courseIndex) % (len(days) * len(sessions))
			session := sessions[slot%len(sessions)]
			offeringID := offering.ID
			schedule := models.Schedule{
				ClassID:          class.ID,
				CourseOfferingID: &offeringID,
				DayOfWeek:        days[slot/len(sessions)],
				Session:          session.Name,
				StartTime:        session.StartTime,
				EndTime:          session.EndTime,
			}
			db.Where("course_offering_id = ?", offering.ID).Assign(schedule).FirstOrCreate(&schedule)
		}
	}

	// =====================
	// 11. ENROLLMENTS
	// Student -> CourseOffering -> Enrollment -> Grade
	// =====================

	var students []models.Student
	db.Where("student_code IN ?", seededStudentCodes).Order("student_code").Find(&students)

	for _, student := range students {
		var offerings []models.CourseOffering
		db.Where("class_id = ? AND status = ?", student.ClassID, "open").Order("course_id").Find(&offerings)

		for _, offering := range offerings {
			offeringID := offering.ID
			enrollment := models.Enrollment{
				CourseOfferingID: &offeringID,
				StudentID:         student.ID,
				ClassID:           offering.ClassID,
				CourseID:          offering.CourseID,
				EnrollDate:        activeSemester.StartDate,
				Status:            "enrolled",
			}
			db.Where("student_id = ? AND class_id = ? AND course_id = ?", student.ID, offering.ClassID, offering.CourseID).
				Assign(enrollment).FirstOrCreate(&enrollment)

			registration := models.CourseRegistration{
				StudentID:    student.ID,
				CourseID:     offering.CourseID,
				ClassID:      offering.ClassID,
				EnrollmentID: &enrollment.ID,
				Status:       "registered",
				RegisteredAt: activeSemester.StartDate,
				Note:         "Dữ liệu mẫu từ CourseOffering",
			}
			db.Where("enrollment_id = ?", enrollment.ID).Assign(registration).FirstOrCreate(&registration)

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

			db.Where("enrollment_id = ?", enrollment.ID).Assign(grade).FirstOrCreate(&grade)
		}
	}

	// =====================
	// 12. ROOM REGISTERS
	// Phòng được lấy từ từng CourseOffering, không lấy ngẫu nhiên từ Class.
	// =====================

	var offerings []models.CourseOffering
	db.Preload("Class").Preload("Course").Where("class_id IN ?", seededClassIDs).Order("id").Find(&offerings)

	for _, offering := range offerings {
		roomRegister := models.RoomRegister{
			ClassID: offering.ClassID,
			RoomID:  offering.RoomID,
			Date:    activeSemester.StartDate,
			Time:    "Theo lịch học phần",
			Note:    fmt.Sprintf("%s - %s", offering.Class.ClassCode, offering.Course.Code),
		}
		db.Where("class_id = ? AND room_id = ? AND note = ?", roomRegister.ClassID, roomRegister.RoomID, roomRegister.Note).
			Assign(roomRegister).FirstOrCreate(&roomRegister)
	}

	// =====================
	// 13. ATTENDANCE SESSION + ATTENDANCE
	// Teacher -> CourseOffering -> AttendanceSession (QR 1 phút) -> Attendance
	// =====================

	statuses := []string{"present", "present", "present", "absent", "late", "excused"}

	// Loại dữ liệu điểm danh do seed cũ tạo trực tiếp, không đi qua QR session.
	seededStudentIDs := make([]uint, 0, len(students))
	for _, student := range students {
		seededStudentIDs = append(seededStudentIDs, student.ID)
	}
	seededEnrollmentIDs := db.Model(&models.Enrollment{}).Select("id").Where("student_id IN ?", seededStudentIDs)
	db.Where("attendance_session_id IS NULL AND enrollment_id IN (?)", seededEnrollmentIDs).Delete(&models.Attendance{})

	for _, offering := range offerings {
		var offeringEnrollments []models.Enrollment
		db.Where("course_offering_id = ? AND status <> ?", offering.ID, "cancelled").Order("student_id").Find(&offeringEnrollments)

		for week := 1; week <= 5; week++ {
			classDate := activeSemester.StartDate.AddDate(0, 0, week*7)
			offeringID := offering.ID
			attendanceSession := models.AttendanceSession{
				CourseOfferingID: &offeringID,
				ClassID:          offering.ClassID,
				CourseID:         offering.CourseID,
				TeacherID:        offering.TeacherID,
				Code:             fmt.Sprintf("QR-O%04d-W%02d", offering.ID, week),
				ClassDate:        classDate,
				ExpiresAt:        classDate.Add(time.Minute),
				IsActive:         false,
				Note:             "Phiên QR mẫu, hiệu lực 1 phút",
			}
			db.Where("code = ?", attendanceSession.Code).Assign(attendanceSession).FirstOrCreate(&attendanceSession)

			for _, enrollment := range offeringEnrollments {
				status := statuses[rand.Intn(len(statuses))]
				var checkedInAt *time.Time
				if status == "present" || status == "late" {
					checkInTime := classDate.Add(30 * time.Second)
					if status == "late" {
						checkInTime = classDate.Add(10 * time.Minute)
					}
					checkedInAt = &checkInTime
				}

				attendanceSessionID := attendanceSession.ID
				attendance := models.Attendance{
					EnrollmentID:       enrollment.ID,
					AttendanceSessionID: &attendanceSessionID,
					ClassDate:          classDate,
					Status:             status,
					Note:               attendanceNote(status),
					CheckedInAt:        checkedInAt,
				}
				db.Where("enrollment_id = ? AND attendance_session_id = ?", enrollment.ID, attendanceSession.ID).
					Assign(attendance).FirstOrCreate(&attendance)
			}
		}
	}

	fmt.Println("Seed data completed successfully!")
}

func semesterNameForCourseIndex(index int) string {
	switch {
	case index < 8:
		return "HK1"
	case index < 16:
		return "HK2"
	case index < 23:
		return "HK3"
	default:
		return "HK4"
	}
}

func enrollmentDateForClass(classCode string) time.Time {
	if len(classCode) >= 3 {
		switch classCode[:3] {
		case "K24":
			return time.Date(2024, 9, 1, 0, 0, 0, 0, time.Local)
		case "K25":
			return time.Date(2025, 9, 1, 0, 0, 0, 0, time.Local)
		case "K26":
			return time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)
		}
	}
	return time.Date(2024, 9, 1, 0, 0, 0, 0, time.Local)
}

func attendanceNote(status string) string {
	switch status {
	case "absent":
		return "Vắng không phép"
	case "late":
		return "Đi trễ"
	case "excused":
		return "Vắng có phép"
	default:
		return "Đi học đầy đủ"
	}
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
