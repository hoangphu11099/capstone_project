package controllers

import (
	"fmt"
	"mime"
	"net/http"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxNotificationRecipients = 1000

type SendNotificationRequest struct {
	TargetType      string `json:"targetType"`
	RecipientUserID *uint  `json:"recipientUserId"`
	RecipientEmail  string `json:"recipientEmail"`
	ClassID         uint   `json:"classId"`
	Cohort          string `json:"cohort"`
	Subject         string `json:"subject" binding:"required"`
	Content         string `json:"content" binding:"required"`
	SendNow         bool   `json:"sendNow"`
}

type notificationRecipient struct {
	UserID   *uint
	Email    string
	FullName string
}

// SendNotificationToEmail hỗ trợ cả thông báo trong app và email. Endpoint cũ
// được giữ nguyên để không làm hỏng các client đang sử dụng /notifications/email.
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

	req.Subject = strings.TrimSpace(req.Subject)
	req.Content = strings.TrimSpace(req.Content)
	if req.Subject == "" || req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Tiêu đề và nội dung không được để trống"})
		return
	}

	recipients, targetType, err := resolveNotificationRecipients(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if len(recipients) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Không tìm thấy người nhận phù hợp"})
		return
	}
	if len(recipients) > maxNotificationRecipients {
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Số người nhận vượt quá giới hạn %d", maxNotificationRecipients)})
		return
	}

	created, sent, failed := 0, 0, 0
	errorsList := make([]string, 0)
	for _, recipient := range recipients {
		status := "created"
		channel := "app"
		var sentAt *time.Time
		var deliveryErr error

		if req.SendNow {
			if strings.TrimSpace(recipient.Email) == "" {
				deliveryErr = fmt.Errorf("%s chưa có email", recipient.FullName)
			} else {
				deliveryErr = sendSMTPEmail(recipient.Email, req.Subject, req.Content)
			}
			if deliveryErr == nil {
				now := time.Now()
				sentAt = &now
				status = "sent"
				if recipient.UserID == nil {
					channel = "email"
				} else {
					channel = "app,email"
				}
				sent++
			} else {
				status = "failed"
				failed++
				if len(errorsList) < 5 {
					errorsList = append(errorsList, deliveryErr.Error())
				}
			}
		}

		notification := models.Notification{
			RecipientUserID: recipient.UserID,
			RecipientEmail:  strings.TrimSpace(recipient.Email),
			Subject:         req.Subject,
			Content:         req.Content,
			TargetType:      targetType,
			Channel:         channel,
			Status:          status,
			SentAt:          sentAt,
			CreatedByID:     &currentUserID,
		}
		if err := config.DB.Create(&notification).Error; err != nil {
			failed++
			if len(errorsList) < 5 {
				errorsList = append(errorsList, "Không lưu được thông báo cho "+recipient.FullName)
			}
			continue
		}
		created++
	}

	message := fmt.Sprintf("Đã tạo %d thông báo trong ứng dụng", created)
	if req.SendNow {
		message = fmt.Sprintf("Đã tạo %d thông báo · Email thành công %d · Thất bại %d", created, sent, failed)
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": message,
		"result": gin.H{
			"targetType": targetType,
			"total":      len(recipients),
			"created":    created,
			"sent":       sent,
			"failed":     failed,
			"errors":     errorsList,
		},
	})
}

func resolveNotificationRecipients(req SendNotificationRequest) ([]notificationRecipient, string, error) {
	targetType := strings.ToLower(strings.TrimSpace(req.TargetType))
	if targetType == "" {
		if req.RecipientUserID != nil {
			targetType = "user"
		} else {
			targetType = "email"
		}
	}

	var users []models.User
	switch targetType {
	case "student", "teacher", "user":
		if req.RecipientUserID == nil || *req.RecipientUserID == 0 {
			return nil, targetType, fmt.Errorf("vui lòng chọn người nhận")
		}
		if err := config.DB.Where("id = ? AND is_active = ?", *req.RecipientUserID, true).Find(&users).Error; err != nil {
			return nil, targetType, err
		}
	case "class":
		if req.ClassID == 0 {
			return nil, targetType, fmt.Errorf("vui lòng chọn lớp")
		}
		if err := config.DB.Model(&models.User{}).
			Joins("JOIN students ON students.user_id = users.id").
			Where("students.class_id = ? AND users.is_active = ?", req.ClassID, true).
			Order("users.full_name").Find(&users).Error; err != nil {
			return nil, targetType, err
		}
	case "cohort":
		cohort := strings.ToUpper(strings.TrimSpace(req.Cohort))
		if cohort == "" {
			return nil, targetType, fmt.Errorf("vui lòng chọn khóa")
		}
		if err := config.DB.Model(&models.User{}).
			Joins("JOIN students ON students.user_id = users.id").
			Joins("JOIN classes ON classes.id = students.class_id").
			Where("(classes.class_code = ? OR classes.class_code LIKE ?) AND users.is_active = ?", cohort, cohort+"\\_%", true).
			Order("users.full_name").Find(&users).Error; err != nil {
			return nil, targetType, err
		}
	case "all_students", "all_teachers":
		roleName := "student"
		if targetType == "all_teachers" {
			roleName = "teacher"
		}
		if err := config.DB.Model(&models.User{}).
			Joins("JOIN roles ON roles.id = users.role_id").
			Where("LOWER(roles.name) = ? AND users.is_active = ?", roleName, true).
			Order("users.full_name").Find(&users).Error; err != nil {
			return nil, targetType, err
		}
	case "email":
		email := strings.TrimSpace(req.RecipientEmail)
		if _, err := mail.ParseAddress(email); err != nil {
			return nil, targetType, fmt.Errorf("email người nhận không hợp lệ")
		}
		return []notificationRecipient{{Email: email, FullName: email}}, targetType, nil
	default:
		return nil, targetType, fmt.Errorf("đối tượng nhận không hợp lệ")
	}

	result := make([]notificationRecipient, 0, len(users))
	seen := make(map[uint]bool)
	for _, user := range users {
		if seen[user.ID] {
			continue
		}
		seen[user.ID] = true
		userID := user.ID
		result = append(result, notificationRecipient{UserID: &userID, Email: user.Email, FullName: user.FullName})
	}
	return result, targetType, nil
}

// GetNotificationRecipients trả dữ liệu gọn cho các bộ lọc người nhận của admin.
func GetNotificationRecipients(c *gin.Context) {
	type classOption struct {
		ID           uint   `json:"id"`
		ClassCode    string `json:"classCode"`
		Cohort       string `json:"cohort"`
		StudentCount int64  `json:"studentCount"`
	}
	type personOption struct {
		UserID    uint   `json:"userId"`
		Code      string `json:"code"`
		FullName  string `json:"fullName"`
		Email     string `json:"email"`
		ClassCode string `json:"classCode,omitempty"`
		Cohort    string `json:"cohort,omitempty"`
	}
	type cohortOption struct {
		Code         string `json:"code"`
		StudentCount int64  `json:"studentCount"`
	}

	var classes []classOption
	if err := config.DB.Table("classes c").
		Select("c.id, c.class_code, SUBSTRING_INDEX(c.class_code, '_', 1) AS cohort, COUNT(s.id) AS student_count").
		Joins("LEFT JOIN students s ON s.class_id = c.id").
		Group("c.id, c.class_code").Order("c.class_code").Scan(&classes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tải được danh sách lớp", "error": err.Error()})
		return
	}

	var cohorts []cohortOption
	if err := config.DB.Table("classes c").
		Select("SUBSTRING_INDEX(c.class_code, '_', 1) AS code, COUNT(s.id) AS student_count").
		Joins("LEFT JOIN students s ON s.class_id = c.id").
		Where("c.class_code <> ''").Group("code").Order("code DESC").Scan(&cohorts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tải được danh sách khóa", "error": err.Error()})
		return
	}

	var students []personOption
	if err := config.DB.Table("students s").
		Select("u.id AS user_id, s.student_code AS code, u.full_name, u.email, c.class_code, SUBSTRING_INDEX(c.class_code, '_', 1) AS cohort").
		Joins("JOIN users u ON u.id = s.user_id").Joins("LEFT JOIN classes c ON c.id = s.class_id").
		Where("u.is_active = ?", true).Order("c.class_code, u.full_name").Scan(&students).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tải được danh sách sinh viên", "error": err.Error()})
		return
	}

	var teachers []personOption
	if err := config.DB.Table("teachers t").
		Select("u.id AS user_id, t.teacher_code AS code, u.full_name, u.email").
		Joins("JOIN users u ON u.id = t.user_id").Where("u.is_active = ?", true).
		Order("u.full_name").Scan(&teachers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tải được danh sách giảng viên", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy đối tượng nhận thành công", "data": gin.H{
		"classes": classes, "cohorts": cohorts, "students": students, "teachers": teachers,
	}})
}

func sendSMTPEmail(to, subject, content string) error {
	if strings.ContainsAny(to, "\r\n") || strings.ContainsAny(subject, "\r\n") {
		return fmt.Errorf("email hoặc tiêu đề không hợp lệ")
	}
	if _, err := mail.ParseAddress(to); err != nil {
		return fmt.Errorf("email %q không hợp lệ", to)
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
	message := []byte("From: " + from + "\r\n" + "To: " + to + "\r\n" +
		"Subject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\n" +
		"MIME-Version: 1.0\r\n" + "Content-Type: text/plain; charset=UTF-8\r\n\r\n" + content)
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
	if err := query.Limit(500).Find(&notifications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách thông báo", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách thông báo thành công", "data": notifications})
}

func MarkNotificationRead(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	notificationID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || notificationID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID thông báo không hợp lệ"})
		return
	}

	var notification models.Notification
	query := config.DB.Where("id = ?", uint(notificationID))
	if c.GetString("role") != "admin" {
		query = query.Where("recipient_user_id = ?", userID)
	}
	if err := query.First(&notification).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy thông báo"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy thông báo", "error": err.Error()})
		return
	}

	now := time.Now()
	if err := config.DB.Model(&notification).Updates(map[string]interface{}{"is_read": true, "read_at": &now}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không cập nhật được trạng thái đọc", "error": err.Error()})
		return
	}
	notification.IsRead = true
	notification.ReadAt = &now
	c.JSON(http.StatusOK, gin.H{"message": "Đã đánh dấu thông báo là đã đọc", "data": notification})
}
