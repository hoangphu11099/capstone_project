package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const attendanceQRDuration = time.Minute

type CreateAttendanceSessionRequest struct {
	ClassID   uint   `json:"classId" binding:"required"`
	CourseID  uint   `json:"courseId" binding:"required"`
	ClassDate string `json:"classDate"`
	Note      string `json:"note"`
}

type AttendanceQRRequest struct {
	Code string `json:"code" binding:"required"`
}

type UpdateAttendanceStatusRequest struct {
	Status string `json:"status" binding:"required"`
	Note   string `json:"note"`
}

func ListStudentAttendanceClasses(c *gin.Context) {
	student, ok := getCurrentStudent(c)
	if !ok {
		return
	}

	var enrollments []models.Enrollment
	if err := config.DB.Preload("Class.Room").Preload("Course").Preload("Class.Semester").
		Where("student_id = ? AND status <> ?", student.ID, "cancelled").
		Order("class_id, course_id").
		Find(&enrollments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách môn đang học", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách môn điểm danh thành công", "data": enrollments})
}

func AttendanceByQR(c *gin.Context) {
	student, ok := getCurrentStudent(c)
	if !ok {
		return
	}

	var req AttendanceQRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Vui lòng nhập mã QR", "error": err.Error()})
		return
	}

	code := strings.TrimSpace(req.Code)
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Mã QR không được để trống"})
		return
	}

	var session models.AttendanceSession
	if err := config.DB.Preload("Class").Preload("Course").Where("code = ?", code).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Mã QR không tồn tại"})
		return
	}

	now := time.Now()
	if !session.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Phiên điểm danh đã đóng"})
		return
	}

	if now.After(session.ExpiresAt) {
		// Mã sinh viên vừa quét đã quá 1 phút. Hệ thống tự sinh mã mới
		// cho phiên điểm danh này, nhưng không chấp nhận mã cũ nữa.
		if err := rotateAttendanceSessionQRCode(&session); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Mã QR đã hết hạn nhưng hệ thống không thể sinh mã mới", "error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"message": "Mã QR đã hết hạn. Vui lòng quét mã mới", "newExpiresAt": session.ExpiresAt})
		return
	}

	var enrollment models.Enrollment
	if err := config.DB.Where("student_id = ? AND class_id = ? AND course_id = ? AND status <> ?", student.ID, session.ClassID, session.CourseID, "cancelled").First(&enrollment).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"message": "Sinh viên không thuộc lớp hoặc môn học của mã QR này"})
		return
	}

	checkedAt := now
	var attendance models.Attendance
	err := config.DB.Where("attendance_session_id = ? AND enrollment_id = ?", session.ID, enrollment.ID).First(&attendance).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi kiểm tra dữ liệu điểm danh", "error": err.Error()})
		return
	}

	attendance.EnrollmentID = enrollment.ID
	attendance.AttendanceSessionID = &session.ID
	attendance.ClassDate = session.ClassDate
	attendance.Status = "present"
	attendance.Note = "Điểm danh bằng QR"
	attendance.CheckedInAt = &checkedAt

	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = config.DB.Create(&attendance).Error
	} else {
		err = config.DB.Save(&attendance).Error
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Điểm danh thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Điểm danh thành công",
		"data":      attendance,
		"expiresAt": session.ExpiresAt,
	})
}

func ListMyAttendances(c *gin.Context) {
	student, ok := getCurrentStudent(c)
	if !ok {
		return
	}

	var attendances []models.Attendance
	if err := config.DB.Preload("Enrollment.Course").Preload("Enrollment.Class").Preload("AttendanceSession").
		Joins("JOIN enrollments ON enrollments.id = attendances.enrollment_id").
		Where("enrollments.student_id = ?", student.ID).
		Order("attendances.class_date DESC, attendances.id DESC").
		Find(&attendances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy lịch sử điểm danh", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy lịch sử điểm danh thành công", "data": attendances})
}

func ListTeacherAttendanceClasses(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	var classes []models.Class
	if err := config.DB.Preload("Room").Preload("Semester").Preload("Major").
		Where("teacher_id = ?", teacher.ID).
		Order("class_code").Find(&classes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy lớp điểm danh", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách lớp điểm danh thành công", "data": classes})
}

func CreateAttendanceSession(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	var req CreateAttendanceSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu tạo mã QR không hợp lệ", "error": err.Error()})
		return
	}

	var class models.Class
	if err := config.DB.First(&class, req.ClassID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy lớp học"})
		return
	}

	if class.TeacherID != teacher.ID {
		c.JSON(http.StatusForbidden, gin.H{"message": "Giảng viên không được phân công lớp này"})
		return
	}

	var course models.Course
	if err := config.DB.First(&course, req.CourseID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy môn học"})
		return
	}

	var enrollmentCount int64
	config.DB.Model(&models.Enrollment{}).
		Where("class_id = ? AND course_id = ? AND status <> ?", req.ClassID, req.CourseID, "cancelled").
		Count(&enrollmentCount)
	if enrollmentCount == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Lớp này chưa có sinh viên đăng ký môn học đã chọn"})
		return
	}

	classDate := time.Now()
	if strings.TrimSpace(req.ClassDate) != "" {
		parsed, err := parseDateTime(req.ClassDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Ngày điểm danh không hợp lệ. Dùng yyyy-mm-dd hoặc RFC3339"})
			return
		}
		classDate = parsed
	}

	code, err := generateAttendanceCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tạo được mã QR", "error": err.Error()})
		return
	}

	now := time.Now()
	session := models.AttendanceSession{
		ClassID:   req.ClassID,
		CourseID:  req.CourseID,
		TeacherID: teacher.ID,
		Code:      code,
		ClassDate: classDate,
		ExpiresAt: now.Add(attendanceQRDuration),
		IsActive:  true,
		Note:      strings.TrimSpace(req.Note),
	}

	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.AttendanceSession{}).
			Where("class_id = ? AND course_id = ? AND is_active = ?", req.ClassID, req.CourseID, true).
			Updates(map[string]interface{}{"is_active": false}).Error; err != nil {
			return err
		}
		return tx.Create(&session).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Tạo mã QR điểm danh thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Tạo mã QR điểm danh thành công. Mã chỉ có hiệu lực 1 phút",
		"data":       session,
		"qrCode":     session.Code,
		"ttlSeconds": int(attendanceQRDuration.Seconds()),
		"expiresAt":  session.ExpiresAt,
	})
}

func GetAttendanceSession(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	session, ok := findTeacherAttendanceSession(c, teacher.ID)
	if !ok {
		return
	}

	refreshed, err := refreshAttendanceSessionQRCodeIfExpired(&session)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không thể làm mới mã QR", "error": err.Error()})
		return
	}

	remaining := int(time.Until(session.ExpiresAt).Seconds())
	if remaining < 0 {
		remaining = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Lấy phiên điểm danh thành công",
		"data":       session,
		"qrCode":     session.Code,
		"ttlSeconds": remaining,
		"expiresAt":  session.ExpiresAt,
		"refreshed":  refreshed,
	})
}

func GetAttendanceSessionQRCode(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	session, ok := findTeacherAttendanceSession(c, teacher.ID)
	if !ok {
		return
	}

	refreshed, err := refreshAttendanceSessionQRCodeIfExpired(&session)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không thể làm mới mã QR", "error": err.Error()})
		return
	}

	remaining := int(time.Until(session.ExpiresAt).Seconds())
	if remaining < 0 {
		remaining = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Lấy mã QR hiện tại thành công",
		"sessionId":  session.ID,
		"qrCode":     session.Code,
		"ttlSeconds": remaining,
		"expiresAt":  session.ExpiresAt,
		"refreshed":  refreshed,
	})
}

func ListAttendanceSessionRecords(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	session, ok := findTeacherAttendanceSession(c, teacher.ID)
	if !ok {
		return
	}

	var enrollments []models.Enrollment
	if err := config.DB.Preload("Student.User").Preload("Course").
		Where("class_id = ? AND course_id = ? AND status <> ?", session.ClassID, session.CourseID, "cancelled").
		Order("student_id").Find(&enrollments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách sinh viên", "error": err.Error()})
		return
	}

	attendanceByEnrollment := map[uint]models.Attendance{}
	var attendances []models.Attendance
	if err := config.DB.Where("attendance_session_id = ?", session.ID).Find(&attendances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy dữ liệu điểm danh", "error": err.Error()})
		return
	}
	for _, attendance := range attendances {
		attendanceByEnrollment[attendance.EnrollmentID] = attendance
	}

	records := make([]gin.H, 0, len(enrollments))
	for _, enrollment := range enrollments {
		attendance, found := attendanceByEnrollment[enrollment.ID]
		status := "absent"
		if found {
			status = attendance.Status
		}
		records = append(records, gin.H{
			"enrollment": enrollment,
			"attendance": attendance,
			"status":     status,
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách điểm danh thành công", "session": session, "data": records})
}

func UpdateAttendanceStatus(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	attendanceID, err := strconv.Atoi(c.Param("id"))
	if err != nil || attendanceID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID điểm danh không hợp lệ"})
		return
	}

	var req UpdateAttendanceStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu cập nhật điểm danh không hợp lệ", "error": err.Error()})
		return
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if !isValidAttendanceStatus(status) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Trạng thái điểm danh chỉ gồm present, absent, late, excused"})
		return
	}

	var attendance models.Attendance
	if err := config.DB.Preload("Enrollment.Class").First(&attendance, attendanceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy bản ghi điểm danh"})
		return
	}

	if attendance.Enrollment.Class.TeacherID != teacher.ID {
		c.JSON(http.StatusForbidden, gin.H{"message": "Giảng viên không được sửa điểm danh của lớp này"})
		return
	}

	attendance.Status = status
	attendance.Note = strings.TrimSpace(req.Note)
	if err := config.DB.Save(&attendance).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Cập nhật điểm danh thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cập nhật điểm danh thành công", "data": attendance})
}

func findTeacherAttendanceSession(c *gin.Context, teacherID uint) (models.AttendanceSession, bool) {
	sessionID, err := strconv.Atoi(c.Param("id"))
	if err != nil || sessionID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID phiên điểm danh không hợp lệ"})
		return models.AttendanceSession{}, false
	}

	var session models.AttendanceSession
	if err := config.DB.Preload("Class").Preload("Course").First(&session, sessionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy phiên điểm danh"})
		return models.AttendanceSession{}, false
	}

	if session.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"message": "Giảng viên không được quản lý phiên điểm danh này"})
		return models.AttendanceSession{}, false
	}

	return session, true
}

func refreshAttendanceSessionQRCodeIfExpired(session *models.AttendanceSession) (bool, error) {
	if !session.IsActive {
		return false, nil
	}

	if time.Now().Before(session.ExpiresAt) {
		return false, nil
	}

	if err := rotateAttendanceSessionQRCode(session); err != nil {
		return false, err
	}

	return true, nil
}

func rotateAttendanceSessionQRCode(session *models.AttendanceSession) error {
	code, err := generateAttendanceCode()
	if err != nil {
		return err
	}

	session.Code = code
	session.ExpiresAt = time.Now().Add(attendanceQRDuration)
	session.IsActive = true

	return config.DB.Save(session).Error
}

func generateAttendanceCode() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ATT-" + hex.EncodeToString(bytes), nil
}

func isValidAttendanceStatus(status string) bool {
	switch status {
	case "present", "absent", "late", "excused":
		return true
	default:
		return false
	}
}
