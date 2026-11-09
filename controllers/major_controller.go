package controllers

import (
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
	"student-management/config"
	"student-management/models"
)

func ListMajors(c *gin.Context) {
	var majors []models.Major
	if err := config.DB.Order("code").Find(&majors).Error; err != nil {
		c.JSON(500, gin.H{"message": "Không tải được danh mục chuyên ngành"})
		return
	}
	c.JSON(200, gin.H{"data": majors})
}

// Codes are stable identifiers used in class codes and student identities.
func UpdateMajor(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"message": "ID chuyên ngành không hợp lệ"})
		return
	}
	var req struct {
		Name     string `json:"name"`
		Code     string `json:"code"`
		IsActive *bool  `json:"isActive"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(400, gin.H{"message": "Vui lòng nhập tên chuyên ngành"})
		return
	}
	var major models.Major
	if config.DB.First(&major, id).Error != nil {
		c.JSON(404, gin.H{"message": "Không tìm thấy chuyên ngành"})
		return
	}
	if req.Code != "" && strings.ToUpper(strings.TrimSpace(req.Code)) != major.Code {
		c.JSON(400, gin.H{"message": "Mã chuyên ngành được giữ nguyên để bảo toàn mã lớp và sinh viên"})
		return
	}
	fields := map[string]interface{}{"name": strings.TrimSpace(req.Name)}
	if req.IsActive != nil {
		fields["is_active"] = *req.IsActive
	}
	if err := config.DB.Model(&major).Updates(fields).Error; err != nil {
		c.JSON(500, gin.H{"message": "Không cập nhật được chuyên ngành"})
		return
	}
	c.JSON(200, gin.H{"message": "Đã cập nhật chuyên ngành", "data": major})
}
