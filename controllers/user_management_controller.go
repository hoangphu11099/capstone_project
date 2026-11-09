package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"student-management/config"
	"student-management/models"
	"student-management/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CreateStudentRequest struct {
	Username       string `json:"username" binding:"required"`
	Password       string `json:"password"`
	Email          string `json:"email"`
	FullName       string `json:"fullName" binding:"required"`
	StudentCode    string `json:"studentCode"`
	MajorID        uint   `json:"majorId"`
	TrainingSystem string `json:"trainingSystem"`
	AutoIdentity   bool   `json:"autoIdentity"`
	ClassID        uint   `json:"classId"`
	DateOfBirth    string `json:"dateOfBirth"`
	Gender         string `json:"gender"`
	Phone          string `json:"phone"`
	Address        string `json:"address"`
	EnrollmentDate string `json:"enrollmentDate"`
	Status         string `json:"status"`
}

type CreateTeacherRequest struct {
	Username      string `json:"username" binding:"required"`
	Password      string `json:"password"`
	Email         string `json:"email"`
	FullName      string `json:"fullName" binding:"required"`
	TeacherCode   string `json:"teacherCode"`
	Phone         string `json:"phone"`
	Address       string `json:"address"`
	Qualification string `json:"qualification"`
	MajorID       uint   `json:"majorId"`
}

func CreateTeacher(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req CreateTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu tạo giảng viên không hợp lệ", "error": err.Error()})
		return
	}

	if !validateTeacherMajor(c, req.MajorID) {
		return
	}
	password := req.Password
	if password == "" {
		temporary := make([]byte, 12)
		if _, err := rand.Read(temporary); err != nil {
			c.JSON(500, gin.H{"message": "Cannot generate temporary password"})
			return
		}
		password = hex.EncodeToString(temporary)
	}
	if len(password) < 6 || len(password) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Mật khẩu phải có ít nhất 6 ký tự"})
		return
	}

	roleID, err := getRoleIDByName("teacher")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tìm thấy role teacher", "error": err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"message": "Không thể tạo mật khẩu"})
		return
	}
	var teacher models.Teacher
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		code, err := generateTeacherCode(tx)
		if err != nil {
			return err
		}
		user := models.User{Username: strings.TrimSpace(req.Username), Password: string(hash), Email: strings.TrimSpace(req.Email), FullName: strings.TrimSpace(req.FullName), RoleID: roleID, IsActive: true, FirstLogin: true}
		if user.Username == "" || user.FullName == "" {
			return errors.New("vui lòng nhập tên đăng nhập và họ tên")
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		teacher = models.Teacher{TeacherCode: code, UserID: user.ID, MajorID: req.MajorID, Phone: strings.TrimSpace(req.Phone), Address: strings.TrimSpace(req.Address), Qualification: strings.TrimSpace(req.Qualification), HireDate: time.Now()}
		if err := tx.Create(&teacher).Error; err != nil {
			return err
		}
		teacher.User = user
		return nil
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Không thể tạo giảng viên. Kiểm tra tên đăng nhập hoặc email đã được sử dụng.", "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Đã thêm giảng viên. Mã giảng viên: " + teacher.TeacherCode, "data": teacher, "defaultPassword": password})

}

func CreateStudent(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req CreateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu tạo sinh viên không hợp lệ", "error": err.Error()})
		return
	}
	gender, err := normalizeStudentGender(req.Gender)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	system, err := utils.NormalizeTrainingSystem(req.TrainingSystem)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	code := strings.ToUpper(strings.TrimSpace(req.StudentCode))
	if code != "" && !studentCodePattern.MatchString(code) {
		c.JSON(400, gin.H{"message": "Mã sinh viên chỉ gồm chữ in hoa và chữ số, từ 3 đến 32 ký tự"})
		return
	}

	password := req.Password
	if password == "" {
		temporary := make([]byte, 12)
		if _, err := rand.Read(temporary); err != nil {
			c.JSON(500, gin.H{"message": "Cannot generate temporary password"})
			return
		}
		password = hex.EncodeToString(temporary)
	}
	if len(password) < 6 || len(password) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Mật khẩu phải có ít nhất 6 ký tự"})
		return
	}

	roleID, err := getRoleIDByName("student")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tìm thấy role student", "error": err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không thể mã hóa mật khẩu", "error": err.Error()})
		return
	}

	dateOfBirth, err := parseOptionalDate(req.DateOfBirth)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ngày sinh không hợp lệ. Dùng định dạng YYYY-MM-DD"})
		return
	}

	enrollmentDate, err := parseOptionalDate(req.EnrollmentDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ngày nhập học không hợp lệ. Dùng định dạng YYYY-MM-DD"})
		return
	}
	if enrollmentDate.IsZero() {
		enrollmentDate = time.Now()
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "active"
	}

	var class models.Class
	if req.ClassID > 0 {
		if err := config.DB.First(&class, req.ClassID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy lớp của sinh viên"})
			return
		}
		if class.MajorID == 0 || (req.MajorID > 0 && req.MajorID != class.MajorID) {
			c.JSON(400, gin.H{"message": "Vui lòng chọn lớp thuộc đúng chuyên ngành đã đăng ký"})
			return
		}
		if req.MajorID == 0 {
			req.MajorID = class.MajorID
		}
	}
	var major models.Major
	if err := config.DB.Where("is_active = ?", true).First(&major, req.MajorID).Error; err != nil {
		c.JSON(400, gin.H{"message": "Không tìm thấy chuyên ngành đăng ký"})
		return
	}
	if req.AutoIdentity {
		username := strings.TrimSpace(req.Username)
		sequence, parseErr := strconv.ParseUint(strings.TrimPrefix(username, "student"), 10, 64)
		identity, formatErr := utils.FormatStudentIdentity(req.FullName, enrollmentDate.Year(), system, major.Code, sequence)
		if parseErr != nil || formatErr != nil || username != identity.Username || code != identity.StudentCode || strings.TrimSpace(req.Email) != identity.Email || identity.Email == "" {
			c.JSON(400, gin.H{"message": "Mã sinh viên, email hoặc tên đăng nhập chưa khớp thông tin. Vui lòng bấm Cập nhật mã để lấy lại."})
			return
		}
	}

	user := models.User{
		Username:   strings.TrimSpace(req.Username),
		Password:   string(hash),
		Email:      strings.TrimSpace(req.Email),
		FullName:   strings.TrimSpace(req.FullName),
		RoleID:     roleID,
		IsActive:   true,
		FirstLogin: true,
	}

	student := models.Student{
		ClassID:        req.ClassID,
		MajorID:        major.ID,
		TrainingSystem: system,
		DateOfBirth:    dateOfBirth,
		Gender:         gender,
		Phone:          strings.TrimSpace(req.Phone),
		Address:        strings.TrimSpace(req.Address),
		EnrollmentDate: enrollmentDate,
		Status:         status,
	}

	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if req.ClassID > 0 {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&class, req.ClassID).Error; err != nil {
				return err
			}
			if class.Status != "open" {
				return errors.New("Lớp đã đóng, không thể thêm sinh viên")
			}
			if class.MajorID == 0 || (req.MajorID > 0 && req.MajorID != class.MajorID) {
				return errStudentMajorMismatch
			}
			student.MajorID = class.MajorID
			if err := tx.Model(&models.Student{}).Where("class_id=?", class.ID).Count(&count).Error; err != nil {
				return err
			}
			if class.MaxStudents > 0 && count >= int64(class.MaxStudents) {
				return errClassCapacityExceeded
			}
		}
		if code == "" {
			identity, err := nextStudentIdentity(tx, req.FullName, enrollmentDate.Year(), system, major)
			if err != nil {
				return err
			}
			code = identity.StudentCode
		}
		if req.AutoIdentity {
			if err := tx.Model(&models.User{}).Where("username=? OR email=?", user.Username, user.Email).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return errStudentCodeTaken
			}
		}
		if err := tx.Model(&models.Student{}).Where("student_code=?", code).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errStudentCodeTaken
		}
		student.StudentCode = code
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		student.UserID = user.ID
		if err := tx.Create(&student).Error; err != nil {
			return err
		}
		if student.ClassID == 0 {
			return nil
		}
		return syncStudentClassEnrollments(tx, student.ID, student.ClassID)
	}); err != nil {
		if errors.Is(err, errStudentCodeTaken) {
			c.JSON(409, gin.H{"message": "Mã hoặc tên đăng nhập đã được sử dụng. Vui lòng bấm Cập nhật mã để lấy số tiếp theo."})
			return
		}
		if errors.Is(err, errStudentMajorMismatch) {
			c.JSON(400, gin.H{"message": "Lớp không thuộc chuyên ngành sinh viên đăng ký"})
			return
		}
		if errors.Is(err, errClassCapacityExceeded) {
			c.JSON(400, gin.H{"message": "Lớp đã đủ sĩ số tối đa. Vui lòng chọn lớp khác cùng chuyên ngành."})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"message": "Tạo sinh viên thất bại. Username/email/mã sinh viên có thể đã tồn tại", "error": err.Error()})
		return
	}

	config.DB.Preload("User").Preload("Class").First(&student, student.ID)
	c.JSON(http.StatusCreated, gin.H{"message": "Tạo sinh viên thành công. Mã sinh viên được tạo tự động: " + student.StudentCode, "data": student, "defaultPassword": password})
}

func ListStudents(c *gin.Context) {
	var students []models.Student
	query := config.DB.Preload("User").Preload("Class.Major").Preload("Major")
	if majorID := strings.TrimSpace(c.Query("majorId")); majorID != "" {
		id, err := strconv.ParseUint(majorID, 10, 32)
		if err != nil || id == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Chuyên ngành không hợp lệ"})
			return
		}
		query = query.Where("students.major_id=? OR ((students.major_id IS NULL OR students.major_id=0) AND students.class_id IN (SELECT id FROM classes WHERE major_id=?))", id, id)
	}
	if classID := strings.TrimSpace(c.Query("classId")); classID != "" {
		id, err := strconv.ParseUint(classID, 10, 64)
		if err != nil || id == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "ID lớp không hợp lệ"})
			return
		}
		query = query.Where("students.class_id = ?", id)
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		like := "%" + q + "%"
		query = query.Joins("JOIN users ON users.id = students.user_id").
			Where("students.student_code LIKE ? OR users.full_name LIKE ? OR users.username LIKE ?", like, like, like)
	}
	if err := query.Order("students.id desc").Find(&students).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách sinh viên", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách sinh viên thành công", "data": students})
}

func DeleteTeacher(c *gin.Context) {
	teacherID, err := strconv.Atoi(c.Param("id"))
	if err != nil || teacherID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID giảng viên không hợp lệ"})
		return
	}

	var teacher models.Teacher
	if err := config.DB.First(&teacher, teacherID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy giảng viên"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy giảng viên", "error": err.Error()})
		return
	}

	var classCount int64
	if err := config.DB.Model(&models.Class{}).Where("teacher_id = ?", teacher.ID).Count(&classCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi kiểm tra lớp giảng dạy", "error": err.Error()})
		return
	}
	if classCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Giảng viên đang được phân công lớp nên không thể xóa. Hãy đổi giảng viên phụ trách trước"})
		return
	}

	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("teacher_id = ?", teacher.ID).Delete(&models.ClassOffer{}).Error; err != nil {
			return err
		}
		if err := tx.Where("teacher_id = ?", teacher.ID).Delete(&models.ClassTeacher{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&teacher).Error; err != nil {
			return err
		}
		return tx.Delete(&models.User{}, teacher.UserID).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Xóa giảng viên thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Xóa giảng viên thành công"})
}

func ListTeachers(c *gin.Context) {
	var teachers []models.Teacher
	query := config.DB.Preload("User").Preload("Major")
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		like := "%" + q + "%"
		query = query.Joins("JOIN users ON users.id = teachers.user_id").
			Where("teachers.teacher_code LIKE ? OR users.full_name LIKE ? OR users.username LIKE ?", like, like, like)
	}
	if err := query.Order("teachers.id desc").Find(&teachers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách giảng viên", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách giảng viên thành công", "data": teachers})
}

func parseOptionalDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q", value)
	}
	return date, nil
}

var studentCodePattern = regexp.MustCompile(`^[A-Z0-9]{3,32}$`)
var errStudentCodeTaken = errors.New("student code already used")

func normalizeStudentGender(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "", nil
	case "nam", "male", "m":
		return "Nam", nil
	case "nu", "female", "f":
		return "N\u1eef", nil
	}
	if strings.EqualFold(strings.TrimSpace(value), "N\u1eef") {
		return "N\u1eef", nil
	}
	return "", errors.New("Giới tính chỉ được chọn Nam hoặc Nữ")
}

// Existing codes stay unchanged. New codes use cryptographic randomness and the database unique constraint.
func generateTeacherCode(tx *gorm.DB) (string, error) {
	for attempt := 0; attempt < 10; attempt++ {
		raw := make([]byte, 5)
		if _, err := rand.Read(raw); err != nil {
			return "", err
		}
		code := "GV" + strings.ToUpper(hex.EncodeToString(raw))
		var count int64
		if err := tx.Model(&models.Teacher{}).Where("teacher_code = ?", code).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", errors.New("không tạo được mã giảng viên, vui lòng thử lại")
}
