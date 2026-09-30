package controllers

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"

	"student-management/config"
	"student-management/middleware"
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
	Username string `json:"username"`
	Email    string `json:"email"`
}

func generateTemporaryPassword() (string, error) {
	randomBytes := make([]byte, 9)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return "Tmp-" + base64.RawURLEncoding.EncodeToString(randomBytes), nil
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
		Where("username = ? OR email = ?", input.Username, input.Username).
		First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not found",
		})
		return
	}

	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "User account is inactive",
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

	userIDValue, exists := c.Get(middleware.ContextUserIDKey)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	identity := strings.TrimSpace(req.Username)
	if identity == "" {
		identity = strings.TrimSpace(req.Email)
	}
	if identity == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username or email is required"})
		return
	}

	var user models.User
	if err := config.DB.Where("username = ? OR email = ?", identity, identity).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	tempPass, err := generateTemporaryPassword()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Cannot generate temporary password",
		})
		return
	}

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
