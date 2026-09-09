package controllers

import (
	"fmt"
	"mime"
	"net/http"
	"net/smtp"
	"os"
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
	var deliveryErr error
	if req.SendNow {
		deliveryErr = sendSMTPEmail(recipientEmail, strings.TrimSpace(req.Subject), strings.TrimSpace(req.Content))
		if deliveryErr == nil {
			now := time.Now()
			sentAt = &now
			status = "sent"
		} else {
			status = "failed"
		}
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

	if deliveryErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"message": "Đã lưu thông báo nhưng gửi email thất bại. Kiểm tra cấu hình SMTP",
			"error":   deliveryErr.Error(),
			"data":    notification,
		})
		return
	}

	message := "Tạo thông báo email thành công"
	if req.SendNow {
		message = "Gửi email thành công"
	}
	c.JSON(http.StatusCreated, gin.H{"message": message, "data": notification})
}

func sendSMTPEmail(to, subject, content string) error {
	if strings.ContainsAny(to, "\r\n") || strings.ContainsAny(subject, "\r\n") {
		return fmt.Errorf("email hoặc tiêu đề không hợp lệ")
	}
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	username := strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
	password := os.Getenv("SMTP_PASSWORD")
	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))

	if host == "" || port == "" {
		return fmt.Errorf("SMTP_HOST và SMTP_PORT chưa được cấu hình")
	}
	if from == "" {
		from = username
	}
	if from == "" {
		return fmt.Errorf("SMTP_FROM hoặc SMTP_USERNAME chưa được cấu hình")
	}

	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	message := []byte(
		"From: " + from + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n" +
			"\r\n" + content,
	)

	return smtp.SendMail(host+":"+port, auth, from, []string{to}, message)
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
