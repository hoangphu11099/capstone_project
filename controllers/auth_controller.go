package controllers

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"strings"
	"time"

	"student-management/config"
	"student-management/middleware"
	"student-management/models"
	"student-management/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Username string `json:"username" binding:"required,max=254"`
	Password string `json:"password" binding:"required,max=72"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password" binding:"required"`
}

type ForgotPasswordRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

func issueLoginToken(tx *gorm.DB, user models.User) (string, error) {
	token, err := utils.GenerateToken(user.ID, user.Role.Name)
	if err != nil {
		return "", err
	}
	claims, err := utils.ParseToken(token)
	if err != nil {
		return "", err
	}
	session := models.AuthSession{ID: claims.ID, UserID: user.ID, CredentialFingerprint: utils.CredentialFingerprint(user.Password), ExpiresAt: claims.ExpiresAt.Time}
	if err = tx.Create(&session).Error; err != nil {
		return "", err
	}
	return token, nil
}

func Login(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	var input LoginRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Vui lòng nhập tên đăng nhập và mật khẩu hợp lệ",
		})
		return
	}
	var user models.User
	if err := config.DB.Preload("Role").
		Where("username = ? OR email = ?", input.Username, input.Username).
		First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Tên đăng nhập hoặc mật khẩu không đúng",
		})
		return
	}

	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Tài khoản đã bị vô hiệu hóa. Vui lòng liên hệ quản trị viên",
		})
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Tên đăng nhập hoặc mật khẩu không đúng",
		})
		return
	}

	token, err := issueLoginToken(config.DB, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Không thể đăng nhập. Vui lòng thử lại sau",
		})
		return
	}

	if user.FirstLogin {
		c.JSON(http.StatusOK, gin.H{
			"message":     "Đây là lần đăng nhập đầu tiên. Vui lòng đổi mật khẩu",
			"token":       token,
			"token_type":  "Bearer",
			"expires_in":  int(utils.AccessTokenLifetime / time.Second),
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
		"message":     "Đăng nhập thành công",
		"token":       token,
		"token_type":  "Bearer",
		"expires_in":  int(utils.AccessTokenLifetime / time.Second),
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

	if len(req.NewPassword) < 6 || len(req.NewPassword) > 72 {
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

	if !user.FirstLogin && bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)) != nil {
		c.JSON(400, gin.H{"message": "Current password is incorrect"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.NewPassword)) == nil {
		c.JSON(400, gin.H{"message": "New password must differ from current password"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Cannot hash password",
		})
		return
	}

	var token string
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		var current models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Role").First(&current, user.ID).Error; err != nil {
			return err
		}
		if current.Password != user.Password {
			return gorm.ErrInvalidData
		}
		current.Password = string(hash)
		current.FirstLogin = false
		if err := tx.Save(&current).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.AuthSession{}).Error; err != nil {
			return err
		}
		var err error
		token, err = issueLoginToken(tx, current)
		return err
	})
	if err != nil {
		c.JSON(500, gin.H{"message": "Cannot update password"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"message": "Password changed successfully", "first_login": false, "token": token, "token_type": "Bearer", "expires_in": int(utils.AccessTokenLifetime / time.Second)})
}

func ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if c.ShouldBindJSON(&req) != nil || (strings.TrimSpace(req.Username) == "" && strings.TrimSpace(req.Email) == "") {
		c.JSON(400, gin.H{"message": "Username or email is required"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(202, gin.H{"message": "Vui lòng liên hệ quản trị viên hoặc Bộ phận Đào tạo để xác minh danh tính và được cấp lại mật khẩu."})
}

func Logout(c *gin.Context) {
	if err := config.DB.Where("id = ?", c.GetString(middleware.ContextSessionIDKey)).Delete(&models.AuthSession{}).Error; err != nil {
		c.JSON(500, gin.H{"message": "Không thể kết thúc phiên đăng nhập. Vui lòng thử lại."})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"success": true, "message": "Đăng xuất thành công"})
}
