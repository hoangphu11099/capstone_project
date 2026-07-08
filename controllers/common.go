package controllers

import (
	"net/http"
	"time"

	"student-management/config"
	"student-management/middleware"
	"student-management/models"

	"github.com/gin-gonic/gin"
)

func getCurrentUserID(c *gin.Context) (uint, bool) {
	value, exists := c.Get(middleware.ContextUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Vui lòng đăng nhập"})
		return 0, false
	}

	switch v := value.(type) {
	case uint:
		return v, true
	case int:
		return uint(v), true
	case int64:
		return uint(v), true
	case float64:
		return uint(v), true
	default:
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Token không hợp lệ"})
		return 0, false
	}
}

func getCurrentStudent(c *gin.Context) (models.Student, bool) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		return models.Student{}, false
	}

	var student models.Student
	if err := config.DB.Where("user_id = ?", userID).First(&student).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"message": "Tài khoản không phải sinh viên"})
		return models.Student{}, false
	}

	return student, true
}

func getCurrentTeacher(c *gin.Context) (models.Teacher, bool) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		return models.Teacher{}, false
	}

	var teacher models.Teacher
	if err := config.DB.Where("user_id = ?", userID).First(&teacher).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"message": "Tài khoản không phải giảng viên"})
		return models.Teacher{}, false
	}

	return teacher, true
}

func parseDateTime(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}

	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return parsed, nil
		}
		lastErr = err
	}

	return time.Time{}, lastErr
}
