package controllers

import (
	"net/http"

	"student-management/config"
	"student-management/middleware"
	"student-management/models"
	"student-management/responses"

	"github.com/gin-gonic/gin"
)

func ViewSchedule(c *gin.Context) {
	userID, exists := c.Get(middleware.ContextUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Vui lòng đăng nhập",
		})
		return
	}

	var student models.Student
	if err := config.DB.Where("user_id = ?", userID).First(&student).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "Không phải sinh viên",
		})
		return
	}

	var schedules []responses.ScheduleResponse
	err := config.DB.Table("enrollments e").
		Select(`
			c.code AS course_code,
			c.name AS course_name,
			u.full_name AS teacher_name,
			r.name AS room,
			s.day_of_week,
			s.session AS period,
			DATE_FORMAT(sem.start_date, '%Y-%m-%d') AS start_date,
			DATE_FORMAT(sem.end_date, '%Y-%m-%d') AS end_date
		`).
		Joins("JOIN courses c ON e.course_id = c.id").
		Joins("JOIN classes cl ON e.class_id = cl.id").
		Joins("JOIN schedules s ON s.class_id = cl.id").
		Joins("JOIN teachers t ON cl.teacher_id = t.id").
		Joins("JOIN users u ON t.user_id = u.id").
		Joins("LEFT JOIN rooms r ON cl.room_id = r.id").
		Joins("LEFT JOIN semesters sem ON cl.semester_id = sem.id").
		Where("e.student_id = ?", student.ID).
		Order("s.day_of_week, s.start_time").
		Scan(&schedules).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Hệ thống lỗi",
			"error":   err.Error(),
		})
		return
	}

	if len(schedules) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Bạn chưa đăng ký môn học nào",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Lấy thời khóa biểu thành công",
		"data":    schedules,
	})
}
