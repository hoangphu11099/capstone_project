package controllers

import (
	"math"
	"net/http"
	"time"

	"student-management/config"
	"student-management/middleware"
	"student-management/models"

	"github.com/gin-gonic/gin"
)

func ViewStatisticalReport(c *gin.Context) {
	role := c.GetString(middleware.ContextRoleKey)

	if role == "teacher" {
		viewTeacherStatisticalReport(c)
		return
	}

	viewAdminStatisticalReport(c)
}

func viewAdminStatisticalReport(c *gin.Context) {
	var totalStudents int64
	var totalTeachers int64
	var totalClasses int64
	var totalCourses int64
	var totalEnrollments int64
	var totalWarnings int64
	var activeSemester models.Semester

	config.DB.Model(&models.Student{}).Count(&totalStudents)
	config.DB.Model(&models.Teacher{}).Count(&totalTeachers)
	config.DB.Model(&models.Class{}).Count(&totalClasses)
	config.DB.Model(&models.Course{}).Where("is_active = ?", true).Count(&totalCourses)
	config.DB.Model(&models.Enrollment{}).Where("status <> ?", "cancelled").Count(&totalEnrollments)
	config.DB.Model(&models.AcademicWarning{}).Count(&totalWarnings)
	config.DB.Where("status = ?", "active").First(&activeSemester)

	attendanceSummary := countAttendanceByStatus(0)
	gradeSummary := gradeSummaryByScope(0)

	c.JSON(http.StatusOK, gin.H{
		"message": "Lấy báo cáo thống kê thành công",
		"data": gin.H{
			"scope":            "admin",
			"generatedAt":      time.Now(),
			"activeSemester":   activeSemester,
			"totalStudents":    totalStudents,
			"totalTeachers":    totalTeachers,
			"totalClasses":     totalClasses,
			"totalCourses":     totalCourses,
			"totalEnrollments": totalEnrollments,
			"totalWarnings":    totalWarnings,
			"attendance":       attendanceSummary,
			"grades":           gradeSummary,
		},
	})
}

func viewTeacherStatisticalReport(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	var totalClasses int64
	var totalEnrollments int64
	var totalExercises int64
	var totalSubmissions int64

	config.DB.Model(&models.Class{}).Where("teacher_id = ?", teacher.ID).Count(&totalClasses)
	config.DB.Model(&models.Enrollment{}).
		Joins("JOIN classes ON classes.id = enrollments.class_id").
		Where("classes.teacher_id = ? AND enrollments.status <> ?", teacher.ID, "cancelled").
		Count(&totalEnrollments)
	config.DB.Model(&models.Exercise{}).Where("teacher_id = ?", teacher.ID).Count(&totalExercises)
	config.DB.Model(&models.Submission{}).
		Joins("JOIN exercises ON exercises.id = submissions.exercise_id").
		Where("exercises.teacher_id = ?", teacher.ID).
		Count(&totalSubmissions)

	attendanceSummary := countAttendanceByStatus(teacher.ID)
	gradeSummary := gradeSummaryByScope(teacher.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Lấy báo cáo thống kê giảng viên thành công",
		"data": gin.H{
			"scope":            "teacher",
			"generatedAt":      time.Now(),
			"totalClasses":     totalClasses,
			"totalEnrollments": totalEnrollments,
			"totalExercises":   totalExercises,
			"totalSubmissions": totalSubmissions,
			"attendance":       attendanceSummary,
			"grades":           gradeSummary,
		},
	})
}

func countAttendanceByStatus(teacherID uint) map[string]int64 {
	statuses := []string{"present", "absent", "late", "excused"}
	result := map[string]int64{}

	for _, status := range statuses {
		query := config.DB.Model(&models.Attendance{}).
			Joins("JOIN enrollments ON enrollments.id = attendances.enrollment_id").
			Joins("JOIN classes ON classes.id = enrollments.class_id").
			Where("attendances.status = ?", status)
		if teacherID > 0 {
			query = query.Where("classes.teacher_id = ?", teacherID)
		}
		var count int64
		query.Count(&count)
		result[status] = count
	}

	return result
}

func gradeSummaryByScope(teacherID uint) gin.H {
	query := config.DB.Table("grades g").
		Joins("JOIN enrollments e ON e.id = g.enrollment_id").
		Joins("JOIN classes cl ON cl.id = e.class_id")
	if teacherID > 0 {
		query = query.Where("cl.teacher_id = ?", teacherID)
	}

	var total int64
	query.Count(&total)

	var avg struct {
		Average float64
	}
	query.Select("AVG(COALESCE(NULLIF(g.total_score, 0), g.score, 0)) AS average").Scan(&avg)

	letterCounts := map[string]int64{}
	letters := []string{"A", "B+", "B", "C+", "C", "D+", "D", "F"}
	for _, letter := range letters {
		letterQuery := config.DB.Table("grades g").
			Joins("JOIN enrollments e ON e.id = g.enrollment_id").
			Joins("JOIN classes cl ON cl.id = e.class_id").
			Where("COALESCE(NULLIF(g.grade_letter, ''), g.letter_grade, '') = ?", letter)
		if teacherID > 0 {
			letterQuery = letterQuery.Where("cl.teacher_id = ?", teacherID)
		}
		var count int64
		letterQuery.Count(&count)
		letterCounts[letter] = count
	}

	return gin.H{
		"totalGrades":  total,
		"averageScore": math.Round(avg.Average*100) / 100,
		"byLetter":     letterCounts,
	}
}
