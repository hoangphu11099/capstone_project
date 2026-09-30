package controllers

import (
	"net/http"
	"strings"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
)

func Search(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("q"))
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Vui lòng nhập từ khóa tìm kiếm"})
		return
	}

	like := "%" + keyword + "%"

	var users []models.User
	if err := config.DB.Preload("Role").
		Where("username LIKE ? OR full_name LIKE ? OR email LIKE ?", like, like, like).
		Limit(20).
		Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi tìm kiếm người dùng", "error": err.Error()})
		return
	}

	var students []models.Student
	if err := config.DB.Preload("User").Preload("Class").
		Where("student_code LIKE ? OR phone LIKE ? OR address LIKE ?", like, like, like).
		Limit(20).
		Find(&students).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi tìm kiếm sinh viên", "error": err.Error()})
		return
	}

	var teachers []models.Teacher
	if err := config.DB.Preload("User").
		Where("teacher_code LIKE ? OR phone LIKE ? OR qualification LIKE ?", like, like, like).
		Limit(20).
		Find(&teachers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi tìm kiếm giảng viên", "error": err.Error()})
		return
	}

	var courses []models.Course
	if err := config.DB.Preload("Major").Preload("Semester").
		Where("code LIKE ? OR name LIKE ?", like, like).
		Limit(20).
		Find(&courses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi tìm kiếm môn học", "error": err.Error()})
		return
	}

	var classes []models.Class
	if err := config.DB.Preload("Major").Preload("Teacher.User").Preload("Room").Preload("Semester").
		Where("class_code LIKE ? OR status LIKE ?", like, like).
		Limit(20).
		Find(&classes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi tìm kiếm lớp", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Tìm kiếm thành công",
		"keyword":  keyword,
		"users":    users,
		"students": students,
		"teachers": teachers,
		"courses":  courses,
		"classes":  classes,
	})
}
