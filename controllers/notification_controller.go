package controllers

import (
	"net/http"
	"strings"
	"time"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
)

type SendNotificationRequest struct {
	RecipientUserID *uint  `json:"recipientUserId"`
	RecipientEmail  string `json:"recipientEmail"`
	Subject         string `json:"subject" binding:"required"`
	Content         string `json:"content" binding:"required"`
	SendNow         bool   `json:"sendNow"`
}

func SendNotificationToEmail(c *gin.Context) {
	currentUserID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	var req SendNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu thông báo không hợp lệ", "error": err.Error()})
		return
	}

	recipientEmail := strings.TrimSpace(req.RecipientEmail)
	if req.RecipientUserID != nil {
		var user models.User
		if err := config.DB.First(&user, *req.RecipientUserID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy người nhận"})
			return
		}
		if recipientEmail == "" {
			recipientEmail = user.Email
		}
	}

	if recipientEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Vui lòng nhập email người nhận hoặc recipientUserId"})
		return
	}

	status := "created"
	var sentAt *time.Time
	if req.SendNow {
		// Trong đồ án hiện tại chưa cấu hình SMTP thật, nên trạng thái được ghi nhận là sent để mô phỏng gửi email.
		now := time.Now()
		sentAt = &now
		status = "sent"
	}

	notification := models.Notification{
		RecipientUserID: req.RecipientUserID,
		RecipientEmail:  recipientEmail,
		Subject:         strings.TrimSpace(req.Subject),
		Content:         strings.TrimSpace(req.Content),
		Channel:         "email",
		Status:          status,
		SentAt:          sentAt,
		CreatedByID:     &currentUserID,
	}

	if err := config.DB.Create(&notification).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Tạo thông báo thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Tạo thông báo email thành công", "data": notification})
}

func ListNotifications(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	role := c.GetString("role")
	var notifications []models.Notification
	query := config.DB.Preload("RecipientUser").Order("created_at desc")
	if role != "admin" {
		query = query.Where("recipient_user_id = ?", userID)
	}

	if err := query.Limit(100).Find(&notifications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách thông báo", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách thông báo thành công", "data": notifications})
}
