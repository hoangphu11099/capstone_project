package controllers

import (
	"net/http"

	"student-management/config"
	"student-management/models"
	"student-management/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type ChangePasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required"`
}

type ForgotPasswordRequest struct {
	Username string `json:"username" binding:"required"`
}

func Login(c *gin.Context) {
	var input LoginRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Username and password are required",
		})
		return
	}

	var user models.User
	if err := config.DB.Preload("Role").
		Where("username = ?", input.Username).
		First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not found",
		})
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Wrong password",
		})
		return
	}

	token, err := utils.GenerateToken(user.ID, user.Role.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Cannot generate token",
		})
		return
	}

	if user.FirstLogin {
		c.JSON(http.StatusOK, gin.H{
			"message":     "First login, please change password",
			"token":       token,
			"first_login": true,
			"user": gin.H{
				"id":       user.ID,
				"username": user.Username,
				"fullName": user.FullName,
				"role":     user.Role.Name,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Login successfully",
		"token":       token,
		"first_login": false,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"fullName": user.FullName,
			"role":     user.Role.Name,
		},
	})
}

func ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest

	userIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "New password is required",
		})
		return
	}

	if len(req.NewPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "New password must be at least 6 characters",
		})
		return
	}

	var userID uint

	switch v := userIDValue.(type) {
	case uint:
		userID = v
	case float64:
		userID = uint(v)
	case int:
		userID = uint(v)
	default:
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid user ID in token",
		})
		return
	}

	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Cannot hash password",
		})
		return
	}

	user.Password = string(hash)
	user.FirstLogin = false

	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Cannot update password",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Password changed successfully",
		"first_login": false,
	})
}

func ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Username is required",
		})
		return
	}

	var user models.User
	if err := config.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	tempPass := "Temp1234"

	hash, err := bcrypt.GenerateFromPassword([]byte(tempPass), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Cannot hash temporary password",
		})
		return
	}

	user.Password = string(hash)
	user.FirstLogin = true

	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Cannot reset password",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Temporary password set. User must change password on first login.",
		"temp_password": tempPass,
		"first_login":   true,
	})
}

func Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Đăng xuất thành công",
	})
}

// // Lấy bảng điểm môn học của sinh viên
// func GetStudentTranscript(c *gin.Context) {
// 	studentID := c.Param("studentId")

// 	var student models.Student

// 	// Lấy sinh viên cùng danh sách môn học và điểm
// 	err := config.DB.
// 		Preload("Enrollments.Course").
// 		Preload("Enrollments.Grade").
// 		First(&student, studentID).Error

// 	if err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{
// 			"message": "Không tìm thấy sinh viên",
// 		})
// 		return
// 	}

// 	// Khởi tạo dữ liệu bảng điểm trả về
// 	result := responses.TranscriptResponse{
// 		StudentID:   student.ID,
// 		StudentCode: student.StudentCode,
// 		StudentName: student.FullName,
// 		Courses:     []responses.TranscriptItem{},
// 	}

// 	// Duyệt các môn sinh viên đã đăng ký
// 	for _, enrollment := range student.Enrollments {
// 		item := responses.TranscriptItem{
// 			CourseCode: enrollment.Course.Code,
// 			CourseName: enrollment.Course.Name,
// 			Credits:    enrollment.Course.Credits,

// 			AssignmentScore: enrollment.Grade.AssignmentScore,
// 			MidtermScore:    enrollment.Grade.MidtermScore,
// 			FinalScore:      enrollment.Grade.FinalScore,

// 			TotalScore:  enrollment.Grade.TotalScore,
// 			GradeLetter: enrollment.Grade.GradeLetter,
// 		}

// 		result.Courses = append(result.Courses, item)
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"message": "Lấy bảng điểm thành công",
// 		"data":    result,
// 	})
// }
