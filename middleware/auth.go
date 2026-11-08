package middleware

import (
	"net/http"
	"strings"
	"student-management/config"
	"student-management/models"
	"time"

	"student-management/utils"

	"github.com/gin-gonic/gin"
)

const (
	ContextUserIDKey    = "userID"
	ContextRoleKey      = "role"
	ContextSessionIDKey = "sessionID"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No token"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header"})
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(parts[1])

		claims, err := utils.ParseToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Phiên đăng nhập không hợp lệ hoặc đã hết hạn. Vui lòng đăng nhập lại."})
			c.Abort()
			return
		}

		var session models.AuthSession
		var user models.User
		if config.DB == nil || config.DB.Where("id = ? AND user_id = ? AND expires_at > ?", claims.ID, claims.UserID, time.Now()).First(&session).Error != nil || config.DB.Preload("Role").First(&user, claims.UserID).Error != nil || !user.IsActive || user.Role.Name != claims.Role || session.CredentialFingerprint != utils.CredentialFingerprint(user.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Phiên đăng nhập không hợp lệ hoặc đã hết hạn. Vui lòng đăng nhập lại."})
			c.Abort()
			return
		}
		if user.FirstLogin && c.Request.URL.Path != "/change-password" && c.Request.URL.Path != "/logout" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "PASSWORD_CHANGE_REQUIRED", "message": "Vui lòng đổi mật khẩu lần đầu trước khi sử dụng hệ thống."})
			return
		}

		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextRoleKey, claims.Role)
		c.Set(ContextSessionIDKey, claims.ID)

		c.Next()
	}
}

func RoleRequired(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := c.GetString(ContextRoleKey)

		if userRole != role && userRole != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
			c.Abort()
			return
		}

		c.Next()
	}
}
