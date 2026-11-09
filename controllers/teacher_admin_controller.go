package controllers

import (
	"crypto/rand"
	"errors"
	"github.com/gin-gonic/gin"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"strings"
	"student-management/config"
	"student-management/models"
	"student-management/utils"
)

func UpdateTeacher(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"message": "ID giảng viên không hợp lệ"})
		return
	}
	var req CreateTeacherRequest
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.FullName) == "" {
		c.JSON(400, gin.H{"message": "Vui lòng nhập tên đăng nhập và họ tên"})
		return
	}
	if !validateTeacherMajor(c, req.MajorID) {
		return
	}
	var hash []byte
	if req.Password != "" {
		if len(req.Password) < 6 {
			c.JSON(400, gin.H{"message": "Mật khẩu phải có ít nhất 6 ký tự"})
			return
		}
		hash, err = bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(400, gin.H{"message": "Mật khẩu không hợp lệ"})
			return
		}
	}
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		var teacher models.Teacher
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&teacher, id).Error; err != nil {
			return err
		}
		account := map[string]interface{}{"username": strings.TrimSpace(req.Username), "full_name": strings.TrimSpace(req.FullName), "email": strings.TrimSpace(req.Email)}
		if len(hash) > 0 {
			account["password"] = string(hash)
			account["first_login"] = true
		}
		if err := tx.Model(&models.User{}).Where("id = ?", teacher.UserID).Updates(account).Error; err != nil {
			return errors.New("Không thể cập nhật tài khoản. Kiểm tra tên đăng nhập hoặc email đã tồn tại")
		}
		return tx.Model(&models.Teacher{}).Where("id = ?", teacher.ID).Updates(map[string]interface{}{
			"phone": strings.TrimSpace(req.Phone), "address": strings.TrimSpace(req.Address), "qualification": strings.TrimSpace(req.Qualification), "major_id": req.MajorID,
		}).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(404, gin.H{"message": "Không tìm thấy giảng viên"})
		return
	}
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Cập nhật giảng viên thành công"})
}

func validateTeacherMajor(c *gin.Context, id uint) bool {
	var major models.Major
	if id == 0 || config.DB.Where("is_active = ?", true).First(&major, id).Error != nil {
		c.JSON(400, gin.H{"message": "Vui lòng chọn ngành dạy hợp lệ cho giảng viên"})
		return false
	}
	return true
}

func CreateMajor(c *gin.Context) {
	var req struct {
		Code     string `json:"code"`
		Name     string `json:"name"`
		IsActive *bool  `json:"isActive"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(400, gin.H{"message": "Vui lòng nhập tên chuyên ngành"})
		return
	}
	major := models.Major{Code: strings.ToUpper(strings.TrimSpace(req.Code)), Name: strings.TrimSpace(req.Name), IsActive: true}
	automatic := major.Code == ""
	base := utils.MajorCodeBase(major.Name)
	if automatic {
		major.Code = base
	}
	for _, char := range major.Code {
		if !(char >= 'A' && char <= 'Z' || char >= '0' && char <= '9') {
			c.JSON(400, gin.H{"message": "Mã chuyên ngành chỉ gồm chữ cái không dấu và chữ số"})
			return
		}
	}
	if len(major.Code) > 20 {
		c.JSON(400, gin.H{"message": "Mã chuyên ngành tối đa 20 ký tự"})
		return
	}
	for attempt := 0; attempt < 12; attempt++ {
		major.ID = 0
		err := config.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&major).Error; err != nil {
				return err
			}
			if req.IsActive != nil && !*req.IsActive {
				return tx.Model(&major).Update("is_active", false).Error
			}
			return nil
		})
		if err == nil {
			c.JSON(201, gin.H{"message": "Tạo chuyên ngành thành công", "data": major})
			return
		}
		var duplicate *mysqlDriver.MySQLError
		if automatic && errors.As(err, &duplicate) && duplicate.Number == 1062 {
			var suffix [4]byte
			if _, err := rand.Read(suffix[:]); err != nil {
				c.JSON(500, gin.H{"message": "Không tạo được mã chuyên ngành"})
				return
			}
			for i := range suffix {
				suffix[i] = 'A' + suffix[i]%26
			}
			major.Code = base + string(suffix[:])
			continue
		}
		c.JSON(400, gin.H{"message": "Không tạo được chuyên ngành. Kiểm tra mã chuyên ngành đã tồn tại."})
		return
	}
	c.JSON(409, gin.H{"message": "Chưa tạo được mã chuyên ngành duy nhất. Vui lòng thử lại."})
}
