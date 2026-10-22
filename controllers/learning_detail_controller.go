package controllers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"strconv"
	"student-management/config"
	"student-management/models"
)

// StudentLearningDetail is admin-only; the teacher endpoint scopes by offering owner.
func StudentLearningDetail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"message": "Invalid student ID"})
		return
	}
	var student models.Student
	if err := config.DB.First(&student, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "Student not found"})
		return
	}
	var rows []models.Enrollment
	err = learningDetailQuery().Where("enrollments.student_id = ?", id).Order("enrollments.id").Find(&rows).Error
	if err != nil {
		c.JSON(500, gin.H{"message": "Unable to load learning details"})
		return
	}
	c.JSON(200, gin.H{"data": rows})
}

func AdminClassLearningDetail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"message": "ID lớp không hợp lệ"})
		return
	}
	var rows []models.Enrollment
	if err := learningDetailQuery().Where("enrollments.class_id = ?", id).Order("enrollments.student_id, enrollments.id").Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"message": "Không tải được chuyên cần của lớp"})
		return
	}
	c.JSON(200, gin.H{"data": rows})
}

func TeacherClassLearningDetail(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"message": "Invalid class ID"})
		return
	}
	if !teacherOwnsClass(uint(id), teacher.ID) {
		c.JSON(403, gin.H{"message": "Teacher is not assigned to this class"})
		return
	}
	var rows []models.Enrollment
	err = learningDetailQuery().Joins("JOIN course_offerings ON course_offerings.id = enrollments.course_offering_id").Where("enrollments.class_id = ? AND course_offerings.teacher_id = ?", id, teacher.ID).Order("enrollments.student_id, enrollments.id").Find(&rows).Error
	if err != nil {
		c.JSON(500, gin.H{"message": "Unable to load class learning details"})
		return
	}
	c.JSON(200, gin.H{"data": rows})
}

func learningDetailQuery() *gorm.DB {
	return config.DB.Model(&models.Enrollment{}).Where("enrollments.status <> ?", "cancelled").
		Preload("Student.User").Preload("Class").Preload("Course").Preload("Grade").
		Preload("CourseOffering.Semester").Preload("CourseOffering.Room").Preload("CourseOffering.Teacher.User").
		Preload("CourseOffering.Schedules").Preload("Attendances", "attendance_session_id IS NULL OR attendance_session_id IN (SELECT id FROM attendance_sessions WHERE status = ?)", "confirmed").Preload("Attendances.AttendanceSession.Schedule")
}
