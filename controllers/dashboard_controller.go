package controllers

import (
	"net/http"
	"time"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
)

func DashboardAdmin(c *gin.Context) {
	var totalStudents, totalTeachers, totalClasses, pendingOffers, newNotifications int64
	config.DB.Model(&models.Student{}).Count(&totalStudents)
	config.DB.Model(&models.Teacher{}).Count(&totalTeachers)
	config.DB.Model(&models.Class{}).Count(&totalClasses)
	config.DB.Model(&models.ClassOffer{}).Where("status = ?", "pending").Count(&pendingOffers)
	config.DB.Model(&models.Notification{}).Where("created_at >= ?", time.Now().AddDate(0, 0, -7)).Count(&newNotifications)

	snapshot := models.AdminDashboard{
		TotalStudents:      int(totalStudents),
		TotalTeachers:      int(totalTeachers),
		TotalClasses:       int(totalClasses),
		PendingClassOffers: int(pendingOffers),
		NewNotifications:   int(newNotifications),
		GeneratedAt:        time.Now(),
	}
	_ = config.DB.Create(&snapshot).Error

	c.JSON(http.StatusOK, gin.H{
		"message": "Lấy dashboard admin thành công",
		"data": gin.H{
			"totalStudents":      totalStudents,
			"totalTeachers":      totalTeachers,
			"totalClasses":       totalClasses,
			"pendingClassOffers": pendingOffers,
			"newNotifications":   newNotifications,
		},
	})
}

func DashboardStudentTeacher(c *gin.Context) {
	role := c.GetString("role")
	if role == "student" {
		dashboardStudent(c)
		return
	}
	if role == "teacher" {
		dashboardTeacher(c)
		return
	}
	if role == "admin" {
		DashboardAdmin(c)
		return
	}
	c.JSON(http.StatusForbidden, gin.H{"message": "Không có quyền xem dashboard"})
}

func DashboardStudent(c *gin.Context) {
	dashboardStudent(c)
}

func DashboardTeacher(c *gin.Context) {
	dashboardTeacher(c)
}

func dashboardStudent(c *gin.Context) {
	student, ok := getCurrentStudent(c)
	if !ok {
		return
	}

	var enrolledCourses, exercises, submissions, attendances int64
	config.DB.Model(&models.Enrollment{}).Where("student_id = ? AND status <> ?", student.ID, "cancelled").Count(&enrolledCourses)
	config.DB.Model(&models.Exercise{}).
		Joins("JOIN enrollments ON enrollments.class_id = exercises.class_id").
		Where("enrollments.student_id = ? AND exercises.status = ?", student.ID, "open").Count(&exercises)
	config.DB.Model(&models.Submission{}).Where("student_id = ?", student.ID).Count(&submissions)
	config.DB.Model(&models.Attendance{}).
		Joins("JOIN enrollments ON enrollments.id = attendances.enrollment_id").
		Where("enrollments.student_id = ?", student.ID).Count(&attendances)

	c.JSON(http.StatusOK, gin.H{
		"message": "Lấy dashboard sinh viên thành công",
		"data": gin.H{
			"enrolledCourses": enrolledCourses,
			"openExercises":   exercises,
			"submissions":     submissions,
			"attendances":     attendances,
		},
	})
}

func dashboardTeacher(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	var classes, pendingOffers, exercises, pendingSubmissions int64
	config.DB.Model(&models.Class{}).Where("teacher_id = ?", teacher.ID).Count(&classes)
	config.DB.Model(&models.ClassOffer{}).Where("teacher_id = ? AND status = ?", teacher.ID, "pending").Count(&pendingOffers)
	config.DB.Model(&models.Exercise{}).Where("teacher_id = ?", teacher.ID).Count(&exercises)
	config.DB.Model(&models.Submission{}).
		Joins("JOIN exercises ON exercises.id = submissions.exercise_id").
		Where("exercises.teacher_id = ? AND submissions.score IS NULL", teacher.ID).Count(&pendingSubmissions)

	c.JSON(http.StatusOK, gin.H{
		"message": "Lấy dashboard giảng viên thành công",
		"data": gin.H{
			"classes":            classes,
			"pendingClassOffers": pendingOffers,
			"exercises":          exercises,
			"pendingSubmissions": pendingSubmissions,
		},
	})
}
