package controllers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"student-management/config"
	"student-management/models"
)

func ListProgramImages(c *gin.Context) {
	var rows []models.ProgramImage
	if err := config.DB.Order("slug").Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"message": "Không tải được ảnh ngành học"})
		return
	}
	c.JSON(200, gin.H{"data": rows})
}

func UpdateProgramImage(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	slug := c.Param("slug")
	var req struct {
		ImageURL string `json:"imageUrl"`
	}
	if !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(slug) || len(slug) > 100 || c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"message": "Dữ liệu ảnh không hợp lệ"})
		return
	}
	req.ImageURL = strings.TrimSpace(req.ImageURL)
	parsed, err := url.Parse(req.ImageURL)
	valid := req.ImageURL == "" || (err == nil && ((strings.HasPrefix(req.ImageURL, "/") && !strings.HasPrefix(req.ImageURL, "//") && !strings.ContainsAny(req.ImageURL, "\\\r\n")) || ((parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil)))
	if !valid || len(req.ImageURL) > 2048 {
		c.JSON(400, gin.H{"message": "Nhập đường dẫn /ten-anh.jpg hoặc link ảnh http/https"})
		return
	}
	row := models.ProgramImage{Slug: slug, ImageURL: req.ImageURL}
	if err := config.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "slug"}}, DoUpdates: clause.AssignmentColumns([]string{"image_url", "updated_at"})}).Create(&row).Error; err != nil {
		c.JSON(500, gin.H{"message": "Không lưu được ảnh ngành học"})
		return
	}
	c.JSON(200, gin.H{"message": "Đã cập nhật ảnh ngành học", "data": row})
}
