package controllers

import (
	"net/http"
	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
)

func ViewSchedule(c *gin.Context) {

	userId, exists := c.Get("userId")

	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Vui lòng đăng nhập",
		})
		return
	}

	var student models.Student

	err := config.DB.
		Where("user_id = ?", userId).
		First(&student).Error

	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "Không phải sinh viên",
		})
		return
	}

	var schedules []models.ScheduleResponse

	err = config.DB.Table("course_registrations cr").
		Select(`
        c.course_code,
        c.course_name,
        t.full_name as teacher_name,
        s.room,
        s.day_of_week,
        s.period,
        s.start_date,
        s.end_date
    `).
		Joins("JOIN courses c ON cr.course_id = c.course_id").
		Joins("JOIN schedules s ON c.course_id = s.course_id").
		Joins("JOIN teachers t ON s.teacher_id = t.teacher_id").
		Where("cr.student_id = ?", student.StudentCode).
		Scan(&schedules).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Hệ thống lỗi",
		})
		return
	}

	if len(schedules) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Bạn chưa đăng ký môn học nào",
		})
		return
	}

	c.JSON(http.StatusOK, schedules)

}
