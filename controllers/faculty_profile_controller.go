package controllers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"student-management/config"
	"student-management/models"
	"unicode/utf8"
)

func UpdateFacultyProfile(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32768)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"message": "ID giảng viên không hợp lệ"})
		return
	}
	var req struct {
		Biography string `json:"biography"`
		AvatarURL string `json:"avatarUrl"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"message": "Dữ liệu giới thiệu không hợp lệ"})
		return
	}
	req.Biography = strings.TrimSpace(req.Biography)
	req.AvatarURL = strings.TrimSpace(req.AvatarURL)
	if utf8.RuneCountInString(req.Biography) > 5000 {
		c.JSON(400, gin.H{"message": "Giới thiệu tối đa 5000 ký tự"})
		return
	}
	if req.AvatarURL != "" {
		parsed, err := url.Parse(req.AvatarURL)
		if err != nil || len(req.AvatarURL) > 2048 || strings.ContainsAny(req.AvatarURL, "\\\r\n") || !((strings.HasPrefix(req.AvatarURL, "/") && !strings.HasPrefix(req.AvatarURL, "//")) || ((parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil)) {
			c.JSON(400, gin.H{"message": "Link ảnh cần là http/https hoặc /ten-anh.jpg"})
			return
		}
	}
	var teacher models.Teacher
	if config.DB.First(&teacher, id).Error != nil {
		c.JSON(404, gin.H{"message": "Không tìm thấy giảng viên"})
		return
	}
	if err := config.DB.Model(&teacher).Updates(map[string]interface{}{"biography": req.Biography, "avatar_url": req.AvatarURL}).Error; err != nil {
		c.JSON(500, gin.H{"message": "Không lưu được giới thiệu giảng viên"})
		return
	}
	c.JSON(200, gin.H{"message": "Đã cập nhật giới thiệu giảng viên"})
}
