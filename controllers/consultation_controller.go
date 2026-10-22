package controllers

import (
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"student-management/config"
	"student-management/models"
	"time"
	"unicode/utf8"
)

type consultationRequest struct {
	FullName         string `json:"fullName"`
	Phone            string `json:"phone"`
	Email            string `json:"email"`
	Program          string `json:"program"`
	Location         string `json:"location"`
	PreferredSession string `json:"preferredSession"`
	Message          string `json:"message"`
	Consent          bool   `json:"consent"`
	Website          string `json:"website"`
}

func validateConsultation(req *consultationRequest) error {
	req.FullName = strings.TrimSpace(req.FullName)
	req.Email = strings.TrimSpace(req.Email)
	req.Program = strings.TrimSpace(req.Program)
	req.Location = strings.TrimSpace(req.Location)
	req.Phone = strings.NewReplacer(" ", "", "-", "", ".", "", "(", "", ")", "").Replace(strings.TrimSpace(req.Phone))
	req.Message = strings.TrimSpace(req.Message)
	if utf8.RuneCountInString(req.FullName) < 2 || utf8.RuneCountInString(req.FullName) > 120 {
		return errors.New("Họ tên cần từ 2 đến 120 ký tự")
	}
	if !regexp.MustCompile(`^\+?[0-9]{9,15}$`).MatchString(req.Phone) {
		return errors.New("Số điện thoại không hợp lệ")
	}
	address, err := mail.ParseAddress(req.Email)
	if err != nil || address.Address != req.Email || len(req.Email) > 254 {
		return errors.New("Email không hợp lệ")
	}
	if req.Program == "" || utf8.RuneCountInString(req.Program) > 160 || req.Location == "" || utf8.RuneCountInString(req.Location) > 250 || utf8.RuneCountInString(req.Message) > 2000 {
		return errors.New("Vui lòng kiểm tra ngành học, địa điểm và nội dung tư vấn")
	}
	if req.PreferredSession != "Sáng" && req.PreferredSession != "Chiều" && req.PreferredSession != "Tối" && req.PreferredSession != "Linh hoạt" {
		return errors.New("Vui lòng chọn thời gian học mong muốn")
	}
	if !req.Consent {
		return errors.New("Vui lòng đồng ý để nhà trường liên hệ tư vấn")
	}
	return nil
}

func CreatePublicConsultation(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	var req consultationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "Dữ liệu đăng ký không hợp lệ"})
		return
	}
	if req.Website != "" {
		c.JSON(400, gin.H{"message": "Yêu cầu không hợp lệ"})
		return
	}
	if err := validateConsultation(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	var count int64
	if err := config.DB.Model(&models.Consultation{}).Where("(phone = ? OR email = ?) AND created_at > ?", req.Phone, req.Email, time.Now().Add(-time.Minute)).Count(&count).Error; err != nil {
		c.JSON(500, gin.H{"message": "Chưa thể nhận yêu cầu. Vui lòng thử lại"})
		return
	}
	if count > 0 {
		c.JSON(429, gin.H{"message": "Bạn vừa gửi yêu cầu. Vui lòng chờ một phút trước khi gửi lại"})
		return
	}
	row := models.Consultation{FullName: req.FullName, Phone: req.Phone, Email: req.Email, Program: req.Program, Location: req.Location, PreferredSession: req.PreferredSession, Message: req.Message, ConsentAt: time.Now(), Status: "new"}
	if err := config.DB.Create(&row).Error; err != nil {
		c.JSON(500, gin.H{"message": "Không lưu được yêu cầu tư vấn"})
		return
	}
	c.JSON(201, gin.H{"message": "Đã nhận đăng ký tư vấn. Nhà trường sẽ liên hệ theo thông tin bạn cung cấp", "reference": row.ID})
}

func ListConsultations(c *gin.Context) {
	var rows []models.Consultation
	if err := config.DB.Order("created_at desc").Limit(500).Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"message": "Không tải được đăng ký tư vấn"})
		return
	}
	c.JSON(200, gin.H{"data": rows})
}

func UpdateConsultation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"message": "ID không hợp lệ"})
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if c.ShouldBindJSON(&req) != nil || (req.Status != "new" && req.Status != "contacted" && req.Status != "completed") {
		c.JSON(400, gin.H{"message": "Trạng thái không hợp lệ"})
		return
	}
	result := config.DB.Model(&models.Consultation{}).Where("id = ?", id).Update("status", req.Status)
	if result.Error != nil {
		c.JSON(500, gin.H{"message": "Không cập nhật được yêu cầu"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(404, gin.H{"message": "Không tìm thấy yêu cầu hoặc trạng thái không đổi"})
		return
	}
	c.JSON(200, gin.H{"message": "Đã cập nhật yêu cầu tư vấn"})
}

func ListPublicFaculty(c *gin.Context) {
	var rows []struct {
		ID            uint   `json:"id"`
		Name          string `json:"name"`
		Qualification string `json:"qualification"`
		Major         string `json:"major"`
		Biography     string `json:"biography"`
		AvatarURL     string `json:"avatarUrl"`
	}
	// Publish only professional identity, never account credentials or private contact details.
	err := config.DB.Table("teachers t").Select("t.id, u.full_name AS name, t.qualification, m.name AS major, t.biography, t.avatar_url").Joins("JOIN users u ON u.id = t.user_id").Joins("LEFT JOIN majors m ON m.id = t.major_id").Order("u.full_name").Limit(60).Scan(&rows).Error
	if err != nil {
		c.JSON(500, gin.H{"message": "Không tải được thông tin giảng viên"})
		return
	}
	c.JSON(200, gin.H{"data": rows})
}
