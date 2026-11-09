package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"strconv"
	"strings"
	"student-management/config"
	"student-management/models"
	"student-management/utils"
	"time"
)

func nextStudentSequence(db *gorm.DB) (uint64, error) {
	var maximum uint64
	err := db.Raw("SELECT COALESCE(MAX(CAST(SUBSTRING(username,8) AS UNSIGNED)),0) FROM users WHERE username REGEXP '^student[0-9]+$'").Scan(&maximum).Error
	if maximum == ^uint64(0) {
		return 0, fmt.Errorf("Không còn số thứ tự khả dụng")
	}
	return maximum + 1, err
}

func nextStudentIdentity(db *gorm.DB, name string, year int, system string, major models.Major) (utils.StudentIdentity, error) {
	sequence, err := nextStudentSequence(db)
	if err != nil {
		return utils.StudentIdentity{}, err
	}
	for attempt := 0; attempt < 100; attempt++ {
		identity, err := utils.FormatStudentIdentity(name, year, system, major.Code, sequence)
		if err != nil {
			return identity, err
		}
		var users, codes int64
		if err := db.Model(&models.User{}).Where("username=? OR (email<>'' AND email=?)", identity.Username, identity.Email).Count(&users).Error; err != nil {
			return identity, err
		}
		if err := db.Model(&models.Student{}).Where("student_code=?", identity.StudentCode).Count(&codes).Error; err != nil {
			return identity, err
		}
		if users == 0 && codes == 0 {
			return identity, nil
		}
		sequence++
	}
	return utils.StudentIdentity{}, fmt.Errorf("Không tạo được mã định danh tiếp theo")
}

// GenerateStudentCode previews the exact account identifiers used by the admission form.
func GenerateStudentCode(c *gin.Context) {
	year := time.Now().In(attendanceLocation()).Year()
	if raw := c.Query("year"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1900 || n > 9999 {
			c.JSON(400, gin.H{"message": "Năm nhập học không hợp lệ"})
			return
		}
		year = n
	}
	system, err := utils.NormalizeTrainingSystem(c.Query("trainingSystem"))
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	majorID, err := strconv.ParseUint(c.Query("majorId"), 10, 32)
	if err != nil || majorID == 0 {
		c.JSON(400, gin.H{"message": "Chọn chuyên ngành trước khi tạo mã sinh viên"})
		return
	}
	var major models.Major
	if err := config.DB.Where("is_active = ?", true).First(&major, majorID).Error; err != nil {
		c.JSON(400, gin.H{"message": "Không tìm thấy chuyên ngành"})
		return
	}
	identity, err := nextStudentIdentity(config.DB, strings.TrimSpace(c.Query("fullName")), year, system, major)
	if err != nil {
		c.JSON(500, gin.H{"message": "Không tạo được thông tin sinh viên. Vui lòng thử lại."})
		return
	}
	c.JSON(200, gin.H{"data": identity})
}
