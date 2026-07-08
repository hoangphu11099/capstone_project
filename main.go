package main

import (
	"log"

	"student-management/config"
	"student-management/controllers"
	"student-management/middleware"
	// "student-management/seed"

	"github.com/gin-gonic/gin"
)

func main() {
	config.ConnectDB()
	config.AutoMigrateDB()

	// Chỉ mở dòng này khi cần tạo dữ liệu mẫu lại từ đầu.
	// seed.SeedData()

	r := gin.Default()
	//U-1: Login
	r.POST("/login", controllers.Login)
	//U-3: Forgot Password
	r.POST("/forgot-password", controllers.ForgotPassword)
	//U-4: Logout
	r.POST("/logout", middleware.AuthRequired(), controllers.Logout)

	auth := r.Group("/")
	auth.Use(middleware.AuthRequired())
	{
		// U-2: Change Password
		auth.POST("/change-password", controllers.ChangePassword)
		// U-5: View Schedule
		auth.GET("/schedule", controllers.ViewSchedule)
		// U-6: View Transcript
		auth.GET("/transcript", controllers.ViewTranscript)
		// U-07: Register Courses
		auth.GET("/course-classes/open", middleware.RoleRequired("student"), controllers.ListOpenCourseClasses)
		auth.POST("/course-registrations", middleware.RoleRequired("student"), controllers.RegisterCourse)

		// U-08: Create Class
		auth.GET("/classes", middleware.RoleRequired("admin"), controllers.ListClasses)
		auth.POST("/classes", middleware.RoleRequired("admin"), controllers.CreateClass)

		// U-09: Assign Teacher
		auth.PUT("/classes/:id/assign-teacher", middleware.RoleRequired("admin"), controllers.AssignTeacher)

		// U-10: Send Notification To Email
		auth.POST("/notifications/email", middleware.RoleRequired("admin"), controllers.SendNotificationToEmail)
		auth.GET("/notifications", controllers.ListNotifications)

		// U-11: View Class Offer
		auth.POST("/class-offers", middleware.RoleRequired("admin"), controllers.CreateClassOffer)
		// U-12: View Suggest Class
		auth.GET("/class-offers", middleware.RoleRequired("teacher"), controllers.ViewSuggestClass)
		// U-13: Accept Class Offer
		auth.POST("/class-offers/:id/accept", middleware.RoleRequired("teacher"), controllers.AcceptClassOffer)
		// U-13: Reject Class Offer
		auth.POST("/class-offers/:id/reject", middleware.RoleRequired("teacher"), controllers.RejectClassOffer)

		// U-14, U-15: Create Student + Delete Teacher
		auth.GET("/students", middleware.RoleRequired("admin"), controllers.ListStudents)
		auth.POST("/students", middleware.RoleRequired("admin"), controllers.CreateStudent)
		auth.GET("/teachers", middleware.RoleRequired("admin"), controllers.ListTeachers)
		auth.DELETE("/teachers/:id", middleware.RoleRequired("admin"), controllers.DeleteTeacher)

		// U-16, U-17: Dashboard Admin + Dashboard Student/Teacher
		auth.GET("/dashboard/admin", middleware.RoleRequired("admin"), controllers.DashboardAdmin)
		auth.GET("/dashboard", controllers.DashboardStudentTeacher)
		auth.GET("/dashboard/student", middleware.RoleRequired("student"), controllers.DashboardStudent)
		auth.GET("/dashboard/teacher", middleware.RoleRequired("teacher"), controllers.DashboardTeacher)

		// U-18: Search
		auth.GET("/search", middleware.RoleRequired("admin"), controllers.Search)

		// U-19: Cancel Course Registration
		auth.GET("/course-registrations", middleware.RoleRequired("student"), controllers.ListMyCourseRegistrations)
		auth.DELETE("/course-registrations/:id", middleware.RoleRequired("student"), controllers.CancelCourseRegistration)

		// U-20, U-21: Grade Management + Approve Grades
		auth.GET("/teacher/classes", middleware.RoleRequired("teacher"), controllers.ListTeacherClasses)
		auth.GET("/classes/:classID/grades", middleware.RoleRequired("teacher"), controllers.ListClassGrades)
		auth.POST("/grades", middleware.RoleRequired("teacher"), controllers.UpsertGrade)
		auth.PUT("/grades/:id", middleware.RoleRequired("teacher"), controllers.UpdateGrade)
		auth.POST("/grades/:id/approve", middleware.RoleRequired("teacher"), controllers.ApproveGrade)
		auth.POST("/classes/:classID/grades/approve", middleware.RoleRequired("teacher"), controllers.ApproveClassGrades)

		// U-22, U-26: Assignment Submission + Exercise
		auth.POST("/exercises", middleware.RoleRequired("teacher"), controllers.CreateExercise)
		auth.GET("/exercises", middleware.RoleRequired("teacher"), controllers.ListTeacherExercises)
		auth.GET("/exercises/:id/submissions", middleware.RoleRequired("teacher"), controllers.ListExerciseSubmissions)
		auth.GET("/student/exercises", middleware.RoleRequired("student"), controllers.ListStudentExercises)
		auth.POST("/exercises/:id/submissions", middleware.RoleRequired("student"), controllers.SubmitExercise)
		auth.GET("/student/submissions", middleware.RoleRequired("student"), controllers.ListMySubmissions)

		// U-23: Academic Warning
		auth.GET("/academic-warnings", middleware.RoleRequired("admin"), controllers.ListAcademicWarnings)
		auth.POST("/academic-warnings/generate", middleware.RoleRequired("admin"), controllers.GenerateAcademicWarnings)
		auth.DELETE("/academic-warnings/:id", middleware.RoleRequired("admin"), controllers.DeleteAcademicWarning)

		// U-24: Exam Schedule
		auth.GET("/exam-schedules", middleware.RoleRequired("admin"), controllers.ListExamSchedules)
		auth.POST("/exam-schedules", middleware.RoleRequired("admin"), controllers.CreateExamSchedule)
		auth.PUT("/exam-schedules/:id", middleware.RoleRequired("admin"), controllers.UpdateExamSchedule)
		auth.DELETE("/exam-schedules/:id", middleware.RoleRequired("admin"), controllers.DeleteExamSchedule)

		// U-25: Manage Course
		auth.GET("/courses", controllers.GetCourses)
		auth.GET("/courses/:id", controllers.GetCourseDetail)
		auth.POST("/courses", middleware.RoleRequired("admin"), controllers.CreateCourse)
		auth.PUT("/courses/:id", middleware.RoleRequired("admin"), controllers.UpdateCourse)
		auth.DELETE("/courses/:id", middleware.RoleRequired("admin"), controllers.DeleteCourse)
	}

	// TU-01: Attendance by QR - sinh viên quét QR, mã hiệu lực 1 phút
	auth.GET("/student/attendance/classes", middleware.RoleRequired("student"), controllers.ListStudentAttendanceClasses)
	auth.POST("/attendance/qr", middleware.RoleRequired("student"), controllers.AttendanceByQR)
	auth.GET("/student/attendances", middleware.RoleRequired("student"), controllers.ListMyAttendances)

	// TU-02: Take Attendance - giảng viên tạo phiên/mã QR điểm danh
	auth.GET("/teacher/attendance/classes", middleware.RoleRequired("teacher"), controllers.ListTeacherAttendanceClasses)
	auth.POST("/attendance/sessions", middleware.RoleRequired("teacher"), controllers.CreateAttendanceSession)
	auth.GET("/attendance/sessions/:id", middleware.RoleRequired("teacher"), controllers.GetAttendanceSession)
	auth.GET("/attendance/sessions/:id/qr", middleware.RoleRequired("teacher"), controllers.GetAttendanceSessionQRCode)
	auth.GET("/attendance/sessions/:id/records", middleware.RoleRequired("teacher"), controllers.ListAttendanceSessionRecords)
	auth.PUT("/attendances/:id", middleware.RoleRequired("teacher"), controllers.UpdateAttendanceStatus)

	// TU-03, TU-04: Submit grade + export transcript
	auth.POST("/submit-grade", middleware.RoleRequired("teacher"), controllers.SubmitGrade)
	auth.POST("/submit-grades", middleware.RoleRequired("teacher"), controllers.SubmitGrade)
	auth.GET("/transcripts/export", middleware.RoleRequired("teacher"), controllers.ExportTranscript)

	// TU-05: Import list teacher/student
	auth.POST("/import/teachers", middleware.RoleRequired("admin"), controllers.ImportListTeacher)
	auth.POST("/import/students", middleware.RoleRequired("admin"), controllers.ImportListStudent)

	// TU-06: Statistical report
	auth.GET("/statistical-report", middleware.RoleRequired("teacher"), controllers.ViewStatisticalReport)

	log.Println("Server running at http://localhost:8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("Cannot start server:", err)
	}
}
