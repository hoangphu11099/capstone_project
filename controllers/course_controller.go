package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CourseRequest struct {
	Code       string `json:"code" binding:"required"`
	Name       string `json:"name" binding:"required"`
	Credits    int    `json:"credits" binding:"required"`
	MajorID    uint   `json:"majorId" binding:"required"`
	SemesterID uint   `json:"semesterId" binding:"required"`
	IsActive   *bool  `json:"isActive"`
}

func GetCourses(c *gin.Context) {
	var courses []models.Course
	query := config.DB.Preload("Major").Preload("Semester")

	if keyword := strings.TrimSpace(c.Query("q")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("code LIKE ? OR name LIKE ?", like, like)
	}

	if majorID := strings.TrimSpace(c.Query("majorId")); majorID != "" {
		query = query.Where("major_id = ?", majorID)
	}

	if semesterID := strings.TrimSpace(c.Query("semesterId")); semesterID != "" {
		query = query.Where("semester_id = ?", semesterID)
	}

	if err := query.Order("code").Find(&courses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách môn học", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách môn học thành công", "data": courses})
}

func GetCourseDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID môn học không hợp lệ"})
		return
	}

	var course models.Course
	if err := config.DB.Preload("Major").Preload("Semester").First(&course, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy môn học"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy môn học", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy chi tiết môn học thành công", "data": course})
}

func CreateCourse(c *gin.Context) {
	var req CourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu môn học không hợp lệ", "error": err.Error()})
		return
	}

	if req.Credits <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Số tín chỉ phải lớn hơn 0"})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	course := models.Course{
		Code:       strings.TrimSpace(req.Code),
		Name:       strings.TrimSpace(req.Name),
		Credits:    req.Credits,
		MajorID:    req.MajorID,
		SemesterID: req.SemesterID,
		IsActive:   isActive,
	}

	if err := config.DB.Create(&course).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Không thể tạo môn học. Mã môn có thể đã tồn tại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Tạo môn học thành công", "data": course})
}

func UpdateCourse(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID môn học không hợp lệ"})
		return
	}

	var req CourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu môn học không hợp lệ", "error": err.Error()})
		return
	}

	if req.Credits <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Số tín chỉ phải lớn hơn 0"})
		return
	}

	var course models.Course
	if err := config.DB.First(&course, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy môn học"})
		return
	}

	course.Code = strings.TrimSpace(req.Code)
	course.Name = strings.TrimSpace(req.Name)
	course.Credits = req.Credits
	course.MajorID = req.MajorID
	course.SemesterID = req.SemesterID
	if req.IsActive != nil {
		course.IsActive = *req.IsActive
	}

	if err := config.DB.Save(&course).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Không thể cập nhật môn học", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cập nhật môn học thành công", "data": course})
}

func DeleteCourse(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID môn học không hợp lệ"})
		return
	}

	var count int64
	if err := config.DB.Model(&models.Enrollment{}).Where("course_id = ?", id).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi kiểm tra môn học", "error": err.Error()})
		return
	}

	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Môn học đã có sinh viên đăng ký nên không thể xóa"})
		return
	}

	if err := config.DB.Delete(&models.Course{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không thể xóa môn học", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Xóa môn học thành công"})
}
