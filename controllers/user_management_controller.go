package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type CreateStudentRequest struct {
	Username       string `json:"username" binding:"required"`
	Password       string `json:"password"`
	Email          string `json:"email"`
	FullName       string `json:"fullName" binding:"required"`
	StudentCode    string `json:"studentCode" binding:"required"`
	ClassID        uint   `json:"classId" binding:"required"`
	DateOfBirth    string `json:"dateOfBirth"`
	Gender         string `json:"gender"`
	Phone          string `json:"phone"`
	Address        string `json:"address"`
	EnrollmentDate string `json:"enrollmentDate"`
	Status         string `json:"status"`
}

func CreateStudent(c *gin.Context) {
	var req CreateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu tạo sinh viên không hợp lệ", "error": err.Error()})
		return
	}

	password := strings.TrimSpace(req.Password)
	if password == "" {
		password = "Student@123"
	}
	if len(password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Mật khẩu phải có ít nhất 6 ký tự"})
		return
	}

	roleID, err := getRoleIDByName("student")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tìm thấy role student", "error": err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không thể mã hóa mật khẩu", "error": err.Error()})
		return
	}

	dateOfBirth, err := parseOptionalDate(req.DateOfBirth)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ngày sinh không hợp lệ. Dùng định dạng YYYY-MM-DD"})
		return
	}

	enrollmentDate, err := parseOptionalDate(req.EnrollmentDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ngày nhập học không hợp lệ. Dùng định dạng YYYY-MM-DD"})
		return
	}
	if enrollmentDate.IsZero() {
		enrollmentDate = time.Now()
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "active"
	}

	var class models.Class
	if err := config.DB.First(&class, req.ClassID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy lớp của sinh viên"})
		return
	}

	user := models.User{
		Username:   strings.TrimSpace(req.Username),
		Password:   string(hash),
		Email:      strings.TrimSpace(req.Email),
		FullName:   strings.TrimSpace(req.FullName),
		RoleID:     roleID,
		IsActive:   true,
		FirstLogin: true,
	}

	student := models.Student{
		StudentCode:    strings.TrimSpace(req.StudentCode),
		ClassID:        req.ClassID,
		DateOfBirth:    dateOfBirth,
		Gender:         strings.TrimSpace(req.Gender),
		Phone:          strings.TrimSpace(req.Phone),
		Address:        strings.TrimSpace(req.Address),
		EnrollmentDate: enrollmentDate,
		Status:         status,
	}

	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		student.UserID = user.ID
		return tx.Create(&student).Error
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Tạo sinh viên thất bại. Username/email/mã sinh viên có thể đã tồn tại", "error": err.Error()})
		return
	}

	config.DB.Preload("User").Preload("Class").First(&student, student.ID)
	c.JSON(http.StatusCreated, gin.H{"message": "Tạo sinh viên thành công", "data": student, "defaultPassword": password})
}

func ListStudents(c *gin.Context) {
	var students []models.Student
	query := config.DB.Preload("User").Preload("Class")
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		like := "%" + q + "%"
		query = query.Joins("JOIN users ON users.id = students.user_id").
			Where("students.student_code LIKE ? OR users.full_name LIKE ? OR users.username LIKE ?", like, like, like)
	}
	if err := query.Order("students.id desc").Find(&students).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách sinh viên", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách sinh viên thành công", "data": students})
}

func DeleteTeacher(c *gin.Context) {
	teacherID, err := strconv.Atoi(c.Param("id"))
	if err != nil || teacherID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID giảng viên không hợp lệ"})
		return
	}

	var teacher models.Teacher
	if err := config.DB.First(&teacher, teacherID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy giảng viên"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy giảng viên", "error": err.Error()})
		return
	}

	var classCount int64
	if err := config.DB.Model(&models.Class{}).Where("teacher_id = ?", teacher.ID).Count(&classCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi kiểm tra lớp giảng dạy", "error": err.Error()})
		return
	}
	if classCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Giảng viên đang được phân công lớp nên không thể xóa. Hãy đổi giảng viên phụ trách trước"})
		return
	}

	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("teacher_id = ?", teacher.ID).Delete(&models.ClassOffer{}).Error; err != nil {
			return err
		}
		if err := tx.Where("teacher_id = ?", teacher.ID).Delete(&models.ClassTeacher{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&teacher).Error; err != nil {
			return err
		}
		return tx.Delete(&models.User{}, teacher.UserID).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Xóa giảng viên thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Xóa giảng viên thành công"})
}

func ListTeachers(c *gin.Context) {
	var teachers []models.Teacher
	query := config.DB.Preload("User")
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		like := "%" + q + "%"
		query = query.Joins("JOIN users ON users.id = teachers.user_id").
			Where("teachers.teacher_code LIKE ? OR users.full_name LIKE ? OR users.username LIKE ?", like, like, like)
	}
	if err := query.Order("teachers.id desc").Find(&teachers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách giảng viên", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách giảng viên thành công", "data": teachers})
}

func parseOptionalDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q", value)
	}
	return date, nil
}
