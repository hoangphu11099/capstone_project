package controllers

import (
	"net/http"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
)

// GetMetadata trả về dữ liệu tham chiếu phục vụ dropdown/form trên web dashboard.
// Endpoint này tránh việc người dùng phải nhớ và nhập ID thủ công.
func GetMetadata(c *gin.Context) {
	var majors []models.Major
	var semesters []models.Semester
	var rooms []models.Room
	var courses []models.Course
	var teachers []models.Teacher
	var classes []models.Class

	if err := config.DB.Where("is_active = ?", true).Order("code").Find(&majors).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách chuyên ngành", "error": err.Error()})
		return
	}
	if err := config.DB.Order("start_date desc").Find(&semesters).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách học kỳ", "error": err.Error()})
		return
	}
	if err := config.DB.Where("is_active = ?", true).Order("name").Find(&rooms).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách phòng", "error": err.Error()})
		return
	}
	if err := config.DB.Preload("Major").Preload("Semester").Where("is_active = ?", true).Order("code").Find(&courses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách môn học", "error": err.Error()})
		return
	}
	if err := config.DB.Preload("User").Order("teacher_code").Find(&teachers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách giảng viên", "error": err.Error()})
		return
	}
	if err := config.DB.Preload("Major").Preload("Teacher.User").Preload("Room").Preload("Semester").Order("class_code").Find(&classes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách lớp", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Lấy dữ liệu tham chiếu thành công",
		"data": gin.H{
			"majors":    majors,
			"semesters": semesters,
			"rooms":     rooms,
			"courses":   courses,
			"teachers":  teachers,
			"classes":   classes,
		},
	})
}
