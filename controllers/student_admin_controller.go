package controllers

import (
	"errors"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"strconv"
	"strings"
	"student-management/config"
	"student-management/models"
)

func UpdateStudent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"message": "ID sinh viên không hợp lệ"})
		return
	}
	var req CreateStudentRequest
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.FullName) == "" {
		c.JSON(400, gin.H{"message": "Vui lòng nhập tên đăng nhập, họ tên và lớp"})
		return
	}
	dob, err := parseOptionalDate(req.DateOfBirth)
	if err != nil {
		c.JSON(400, gin.H{"message": "Ngày sinh không hợp lệ"})
		return
	}
	enrolled, err := parseOptionalDate(req.EnrollmentDate)
	if err != nil {
		c.JSON(400, gin.H{"message": "Ngày nhập học không hợp lệ"})
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
		var student models.Student
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&student, id).Error; err != nil {
			return err
		}
		var class models.Class
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&class, req.ClassID).Error; err != nil {
			return err
		}
		if student.ClassID != req.ClassID {
			if class.Status != "open" {
				return errors.New("Lớp đã đóng, không thể chuyển sinh viên vào lớp")
			}
			var count int64
			if err := tx.Model(&models.Student{}).Where("class_id = ?", class.ID).Count(&count).Error; err != nil {
				return err
			}
			if class.MaxStudents > 0 && count >= int64(class.MaxStudents) {
				return errors.New("Lớp đã đủ sĩ số")
			}
		}
		users := map[string]interface{}{"username": strings.TrimSpace(req.Username), "full_name": strings.TrimSpace(req.FullName), "email": strings.TrimSpace(req.Email)}
		if len(hash) > 0 {
			users["password"] = string(hash)
			users["first_login"] = true
		}
		if err := tx.Model(&models.User{}).Where("id = ?", student.UserID).Updates(users).Error; err != nil {
			return errors.New("Không thể cập nhật tài khoản. Kiểm tra tên đăng nhập hoặc email đã tồn tại")
		}
		fields := map[string]interface{}{"class_id": req.ClassID, "gender": strings.TrimSpace(req.Gender), "phone": strings.TrimSpace(req.Phone), "address": strings.TrimSpace(req.Address), "date_of_birth": nil}
		if !dob.IsZero() {
			fields["date_of_birth"] = dob
		}
		if !enrolled.IsZero() {
			fields["enrollment_date"] = enrolled
		}
		if err := tx.Model(&models.Student{}).Where("id = ?", id).Updates(fields).Error; err != nil {
			return err
		}
		if student.ClassID != req.ClassID {
			return syncStudentClassEnrollments(tx, student.ID, req.ClassID)
		}
		return nil
	})
	if err != nil {
		studentAdminError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Cập nhật sinh viên thành công"})
}

func DeleteStudent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"message": "ID sinh viên không hợp lệ"})
		return
	}
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		var student models.Student
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&student, id).Error; err != nil {
			return err
		}
		for _, model := range []interface{}{&models.Enrollment{}, &models.CourseRegistration{}, &models.Submission{}, &models.Transcript{}, &models.AcademicWarning{}} {
			var count int64
			if err := tx.Model(model).Where("student_id = ?", student.ID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return errors.New("Sinh viên đã có dữ liệu học tập, không thể xóa để tránh mất lịch sử điểm và điểm danh")
			}
		}
		if err := tx.Delete(&student).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.User{}, student.UserID).Error; err != nil {
			return errors.New("Tài khoản còn dữ liệu liên quan nên chưa thể xóa")
		}
		return nil
	})
	if err != nil {
		studentAdminError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Đã xóa sinh viên và tài khoản đăng nhập"})
}

func studentAdminError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy sinh viên hoặc lớp"})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
}
