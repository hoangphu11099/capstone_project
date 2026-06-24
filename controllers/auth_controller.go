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
	Username string `json:"username"`
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	NewPassword string `json:"new_password"`
}

type ForgotPasswordRequest struct {
	Username string `json:"username"`
}

// func login
func Login(c *gin.Context) {
	var input LoginRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	config.DB.Where("username = ?", input.Username).First(&user)
	if user.ID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Wrong password"})
		return
	}

	// Nếu lần đầu login thì yeu cầu đổi mật khẩu
	if user.FirstLogin {
		c.JSON(http.StatusOK, gin.H{
			"message":     "First login, please change password",
			"first_login": true,
		})
		return
	}

	token, _ := utils.GenerateToken(user.ID, user.Role)
	c.JSON(http.StatusOK, gin.H{
		"token":       token,
		"first_login": false,
	})
}

//func Change Password

func ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	userID := c.GetUint("userID")

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	config.DB.First(&user, userID)

	hash, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	user.Password = string(hash)
	user.FirstLogin = false
	config.DB.Save(&user)

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// ----------------------
// Forgot Password
// ----------------------
func ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	config.DB.Where("username = ?", req.Username).First(&user)
	if user.ID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Tạo mật khẩu tạm
	tempPass := "Temp1234"
	hash, _ := bcrypt.GenerateFromPassword([]byte(tempPass), bcrypt.DefaultCost)
	user.Password = string(hash)
	user.FirstLogin = true
	config.DB.Save(&user)

	c.JSON(http.StatusOK, gin.H{
		"message":       "Temporary password set. User must change password on first login.",
		"temp_password": tempPass,
	})
}
