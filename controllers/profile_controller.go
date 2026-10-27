package controllers

import (
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"strings"
	"student-management/config"
	"student-management/models"
)

func GetOwnProfile(c *gin.Context) {
	id, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	var user models.User
	if err := config.DB.Preload("Role").Preload("Teacher.Major").Preload("Student.Class").First(&user, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "Không tìm thấy hồ sơ"})
		return
	}
	c.JSON(200, gin.H{"data": user})
}

func UpdateOwnProfile(c *gin.Context) {
	id, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	var req struct {
		FullName string `json:"fullName" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Phone    string `json:"phone"`
		Address  string `json:"address"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.FullName) == "" {
		c.JSON(400, gin.H{"message": "Vui lòng nhập họ tên và email hợp lệ"})
		return
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.User{}).Where("id = ?", id).Updates(map[string]interface{}{"full_name": strings.TrimSpace(req.FullName), "email": strings.TrimSpace(req.Email)}).Error; err != nil {
			return err
		}
		contact := map[string]interface{}{"phone": strings.TrimSpace(req.Phone), "address": strings.TrimSpace(req.Address)}
		if err := tx.Model(&models.Teacher{}).Where("user_id = ?", id).Updates(contact).Error; err != nil {
			return err
		}
		return tx.Model(&models.Student{}).Where("user_id = ?", id).Updates(contact).Error
	})
	if err != nil {
		c.JSON(400, gin.H{"message": "Không lưu được hồ sơ. Email có thể đã được sử dụng."})
		return
	}
	c.JSON(200, gin.H{"message": "Cập nhật hồ sơ thành công"})
}

func ChangeOwnPassword(c *gin.Context) {
	id, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	var req struct {
		Current string `json:"currentPassword" binding:"required"`
		New     string `json:"newPassword" binding:"required"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.New) < 6 {
		c.JSON(400, gin.H{"message": "Mật khẩu mới phải có ít nhất 6 ký tự"})
		return
	}
	var user models.User
	if config.DB.First(&user, id).Error != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Current)) != nil {
		c.JSON(400, gin.H{"message": "Mật khẩu hiện tại không đúng"})
		return
	}
	if req.Current == req.New {
		c.JSON(400, gin.H{"message": "Mật khẩu mới phải khác mật khẩu hiện tại"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.New), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"message": "Không đổi được mật khẩu"})
		return
	}
	result := config.DB.Model(&models.User{}).Where("id = ? AND password = ?", id, user.Password).Updates(map[string]interface{}{"password": string(hash), "first_login": false})
	if result.Error != nil || result.RowsAffected != 1 {
		c.JSON(400, gin.H{"message": "Không đổi được mật khẩu. Vui lòng thử lại."})
		return
	}
	c.JSON(200, gin.H{"message": "Đổi mật khẩu thành công"})
}
