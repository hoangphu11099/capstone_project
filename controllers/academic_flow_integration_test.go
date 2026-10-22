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
	"student-management/middleware"
	"student-management/models"
	"testing"
	"time"
)

// Opt-in: creates an isolated local schema, never uses DB_NAME, then removes only that schema.
func TestAcademicFlowIntegration(t *testing.T) {
	if os.Getenv("RUN_ACADEMIC_INTEGRATION") != "1" {
		t.Skip("set RUN_ACADEMIC_INTEGRATION=1 to use isolated local MySQL schema")
	}
	env, _ := godotenv.Read("../.env")
	get := func(k, f string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		if v := env[k]; v != "" {
			return v
		}
		return f
	}
	host := get("DB_HOST", "localhost")
	if host != "localhost" && host != "127.0.0.1" {
		t.Fatal("integration test only supports local MySQL")
	}
	cfg := mysqlDriver.NewConfig()
	cfg.User = get("DB_USER", "root")
	cfg.Passwd = get("DB_PASSWORD", "")
	cfg.Net = "tcp"
	cfg.Addr = host + ":" + get("DB_PORT", "3306")
	cfg.ParseTime = true
	cfg.Loc = attendanceLocation()
	cfg.Timeout = 5 * time.Second
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	if err := admin.Ping(); err != nil {
		t.Fatal("local MySQL connection failed:", err)
	}
	name := fmt.Sprintf("pp_academic_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !strings.HasPrefix(name, "pp_academic_test_") {
			t.Fatal("invalid cleanup target")
		}
		if _, err := admin.Exec("DROP DATABASE `" + name + "`"); err != nil {
			t.Errorf("cleanup: %v", err)
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
	// Simulate the old unique schedule index, then verify the production migration is idempotent.
	if err := db.Exec("CREATE UNIQUE INDEX legacy_one_schedule ON schedules (course_offering_id)").Error; err != nil {
		t.Fatal(err)
	}
	config.AutoMigrateDB()
	config.AutoMigrateDB()
	create := func(v any) {
		t.Helper()
		if err := db.Create(v).Error; err != nil {
			t.Fatal(err)
		}
	}
	adminRole := models.Role{Name: "admin"}
	teacherRole := models.Role{Name: "teacher"}
	studentRole := models.Role{Name: "student"}
	create(&adminRole)
	create(&teacherRole)
	create(&studentRole)
	user := func(name string, role uint) models.User {
		u := models.User{Username: name, Email: name + "@example.test", Password: "test-only", FullName: name, RoleID: role}
		create(&u)
		return u
	}
	adminUser := user("admin", adminRole.ID)
	tu := user("teacher", teacherRole.ID)
	tu2 := user("teacher2", teacherRole.ID)
	major := models.Major{Code: "WEB", Name: "Web"}
	create(&major)
	teacher := models.Teacher{UserID: tu.ID, TeacherCode: "T1", MajorID: major.ID, HireDate: time.Now()}
	teacher2 := models.Teacher{UserID: tu2.ID, TeacherCode: "T2", MajorID: major.ID, HireDate: time.Now()}
	create(&teacher)
	create(&teacher2)
	room := models.Room{Name: "R1", Capacity: 40}
	room2 := models.Room{Name: "R2", Capacity: 40}
	create(&room)
	create(&room2)
	gin.SetMode(gin.TestMode)
	call := func(handler gin.HandlerFunc, uid uint, id string, body any, want int) map[string]json.RawMessage {
		t.Helper()
		raw, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/", bytes.NewReader(raw))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(middleware.ContextUserIDKey, uid)
		role := "student"
		if uid == adminUser.ID {
			role = "admin"
		} else if uid == tu.ID || uid == tu2.ID {
			role = "teacher"
		}
		c.Set(middleware.ContextRoleKey, role)
		c.Params = gin.Params{{Key: "id", Value: id}}
		handler(c)
		if w.Code != want {
			t.Fatalf("want %d got %d: %s", want, w.Code, w.Body.String())
		}
		var result map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	now := attendanceNow()
	day := func(v time.Time) string { return v.Format("2006-01-02") }
	var year models.AcademicYear
	payload := call(SaveAcademicYear, adminUser.ID, "", PeriodRequest{Name: "Test year", StartDate: day(now.AddDate(-1, 0, 0)), EndDate: day(now.AddDate(1, 0, 0))}, 200)
	json.Unmarshal(payload["data"], &year)
	var term, previous models.Semester
	payload = call(SaveSemester, adminUser.ID, "", PeriodRequest{Name: "New term", AcademicYearID: year.ID, StartDate: day(now.AddDate(0, 0, -5)), EndDate: day(now.AddDate(0, 0, 90)), Status: "active"}, 200)
	json.Unmarshal(payload["data"], &term)
	payload = call(SaveSemester, adminUser.ID, "", PeriodRequest{Name: "Previous term", AcademicYearID: year.ID, StartDate: day(now.AddDate(0, 0, -90)), EndDate: day(now.AddDate(0, 0, -10)), Status: "active"}, 200)
	json.Unmarshal(payload["data"], &previous)
	call(SaveSemester, adminUser.ID, "", PeriodRequest{Name: "Bad term", AcademicYearID: year.ID, StartDate: day(now.AddDate(2, 0, 0)), EndDate: day(now.AddDate(2, 0, 1))}, 400)
	payload = call(CreateClass, adminUser.ID, "", CreateClassRequest{MajorID: major.ID, CohortYear: now.Year(), MaxStudents: 30}, 201)
	var classData struct {
		Class models.Class `json:"class"`
	}
	json.Unmarshal(payload["data"], &classData)
	class := classData.Class
	if class.ID == 0 || class.SemesterID != 0 {
		t.Fatal("homeroom creation still requires semester")
	}
	payload = call(CreateCourse, adminUser.ID, "", CourseRequest{Code: "C1", Name: "Reusable course", Credits: 3, MajorID: major.ID}, 201)
	var course models.Course
	json.Unmarshal(payload["data"], &course)
	call(UpdateCourse, adminUser.ID, fmt.Sprint(course.ID), CourseRequest{Code: "C1", Name: "Reusable course updated", Credits: 3, MajorID: major.ID}, 200)
	su := user("student", studentRole.ID)
	student := models.Student{UserID: su.ID, StudentCode: "S1", ClassID: class.ID, DateOfBirth: now.AddDate(-20, 0, 0), EnrollmentDate: now}
	create(&student)
	startMinutes := now.Hour()*60 + now.Minute() - 10
	if startMinutes < 0 {
		startMinutes = 0
	}
	endMinutes := startMinutes + 60
	if endMinutes > 1439 {
		endMinutes = 1439
	}
	clock := func(m int) string { return fmt.Sprintf("%02d:%02d", m/60, m%60) }
	req := AssignCourseOfferingRequest{SemesterID: previous.ID, CourseID: course.ID, TeacherID: teacher.ID, RoomID: room.ID, TeachingDates: []string{day(now.AddDate(0, 0, -20))}, StartTime: clock(startMinutes), EndTime: clock(endMinutes)}
	call(AssignCourseOffering, adminUser.ID, fmt.Sprint(class.ID), req, 400)
	// Historical records predate the new scheduling policy.
	oldOffering := models.CourseOffering{ClassID: class.ID, CourseID: course.ID, TeacherID: teacher.ID, RoomID: room.ID, SemesterID: previous.ID, Status: "open"}
	create(&oldOffering)
	historical, _ := assignmentSchedules(req)
	for _, lesson := range historical {
		lesson.ClassID = class.ID
		lesson.CourseOfferingID = &oldOffering.ID
		create(&lesson)
	}
	if err := ensureEnrollment(db, student.ID, oldOffering); err != nil {
		t.Fatal(err)
	}
	req.SemesterID = term.ID
	req.TeacherID = teacher2.ID
	req.TeachingDates = []string{day(now), day(now.AddDate(0, 0, 7))}
	payload = call(AssignCourseOffering, adminUser.ID, fmt.Sprint(class.ID), req, 200)
	var offering models.CourseOffering
	json.Unmarshal(payload["data"], &offering)
	if offering.ID == oldOffering.ID || len(offering.Schedules) != 2 {
		t.Fatal("term isolation / multiple dates failed")
	}
	var enrollments []models.Enrollment
	db.Where("student_id = ?", student.ID).Find(&enrollments)
	if len(enrollments) != 2 {
		t.Fatal("expected one enrollment per term")
	}
	call(AssignTeacher, adminUser.ID, fmt.Sprint(class.ID), AssignTeacherRequest{TeacherID: teacher.ID}, 200)
	var unchanged models.CourseOffering
	db.First(&unchanged, offering.ID)
	if unchanged.TeacherID != teacher2.ID {
		t.Fatal("homeroom assignment overwrote offering teacher")
	}
	bad := req
	bad.TeachingDates = []string{day(now.AddDate(0, 0, -20))}
	call(AssignCourseOffering, adminUser.ID, fmt.Sprint(class.ID), bad, 400)
	course2 := models.Course{Code: "C2", Name: "Other", Credits: 3, MajorID: major.ID}
	create(&course2)
	bad = req
	bad.CourseID = course2.ID
	bad.TeacherID = teacher.ID
	bad.RoomID = room2.ID
	call(AssignCourseOffering, adminUser.ID, fmt.Sprint(class.ID), bad, 400)
	call(SaveSemester, adminUser.ID, fmt.Sprint(term.ID), PeriodRequest{Name: term.Name, AcademicYearID: year.ID, StartDate: day(now), EndDate: day(now), Status: "active"}, 400)
	payload = call(ViewSchedule, su.ID, "", nil, 200)
	var scheduleRows []map[string]any
	json.Unmarshal(payload["data"], &scheduleRows)
	if len(scheduleRows) != 3 {
		t.Fatalf("student schedule cross joined courses: %d", len(scheduleRows))
	}
	payload = call(CreateAttendanceSession, tu2.ID, "", CreateAttendanceSessionRequest{CourseOfferingID: offering.ID}, 200)
	var attendanceSession models.AttendanceSession
	json.Unmarshal(payload["data"], &attendanceSession)
	call(AttendanceByQR, su.ID, "", AttendanceQRRequest{Code: attendanceSession.Code}, 200)
	payload = call(ListAttendanceSessionRecords, tu2.ID, fmt.Sprint(attendanceSession.ID), nil, 200)
	var liveRecords []struct {
		Status     string            `json:"status"`
		Attendance models.Attendance `json:"attendance"`
	}
	json.Unmarshal(payload["data"], &liveRecords)
	if len(liveRecords) != 1 || liveRecords[0].Status != "present" || liveRecords[0].Attendance.Status != "present" {
		t.Fatal("teacher records did not reflect successful student QR check-in")
	}

	history := call(ListMyAttendances, su.ID, "", nil, 200)
	var pendingHistory []models.Attendance
	json.Unmarshal(history["data"], &pendingHistory)
	if len(pendingHistory) != 0 {
		t.Fatal("unconfirmed attendance leaked into student history")
	}
	call(CancelAttendanceSession, tu.ID, fmt.Sprint(attendanceSession.ID), nil, 403)
	call(CancelAttendanceSession, tu2.ID, fmt.Sprint(attendanceSession.ID), nil, 200)
	payload = call(ListAttendanceSessionRecords, tu2.ID, fmt.Sprint(attendanceSession.ID), nil, 200)
	json.Unmarshal(payload["data"], &liveRecords)
	if len(liveRecords) != 0 {
		t.Fatal("cancelled session recreated attendance")
	}
	payload = call(CreateAttendanceSession, tu2.ID, "", CreateAttendanceSessionRequest{CourseOfferingID: offering.ID}, 200)
	json.Unmarshal(payload["data"], &attendanceSession)
	call(AttendanceByQR, su.ID, "", AttendanceQRRequest{Code: attendanceSession.Code}, 200)
	call(CloseAttendanceSession, tu2.ID, fmt.Sprint(attendanceSession.ID), nil, 200)
	call(AttendanceByQR, su.ID, "", AttendanceQRRequest{Code: attendanceSession.Code}, 400)
	history = call(ListMyAttendances, su.ID, "", nil, 200)
	json.Unmarshal(history["data"], &pendingHistory)
	if len(pendingHistory) != 1 || pendingHistory[0].Status != "present" {
		t.Fatal("confirmed attendance missing from history")
	}
	var enrollment models.Enrollment
	db.Where("student_id = ? AND course_offering_id = ?", student.ID, offering.ID).First(&enrollment)
	call(UpsertGrade, tu.ID, "", GradeRequest{EnrollmentID: enrollment.ID, AssignmentScore: 80, MidtermScore: 80, FinalScore: 80}, 403)
	call(UpsertGrade, tu2.ID, "", GradeRequest{EnrollmentID: enrollment.ID, AssignmentScore: 80, MidtermScore: 80, FinalScore: 80}, 200)
	var grade models.Grade
	db.Where("enrollment_id = ?", enrollment.ID).First(&grade)
	call(ApproveGrade, tu2.ID, fmt.Sprint(grade.ID), nil, 200)
	for _, endpoint := range []struct {
		handler     gin.HandlerFunc
		id          uint
		classScoped bool
	}{{AdminClassLearningDetail, class.ID, true}, {StudentLearningDetail, student.ID, false}} {
		learningPayload := call(endpoint.handler, adminUser.ID, fmt.Sprint(endpoint.id), nil, 200)
		var learningRows []models.Enrollment
		json.Unmarshal(learningPayload["data"], &learningRows)
		found := false
		for _, row := range learningRows {
			if row.ID == enrollment.ID {
				found = row.Grade != nil && row.Grade.TotalScore == 80 && row.Grade.Status == "Approved" && len(row.Attendances) > 0 && row.CourseOffering != nil && len(row.CourseOffering.Schedules) > 0
			}
			if endpoint.classScoped && row.ClassID != class.ID {
				t.Fatal("class learning details included another class")
			}
		}
		if !found {
			t.Fatal("learning details omitted grade, attendance, or planned lessons")
		}
	}
	var transcript models.Transcript
	db.Where("enrollment_id = ?", enrollment.ID).First(&transcript)
	if transcript.SemesterID != term.ID {
		t.Fatal("transcript uses wrong semester")
	}
	call(CreateExercise, tu.ID, "", ExerciseRequest{CourseOfferingID: offering.ID, Title: "Unauthorized", DueDate: day(now.AddDate(0, 0, 1))}, 403)
	payload = call(CreateExercise, tu2.ID, "", ExerciseRequest{CourseOfferingID: offering.ID, Title: "Term exercise", DueDate: day(now.AddDate(0, 0, 1))}, 201)
	var exercise models.Exercise
	json.Unmarshal(payload["data"], &exercise)
	payload = call(SubmitExercise, su.ID, fmt.Sprint(exercise.ID), SubmissionRequest{Content: "My answer"}, 200)
	var submission models.Submission
	json.Unmarshal(payload["data"], &submission)
	call(GradeSubmission, tu2.ID, fmt.Sprint(submission.ID), map[string]interface{}{"score": 85, "feedback": "Good"}, 200)
	call(ListStudentExercises, su.ID, "", nil, 200)
	call(ListExerciseSubmissions, tu2.ID, fmt.Sprint(exercise.ID), nil, 200)
	call(ListExerciseSubmissions, tu.ID, fmt.Sprint(exercise.ID), nil, 403)
	call(GradeSubmission, tu.ID, fmt.Sprint(submission.ID), map[string]interface{}{"score": 90}, 403)
	call(GradeSubmission, tu2.ID, fmt.Sprint(submission.ID), map[string]interface{}{"feedback": "missing score"}, 400)
	call(GradeSubmission, tu2.ID, fmt.Sprint(submission.ID), map[string]interface{}{"score": 101}, 400)
	call(UpdateExerciseAvailability, tu.ID, fmt.Sprint(exercise.ID), ExerciseAvailabilityRequest{Status: "closed"}, 403)
	call(UpdateExerciseAvailability, tu2.ID, fmt.Sprint(exercise.ID), ExerciseAvailabilityRequest{Status: "closed"}, 200)
	call(SubmitExercise, su.ID, fmt.Sprint(exercise.ID), SubmissionRequest{Content: "blocked"}, 409)
	call(UpdateExerciseAvailability, tu2.ID, fmt.Sprint(exercise.ID), ExerciseAvailabilityRequest{Status: "open", DueDate: day(now.AddDate(0, 0, -1))}, 400)
	call(UpdateExerciseAvailability, tu2.ID, fmt.Sprint(exercise.ID), ExerciseAvailabilityRequest{Status: "open", DueDate: now.Add(2 * time.Hour).Format(time.RFC3339)}, 200)
	call(SubmitExercise, su.ID, fmt.Sprint(exercise.ID), SubmissionRequest{Content: "Revised answer", FileURL: "https://example.test/answer.pdf"}, 200)
	var revised models.Submission
	db.First(&revised, submission.ID)
	if revised.Score != nil || revised.Feedback != "" || revised.Status != "submitted" || revised.Content != "Revised answer" {
		t.Fatal("resubmission retained stale grade or lost content")
	}
	var submissionCount int64
	db.Model(&models.Submission{}).Where("exercise_id = ? AND student_id = ?", exercise.ID, student.ID).Count(&submissionCount)
	if submissionCount != 1 {
		t.Fatal("resubmission duplicated rows")
	}
	call(GradeSubmission, tu2.ID, fmt.Sprint(submission.ID), map[string]interface{}{"score": 0, "submittedAt": now.Add(-time.Hour).Format(time.RFC3339)}, 400)
	call(GradeSubmission, tu2.ID, fmt.Sprint(submission.ID), map[string]interface{}{"score": 0, "feedback": "Please revise"}, 200)
	db.First(&revised, submission.ID)
	if revised.Score == nil || *revised.Score != 0 || revised.Status != "graded" {
		t.Fatal("zero grade was not persisted")
	}
	call(ListMySubmissions, su.ID, "", nil, 200)
	db.Model(&models.Exercise{}).Where("id = ?", exercise.ID).Update("due_date", now.Add(-time.Minute))
	call(SubmitExercise, su.ID, fmt.Sprint(exercise.ID), SubmissionRequest{Content: "late overwrite"}, 409)
	db.First(&revised, submission.ID)
	if revised.Content != "Revised answer" || revised.Score == nil {
		t.Fatal("late submission changed saved answer")
	}
	call(CreateExercise, tu2.ID, "", ExerciseRequest{CourseOfferingID: offering.ID, Title: "Past deadline", DueDate: day(now.AddDate(0, 0, -1))}, 400)

	su2 := user("student2", studentRole.ID)
	student2 := models.Student{UserID: su2.ID, StudentCode: "S2", ClassID: class.ID, DateOfBirth: now.AddDate(-20, 0, 0), EnrollmentDate: now}
	create(&student2)
	call(SubmitExercise, su2.ID, fmt.Sprint(exercise.ID), SubmissionRequest{Content: "Not enrolled"}, 403)
	call(RegisterCourse, su2.ID, "", RegisterCourseRequest{CourseOfferingID: oldOffering.ID}, 201)
	call(RegisterCourse, su2.ID, "", RegisterCourseRequest{CourseOfferingID: offering.ID}, 201)
	call(RegisterCourse, su2.ID, "", RegisterCourseRequest{CourseOfferingID: offering.ID}, 400)
	payload = call(ViewStatisticalReport, tu2.ID, "", nil, 200)
	var teacherReport struct {
		TotalEnrollments int `json:"totalEnrollments"`
		Grades           struct {
			Total int `json:"totalGrades"`
		} `json:"grades"`
	}
	json.Unmarshal(payload["data"], &teacherReport)
	if teacherReport.TotalEnrollments != 2 || teacherReport.Grades.Total != 1 {
		t.Fatalf("report scoped to wrong teacher: %+v", teacherReport)
	}
	payload = call(ExportTranscript, tu2.ID, "", nil, 200)
	var exported []TranscriptExportRow
	json.Unmarshal(payload["data"], &exported)
	if len(exported) != 2 || exported[0].SemesterName != term.Name {
		t.Fatal("export lost offering scope/semester")
	}
	payload = call(DashboardStudent, su.ID, "", nil, 200)
	var studentDashboard struct {
		OpenExercises int `json:"openExercises"`
	}
	json.Unmarshal(payload["data"], &studentDashboard)
	if studentDashboard.OpenExercises != 0 {
		t.Fatal("student dashboard counted an expired exercise as open")
	}
	var cancelled models.Enrollment
	db.Where("student_id = ? AND course_offering_id = ?", student2.ID, offering.ID).First(&cancelled)
	call(CancelCourseRegistration, su2.ID, fmt.Sprint(cancelled.ID), nil, 200)
	call(AssignCourseOffering, adminUser.ID, fmt.Sprint(class.ID), req, 200)
	db.First(&cancelled, cancelled.ID)
	if cancelled.Status != "cancelled" {
		t.Fatal("schedule change resurrected cancelled enrollment")
	}
	call(GetMetadata, adminUser.ID, "", nil, 200)
	consultation := consultationRequest{FullName: "Nguyễn An", Phone: "0901234567", Email: "consultation@example.test", Program: "Lập trình phần mềm", Location: "Online", PreferredSession: "Tối", Consent: true}
	consultationPayload := call(CreatePublicConsultation, 0, "", consultation, 201)
	var consultationID uint
	json.Unmarshal(consultationPayload["reference"], &consultationID)
	if consultationID == 0 {
		t.Fatal("consultation reference missing")
	}
	call(CreatePublicConsultation, 0, "", consultation, 429)
	consultationPayload = call(ListConsultations, adminUser.ID, "", nil, 200)
	var consultations []models.Consultation
	json.Unmarshal(consultationPayload["data"], &consultations)
	if len(consultations) != 1 || consultations[0].Phone != "0901234567" || consultations[0].ConsentAt.IsZero() {
		t.Fatal("consultation or consent not persisted")
	}
	call(UpdateConsultation, adminUser.ID, fmt.Sprint(consultationID), map[string]string{"status": "contacted"}, 200)
	var savedConsultation models.Consultation
	db.First(&savedConsultation, consultationID)
	if savedConsultation.Status != "contacted" {
		t.Fatal("consultation status not updated")
	}
	facultyPayload := call(ListPublicFaculty, 0, "", nil, 200)
	var facultyProfiles []struct {
		ID        uint   `json:"id"`
		Biography string `json:"biography"`
		AvatarURL string `json:"avatarUrl"`
	}
	json.Unmarshal(facultyPayload["data"], &facultyProfiles)
	if len(facultyProfiles) == 0 {
		t.Fatal("missing faculty for profile test")
	}
	facultyID := fmt.Sprint(facultyProfiles[0].ID)
	call(UpdateFacultyProfile, adminUser.ID, facultyID, map[string]string{"biography": "Giới thiệu chuyên môn đã cập nhật", "avatarUrl": "/faculty.jpg"}, 200)
	facultyPayload = call(ListPublicFaculty, 0, "", nil, 200)
	json.Unmarshal(facultyPayload["data"], &facultyProfiles)
	if facultyProfiles[0].Biography != "Giới thiệu chuyên môn đã cập nhật" || facultyProfiles[0].AvatarURL != "/faculty.jpg" {
		t.Fatal("faculty biography not persisted")
	}
	call(UpdateFacultyProfile, adminUser.ID, facultyID, map[string]string{"avatarUrl": "javascript:alert(1)"}, 400)

	updateProgramImage := func(c *gin.Context) {
		c.Params = append(c.Params, gin.Param{Key: "slug", Value: c.Param("id")})
		UpdateProgramImage(c)
	}
	// Program image overrides persist across reads and can return to bundled defaults.
	call(updateProgramImage, adminUser.ID, "lap-trinh-phan-mem", map[string]string{"imageUrl": "/new-program.jpg"}, 200)
	imagePayload := call(ListProgramImages, 0, "", nil, 200)
	var programImages []models.ProgramImage
	json.Unmarshal(imagePayload["data"], &programImages)
	if len(programImages) != 1 || programImages[0].ImageURL != "/new-program.jpg" {
		t.Fatal("program image override not persisted")
	}
	call(updateProgramImage, adminUser.ID, "lap-trinh-phan-mem", map[string]string{"imageUrl": "javascript:alert(1)"}, 400)
	call(updateProgramImage, adminUser.ID, "lap-trinh-phan-mem", map[string]string{"imageUrl": "//example.com/image.jpg"}, 400)
	call(updateProgramImage, adminUser.ID, "lap-trinh-phan-mem", map[string]string{"imageUrl": ""}, 200)
	var savedImage models.ProgramImage
	db.First(&savedImage, "slug = ?", "lap-trinh-phan-mem")
	if savedImage.ImageURL != "" {
		t.Fatal("default image reset failed")
	}
	savePublicProgram := func(c *gin.Context) {
		c.Params = append(c.Params, gin.Param{Key: "slug", Value: c.Param("id")})
		SavePublicProgram(c)
	}
	programRequest := map[string]any{"slug": "nganh-thu-nghiem", "name": "Ngành thử nghiệm", "group": "Công nghệ", "symbol": "code", "image": "/test.jpg", "tagline": "Mô tả mới", "description": "Giới thiệu mới", "tools": []string{"React"}, "skills": []string{"Kỹ năng mới"}, "careers": []string{"Nghề nghiệp mới"}, "project": "Dự án mới", "stages": []map[string]any{{"title": "Chặng mới", "topics": []string{"Nội dung mới"}}}, "published": true}
	call(savePublicProgram, adminUser.ID, "nganh-thu-nghiem", programRequest, 200)
	programPayload := call(ListPublicPrograms, 0, "", nil, 200)
	var publicPrograms []publicProgramContent
	json.Unmarshal(programPayload["data"], &publicPrograms)
	if len(publicPrograms) != 1 || publicPrograms[0].Name != "Ngành thử nghiệm" || publicPrograms[0].Stages[0].Topics[0] != "Nội dung mới" {
		t.Fatal("program content did not persist")
	}
	programRequest["published"] = false
	programRequest["name"] = "Đổi tên ngành"
	call(savePublicProgram, adminUser.ID, "nganh-thu-nghiem", programRequest, 200)
	programPayload = call(ListPublicPrograms, 0, "", nil, 200)
	json.Unmarshal(programPayload["data"], &publicPrograms)
	if publicPrograms[0].Published || publicPrograms[0].Name != "Đổi tên ngành" {
		t.Fatal("program visibility or edited content was lost")
	}
	programRequest["stages"] = []map[string]any{}
	call(savePublicProgram, adminUser.ID, "nganh-thu-nghiem", programRequest, 400)
	studentStoryRequest := schoolPostRequest{Title: "Hành trình đồ án", Content: "Nội dung câu chuyện", Excerpt: "Trải nghiệm sinh viên", Category: "student", IsPublished: true}
	storyPayload := call(CreateAdminPost, adminUser.ID, "", studentStoryRequest, 201)
	var story models.SchoolPost
	json.Unmarshal(storyPayload["data"], &story)
	if story.Category != "student" {
		t.Fatal("student story category not persisted")
	}
	storiesPayload := call(GetPublicPosts, 0, "", nil, 200)
	var publishedStories []models.SchoolPost
	json.Unmarshal(storiesPayload["data"], &publishedStories)
	foundStory := false
	for _, item := range publishedStories {
		if item.ID == story.ID && item.Category == "student" {
			foundStory = true
		}
	}
	if !foundStory {
		t.Fatal("published student story missing from public API")
	}

	call(ListAcademicYears, adminUser.ID, "", nil, 200)
	call(ListSemesters, adminUser.ID, "", nil, 200)
	call(ListAdminCourseOfferings, adminUser.ID, "", nil, 200)
	call(ViewTeacherSchedule, tu2.ID, "", nil, 200)
	call(ListClassGrades, tu2.ID, fmt.Sprint(class.ID), nil, 200)
	examReq := ExamScheduleRequest{ClassID: class.ID, CourseID: course.ID, SemesterID: previous.ID, RoomID: room.ID, ExamDate: day(now.AddDate(0, 0, -20)), Session: "AM", StartTime: "08:00", EndTime: "09:00"}
	call(CreateExamSchedule, adminUser.ID, "", examReq, 201)
	examReq.SemesterID = term.ID
	examReq.ExamDate = day(now.AddDate(0, 0, 8))
	call(CreateExamSchedule, adminUser.ID, "", examReq, 201)
	studentExams, err := personalExams(student.ID, 0)
	if err != nil || len(studentExams) == 0 {
		t.Fatalf("student missing exam calendar: %v", err)
	}
	teacherExams, err := personalExams(0, offering.TeacherID)
	if err != nil || len(teacherExams) == 0 {
		t.Fatalf("teacher missing exam calendar: %v", err)
	}
	var examNotifications int64
	db.Model(&models.Notification{}).Where("recipient_user_id = ? AND subject LIKE ?", su.ID, "Lịch thi%").Count(&examNotifications)
	if examNotifications == 0 {
		t.Fatal("student did not receive exam notification")
	}

	payload = call(CreateTeacher, adminUser.ID, "", CreateTeacherRequest{MajorID: major.ID, Username: "created-teacher", FullName: "Created Teacher", Email: "created-teacher@example.test"}, 201)
	var generated models.Teacher
	json.Unmarshal(payload["data"], &generated)
	if !strings.HasPrefix(generated.TeacherCode, "GV") || len(generated.TeacherCode) != 12 {
		t.Fatal("missing generated teacher code")
	}
	payload = call(CreateTeacher, adminUser.ID, "", CreateTeacherRequest{MajorID: major.ID, Username: "created-teacher-2", FullName: "Second Teacher", Email: "created-teacher-2@example.test"}, 201)
	var generated2 models.Teacher
	json.Unmarshal(payload["data"], &generated2)
	if generated2.TeacherCode == generated.TeacherCode {
		t.Fatal("duplicate generated code")
	}
	call(CreateTeacher, adminUser.ID, "", CreateTeacherRequest{MajorID: major.ID, Username: "created-teacher", FullName: "Must not overwrite", Email: "duplicate@example.test"}, 400)
	var original models.User
	db.First(&original, generated.UserID)
	if original.FullName != "Created Teacher" {
		t.Fatal("create overwrote existing user")
	}
	if err := importOneTeacher(TeacherImportItem{Username: "created-teacher", FullName: "Created Teacher", Email: "created-teacher@example.test"}, teacherRole.ID); err != nil {
		t.Fatal(err)
	}
	var afterImport models.Teacher
	db.First(&afterImport, generated.ID)
	if afterImport.TeacherCode != generated.TeacherCode {
		t.Fatal("import without code changed existing code")
	}
	if afterImport.MajorID != major.ID {
		t.Fatal("legacy import erased teaching major")
	}
	if err := importOneTeacher(TeacherImportItem{Username: "imported-new", FullName: "Imported Teacher", Email: "imported-new@example.test", MajorCode: major.Code}, teacherRole.ID); err != nil {
		t.Fatal(err)
	}
	var imported models.Teacher
	db.Joins("JOIN users ON users.id = teachers.user_id").Where("users.username = ?", "imported-new").First(&imported)
	if !strings.HasPrefix(imported.TeacherCode, "GV") {
		t.Fatal("import did not generate teacher code")
	}
	if imported.MajorID != major.ID {
		t.Fatal("import did not save teaching major")
	}

	call(CreateStudent, adminUser.ID, "", CreateStudentRequest{Username: "created-student", FullName: "Created Student", Email: "created-student@example.test", ClassID: class.ID}, 201)
	var editable models.Student
	db.Preload("User").Where("student_code <> ?", "").Joins("JOIN users ON users.id = students.user_id").Where("users.username = ?", "created-student").First(&editable)
	update := CreateStudentRequest{Username: "created-student", FullName: "Updated Student", Email: "created-student@example.test", ClassID: class.ID, Phone: "0123456789"}
	call(UpdateStudent, adminUser.ID, fmt.Sprint(editable.ID), update, 200)
	var updated models.Student
	db.Preload("User").First(&updated, editable.ID)
	if updated.User.FullName != update.FullName || updated.Phone != update.Phone || updated.User.Password != editable.User.Password {
		t.Fatal("student update lost fields or changed password")
	}
	update.Username = adminUser.Username
	call(UpdateStudent, adminUser.ID, fmt.Sprint(editable.ID), update, 400)
	call(DeleteStudent, adminUser.ID, fmt.Sprint(editable.ID), nil, 400)
	emptyClass := models.Class{ClassCode: "EMPTY_TEST", MajorID: major.ID, Status: "open", MaxStudents: 10}
	create(&emptyClass)
	payload = call(CreateStudent, adminUser.ID, "", CreateStudentRequest{Username: "deletable-student", FullName: "Delete me", Email: "delete@example.test", ClassID: emptyClass.ID}, 201)
	var deletable models.Student
	json.Unmarshal(payload["data"], &deletable)
	call(DeleteStudent, adminUser.ID, fmt.Sprint(deletable.ID), nil, 200)
	var remaining int64
	db.Model(&models.User{}).Where("id = ?", deletable.UserID).Count(&remaining)
	if remaining != 0 {
		t.Fatal("deleted student can still log in")
	}
	call(DeleteStudent, adminUser.ID, fmt.Sprint(deletable.ID), nil, 404)

	var beforeTeacherUpdate models.User
	newMajorPayload := call(CreateMajor, adminUser.ID, "", map[string]string{"code": "BIZ", "name": "Business administration"}, 201)
	var teacherMajor models.Major
	json.Unmarshal(newMajorPayload["data"], &teacherMajor)
	call(CreateMajor, adminUser.ID, "", map[string]string{"code": "biz", "name": "Duplicate"}, 400)
	call(CreateMajor, adminUser.ID, "", map[string]string{"code": "", "name": "Missing code"}, 400)
	call(CreateTeacher, adminUser.ID, "", CreateTeacherRequest{Username: "no-major", FullName: "No major"}, 400)
	db.First(&beforeTeacherUpdate, generated.UserID)
	teacherUpdate := CreateTeacherRequest{MajorID: teacherMajor.ID, Username: "created-teacher", FullName: "Updated Teacher", Email: "created-teacher@example.test", Phone: "0123456789", Qualification: "Doctor", TeacherCode: "SHOULD_NOT_CHANGE"}
	call(UpdateTeacher, adminUser.ID, fmt.Sprint(generated.ID), teacherUpdate, 200)
	var editedTeacher models.Teacher
	db.Preload("User").First(&editedTeacher, generated.ID)
	if editedTeacher.MajorID != teacherMajor.ID {
		t.Fatal("teacher update failed to persist teaching major")
	}
	if editedTeacher.TeacherCode != generated.TeacherCode || editedTeacher.Qualification != "Doctor" || editedTeacher.User.FullName != "Updated Teacher" || editedTeacher.User.Password != beforeTeacherUpdate.Password {
		t.Fatal("teacher update changed code/password or failed to save profile")
	}
	teacherUpdate.Username = adminUser.Username
	teacherUpdate.Qualification = "Should roll back"
	call(UpdateTeacher, adminUser.ID, fmt.Sprint(generated.ID), teacherUpdate, 400)
	db.Preload("User").First(&editedTeacher, generated.ID)
	if editedTeacher.Qualification != "Doctor" {
		t.Fatal("failed teacher update was not atomic")
	}
	teacherUpdate.Username = "created-teacher"
	teacherUpdate.Password = "Updated@123"
	call(UpdateTeacher, adminUser.ID, fmt.Sprint(generated.ID), teacherUpdate, 200)
	db.Preload("User").First(&editedTeacher, generated.ID)
	if bcrypt.CompareHashAndPassword([]byte(editedTeacher.User.Password), []byte(teacherUpdate.Password)) != nil {
		t.Fatal("new teacher password not saved")
	}
	call(UpdateTeacher, adminUser.ID, "0", teacherUpdate, 400)
	call(UpdateTeacher, adminUser.ID, "99999999", teacherUpdate, 404)

	payload = call(SaveRoom, adminUser.ID, "", RoomRequest{Name: " A999 ", Building: "New building", Capacity: 40, Description: "Lab"}, 201)
	var newRoom models.Room
	json.Unmarshal(payload["data"], &newRoom)
	if newRoom.Name != "A999" || !newRoom.IsActive {
		t.Fatal("new room not active or trimmed")
	}
	call(SaveRoom, adminUser.ID, "", RoomRequest{Name: "A999", Capacity: 30}, 400)
	call(SaveRoom, adminUser.ID, "", RoomRequest{Name: "Invalid", Capacity: -1}, 400)
	call(SaveRoom, adminUser.ID, "", RoomRequest{Name: "   ", Capacity: 30}, 400)
	call(SaveRoom, adminUser.ID, fmt.Sprint(newRoom.ID), RoomRequest{Name: "A998", Building: "", Capacity: 45, Description: ""}, 200)
	db.First(&newRoom, newRoom.ID)
	if newRoom.Name != "A998" || newRoom.Building != "" || newRoom.Description != "" || newRoom.Capacity != 45 {
		t.Fatal("room update failed")
	}
	call(ListRooms, adminUser.ID, "", nil, 200)
	payload = call(GetMetadata, adminUser.ID, "", nil, 200)
	var roomMetadata struct {
		Rooms []models.Room `json:"rooms"`
	}
	json.Unmarshal(payload["data"], &roomMetadata)
	foundRoom := false
	for _, item := range roomMetadata.Rooms {
		if item.ID == newRoom.ID {
			foundRoom = true
		}
	}
	if !foundRoom {
		t.Fatal("created room missing from scheduling metadata")
	}

	t.Run("teaching offer creates schedule only after acceptance", func(t *testing.T) {
		offerClass := models.Class{ClassCode: "OFFER_TEST", MajorID: major.ID, Status: "open", MaxStudents: 30}
		create(&offerClass)
		offerCourse := models.Course{Code: "OFFER_COURSE", Name: "Offer course", MajorID: major.ID, IsActive: true}
		create(&offerCourse)
		offerStudentUser := user("offer-student", studentRole.ID)
		offerStudent := models.Student{UserID: offerStudentUser.ID, StudentCode: "OFFER_S", ClassID: offerClass.ID, EnrollmentDate: now, DateOfBirth: now.AddDate(-20, 0, 0)}
		create(&offerStudent)
		input := CreateClassOfferRequest{ClassID: offerClass.ID, TeacherID: teacher2.ID, CourseID: offerCourse.ID, SemesterID: term.ID, RoomID: newRoom.ID, TeachingDates: []string{day(now.AddDate(0, 0, 30)), day(now.AddDate(0, 0, 37))}, StartTime: "22:00", EndTime: "23:00", Title: "Detailed plan"}
		invalid := input
		invalid.TeachingDates = nil
		call(CreateClassOffer, adminUser.ID, "", invalid, 400)
		result := call(CreateClassOffer, adminUser.ID, "", input, 201)
		var offer models.ClassOffer
		json.Unmarshal(result["data"], &offer)
		var planned teachingOfferPlan
		if json.Unmarshal([]byte(offer.PlanJSON), &planned) != nil || len(planned.Schedules) != 2 {
			t.Fatal("missing immutable detailed plan")
		}
		call(CreateClassOffer, adminUser.ID, "", input, 400)
		var count int64
		db.Model(&models.CourseOffering{}).Where("class_id = ?", offerClass.ID).Count(&count)
		if count != 0 {
			t.Fatal("pending offer assigned course before acceptance")
		}
		call(AcceptClassOffer, tu.ID, fmt.Sprint(offer.ID), ClassOfferResponseRequest{}, 404)
		call(AcceptClassOffer, tu2.ID, fmt.Sprint(offer.ID), ClassOfferResponseRequest{Note: "Confirmed"}, 200)
		call(AcceptClassOffer, tu2.ID, fmt.Sprint(offer.ID), ClassOfferResponseRequest{}, 400)
		var received models.CourseOffering
		if err := db.Preload("Schedules").Preload("Enrollments").Where("class_id = ? AND course_id = ?", offerClass.ID, offerCourse.ID).First(&received).Error; err != nil {
			t.Fatal(err)
		}
		if received.TeacherID != teacher2.ID || len(received.Schedules) != 2 || len(received.Enrollments) != 1 {
			t.Fatal("acceptance did not atomically assign lessons and enroll student")
		}
		listed := call(ListTeacherClasses, tu2.ID, "", nil, 200)
		var teacherClasses []models.Class
		json.Unmarshal(listed["data"], &teacherClasses)
		found := false
		for _, c := range teacherClasses {
			if c.ID == offerClass.ID {
				found = true
			}
		}
		if !found {
			t.Fatal("accepted class missing from My Classes")
		}
		call(ViewTeacherSchedule, tu2.ID, "", nil, 200)

		filtered := call(func(c *gin.Context) {
			c.Request.URL.RawQuery = "classId=" + fmt.Sprint(offerClass.ID)
			ListAdminCourseOfferings(c)
		}, adminUser.ID, "", nil, 200)
		var classOfferings []models.CourseOffering
		json.Unmarshal(filtered["data"], &classOfferings)
		if len(classOfferings) != 1 || classOfferings[0].ClassID != offerClass.ID {
			t.Fatal("class details returned offerings from another class")
		}
		roster := call(func(c *gin.Context) {
			c.Request.URL.RawQuery = "classId=" + fmt.Sprint(offerClass.ID)
			ListStudents(c)
		}, adminUser.ID, "", nil, 200)
		var classStudents []models.Student
		json.Unmarshal(roster["data"], &classStudents)
		if len(classStudents) != 1 || classStudents[0].UserID != offerStudentUser.ID || classStudents[0].User.FullName == "" {
			t.Fatal("class roster omitted student details or included students from other classes")
		}
		call(func(c *gin.Context) {
			c.Request.URL.RawQuery = "classId=invalid"
			ListStudents(c)
		}, adminUser.ID, "", nil, 400)
		classList := call(ListClasses, adminUser.ID, "", nil, 200)
		var classesWithSchedules []models.Class
		json.Unmarshal(classList["data"], &classesWithSchedules)
		for _, row := range classesWithSchedules {
			if row.ID == offerClass.ID && (len(row.CourseOfferings) != 1 || len(row.CourseOfferings[0].Schedules) == 0) {
				t.Fatal("class list omitted schedules needed to determine completed status")
			}
		}

		rejectedCourse := models.Course{Code: "REJECT_COURSE", Name: "Rejected", MajorID: major.ID, IsActive: true}
		create(&rejectedCourse)
		input.CourseID = rejectedCourse.ID
		input.TeachingDates = []string{day(now.AddDate(0, 0, 31))}
		result = call(CreateClassOffer, adminUser.ID, "", input, 201)
		json.Unmarshal(result["data"], &offer)
		call(RejectClassOffer, tu2.ID, fmt.Sprint(offer.ID), ClassOfferResponseRequest{Note: "Unavailable"}, 200)
		db.Model(&models.CourseOffering{}).Where("class_id = ? AND course_id = ?", offerClass.ID, rejectedCourse.ID).Count(&count)
		if count != 0 {
			t.Fatal("rejection created an offering")
		}

		// A conflict appearing after the proposal must be checked again at acceptance.
		conflictCourse := models.Course{Code: "CONFLICT_OFFER", Name: "Conflict", MajorID: major.ID, IsActive: true}
		create(&conflictCourse)
		input.CourseID = conflictCourse.ID
		input.TeachingDates = []string{day(now.AddDate(0, 0, 32))}
		result = call(CreateClassOffer, adminUser.ID, "", input, 201)
		json.Unmarshal(result["data"], &offer)
		blockingClass := models.Class{ClassCode: "BLOCKING_OFFER", MajorID: major.ID, Status: "open"}
		create(&blockingClass)
		call(AssignCourseOffering, adminUser.ID, fmt.Sprint(blockingClass.ID), AssignCourseOfferingRequest{CourseID: conflictCourse.ID, TeacherID: teacher2.ID, SemesterID: term.ID, RoomID: room2.ID, TeachingDates: input.TeachingDates, StartTime: "22:00", EndTime: "23:00"}, 200)
		call(AcceptClassOffer, tu2.ID, fmt.Sprint(offer.ID), ClassOfferResponseRequest{}, 400)
		db.First(&offer, offer.ID)
		if offer.Status != "pending" {
			t.Fatal("failed acceptance consumed offer")
		}
		db.Model(&models.CourseOffering{}).Where("class_id = ? AND course_id = ?", offerClass.ID, conflictCourse.ID).Count(&count)
		if count != 0 {
			t.Fatal("failed acceptance partially saved schedule")
		}
		call(RejectClassOffer, tu2.ID, fmt.Sprint(offer.ID), ClassOfferResponseRequest{}, 200)

		// A changed plan must never be accepted using stale details.
		staleCourse := models.Course{Code: "STALE_OFFER", Name: "Stale", MajorID: major.ID, IsActive: true}
		create(&staleCourse)
		input.CourseID = staleCourse.ID
		input.TeachingDates = []string{day(now.AddDate(0, 0, 33))}
		result = call(CreateClassOffer, adminUser.ID, "", input, 201)
		json.Unmarshal(result["data"], &offer)
		call(AssignCourseOffering, adminUser.ID, fmt.Sprint(offerClass.ID), AssignCourseOfferingRequest{CourseID: staleCourse.ID, TeacherID: teacher2.ID, SemesterID: term.ID, RoomID: newRoom.ID, TeachingDates: input.TeachingDates, StartTime: "20:00", EndTime: "21:00"}, 200)
		call(AcceptClassOffer, tu2.ID, fmt.Sprint(offer.ID), ClassOfferResponseRequest{}, 400)
		call(RejectClassOffer, tu2.ID, fmt.Sprint(offer.ID), ClassOfferResponseRequest{}, 200)
		legacy := models.ClassOffer{ClassID: offerClass.ID, TeacherID: teacher2.ID, OfferedByUserID: adminUser.ID, Status: "pending", OfferedAt: now}
		create(&legacy)
		call(AcceptClassOffer, tu2.ID, fmt.Sprint(legacy.ID), ClassOfferResponseRequest{}, 400)
		call(RejectClassOffer, tu2.ID, fmt.Sprint(legacy.ID), ClassOfferResponseRequest{}, 200)
		listed = call(ListClasses, adminUser.ID, "", nil, 200)
		var allClasses []models.Class
		json.Unmarshal(listed["data"], &allClasses)
		if len(allClasses) == 0 || allClasses[0].ID != blockingClass.ID {
			t.Fatal("newest class is not first")
		}
	})
	call(SaveSemester, adminUser.ID, fmt.Sprint(term.ID), PeriodRequest{Name: term.Name, AcademicYearID: year.ID, StartDate: day(term.StartDate), EndDate: day(term.EndDate), Status: "closed"}, 200)
	call(AssignCourseOffering, adminUser.ID, fmt.Sprint(class.ID), req, 400)
	call(CreateAttendanceSession, tu2.ID, "", CreateAttendanceSessionRequest{CourseOfferingID: offering.ID}, 400)
	t.Run("self profile and password", func(t *testing.T) {
		for _, uid := range []uint{adminUser.ID, tu.ID} {
			profilePayload := call(GetOwnProfile, uid, "", nil, 200)
			var profile models.User
			json.Unmarshal(profilePayload["data"], &profile)
			if profile.ID != uid {
				t.Fatal("profile returned another account")
			}
			if bytes.Contains(profilePayload["data"], []byte(`"Password"`)) {
				t.Fatal("profile leaked password")
			}
			call(UpdateOwnProfile, uid, "", map[string]interface{}{"fullName": "Updated Profile", "email": profile.Email, "phone": "0901234567", "address": "Contact address", "roleId": studentRole.ID, "majorId": 999999}, 200)
			var updated models.User
			db.First(&updated, uid)
			if updated.FullName != "Updated Profile" || updated.RoleID != profile.RoleID {
				t.Fatal("profile did not update name or allowed role change")
			}
			if uid == tu.ID {
				var updatedTeacher models.Teacher
				db.First(&updatedTeacher, teacher.ID)
				if updatedTeacher.Phone != "0901234567" || updatedTeacher.MajorID != major.ID {
					t.Fatal("profile failed contact update or changed teaching major")
				}
			}
			call(UpdateOwnProfile, uid, "", map[string]string{"fullName": "Invalid overwrite", "email": "bad-email"}, 400)
			oldHash, err := bcrypt.GenerateFromPassword([]byte("OldSecret123"), bcrypt.DefaultCost)
			if err != nil {
				t.Fatal(err)
			}
			db.Model(&models.User{}).Where("id = ?", uid).Update("password", string(oldHash))
			call(ChangeOwnPassword, uid, "", map[string]string{"currentPassword": "wrong", "newPassword": "NewSecret123"}, 400)
			call(ChangeOwnPassword, uid, "", map[string]string{"currentPassword": "OldSecret123", "newPassword": "short"}, 400)
			call(ChangeOwnPassword, uid, "", map[string]string{"currentPassword": "OldSecret123", "newPassword": "NewSecret123"}, 200)
			db.First(&updated, uid)
			if bcrypt.CompareHashAndPassword([]byte(updated.Password), []byte("NewSecret123")) != nil || updated.FirstLogin {
				t.Fatal("password update failed")
			}
		}
	})
	t.Log("PASS: migration, academic periods, reusable class/course, term isolation, today/multi-date schedule, conflicts, student schedule, attendance, grading, transcript, exercises, registration, exams, closed term")
}
