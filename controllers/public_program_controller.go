package controllers

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"student-management/config"
	"student-management/models"
	"unicode/utf8"
)

type publicProgramContent struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Group       string   `json:"group"`
	Symbol      string   `json:"symbol"`
	Image       string   `json:"image"`
	Tagline     string   `json:"tagline"`
	Description string   `json:"description"`
	Tools       []string `json:"tools"`
	Skills      []string `json:"skills"`
	Careers     []string `json:"careers"`
	Project     string   `json:"project"`
	Stages      []struct {
		Title  string   `json:"title"`
		Topics []string `json:"topics"`
	} `json:"stages"`
	Published bool `json:"published"`
}

func validatePublicProgram(p *publicProgramContent) error {
	p.Name = strings.TrimSpace(p.Name)
	p.Image = strings.TrimSpace(p.Image)
	if !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(p.Slug) || len(p.Slug) > 100 {
		return errors.New("Đường dẫn ngành chỉ gồm chữ thường không dấu, số và dấu gạch ngang")
	}
	if p.Name == "" || utf8.RuneCountInString(p.Name) > 160 {
		return errors.New("Tên ngành cần từ 1 đến 160 ký tự")
	}
	if p.Group != "Công nghệ" && p.Group != "Sáng tạo" && p.Group != "Kinh doanh" {
		return errors.New("Nhóm ngành không hợp lệ")
	}
	if !map[string]bool{"code": true, "ai": true, "shield": true, "cloud": true, "chip": true, "design": true, "ux": true, "marketing": true, "business": true, "logistics": true}[p.Symbol] {
		return errors.New("Biểu tượng ngành không hợp lệ")
	}
	parsed, err := url.Parse(p.Image)
	if p.Image == "" || len(p.Image) > 2048 || err != nil || strings.ContainsAny(p.Image, "\\\r\n") || !((strings.HasPrefix(p.Image, "/") && !strings.HasPrefix(p.Image, "//")) || ((parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil)) {
		return errors.New("Nhập link ảnh http/https hoặc đường dẫn /ten-anh.jpg")
	}
	for _, value := range []string{p.Tagline, p.Description, p.Project} {
		if strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > 5000 {
			return errors.New("Điền đầy đủ mô tả, giới thiệu và dự án (tối đa 5000 ký tự mỗi mục)")
		}
	}
	lists := [][]string{p.Tools, p.Skills, p.Careers}
	if len(p.Stages) < 1 || len(p.Stages) > 12 {
		return errors.New("Lộ trình cần từ 1 đến 12 chặng học")
	}
	for _, stage := range p.Stages {
		if strings.TrimSpace(stage.Title) == "" || utf8.RuneCountInString(stage.Title) > 200 {
			return errors.New("Tên chặng học không hợp lệ")
		}
		lists = append(lists, stage.Topics)
	}
	for _, list := range lists {
		if len(list) < 1 || len(list) > 40 {
			return errors.New("Mỗi danh sách cần từ 1 đến 40 nội dung")
		}
		for _, item := range list {
			if strings.TrimSpace(item) == "" || utf8.RuneCountInString(item) > 1000 {
				return errors.New("Nội dung danh sách không được trống và tối đa 1000 ký tự")
			}
		}
	}
	return nil
}

func ListPublicPrograms(c *gin.Context) {
	var rows []models.PublicProgram
	if err := config.DB.Order("slug").Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"message": "Không tải được nội dung ngành học"})
		return
	}
	data := make([]json.RawMessage, 0, len(rows))
	for _, row := range rows {
		if json.Valid([]byte(row.Content)) {
			data = append(data, json.RawMessage(row.Content))
		}
	}
	c.JSON(200, gin.H{"data": data})
}

func SavePublicProgram(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 65536)
	var req publicProgramContent
	if c.ShouldBindJSON(&req) != nil || req.Slug != c.Param("slug") {
		c.JSON(400, gin.H{"message": "Dữ liệu ngành học không hợp lệ"})
		return
	}
	if err := validatePublicProgram(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	content, err := json.Marshal(req)
	if err != nil {
		c.JSON(400, gin.H{"message": "Không đọc được nội dung ngành"})
		return
	}
	row := models.PublicProgram{Slug: req.Slug, Content: string(content)}
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "slug"}}, DoUpdates: clause.AssignmentColumns([]string{"content", "updated_at"})}).Create(&row).Error; err != nil {
			return err
		}
		// Keep image overrides created by the previous editor consistent.
		image := models.ProgramImage{Slug: req.Slug, ImageURL: req.Image}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "slug"}}, DoUpdates: clause.AssignmentColumns([]string{"image_url", "updated_at"})}).Create(&image).Error
	})
	if err != nil {
		c.JSON(500, gin.H{"message": "Không lưu được nội dung ngành học"})
		return
	}
	c.JSON(200, gin.H{"message": "Đã lưu nội dung ngành học", "data": req})
}
