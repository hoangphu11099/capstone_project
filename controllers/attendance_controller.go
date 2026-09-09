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
	CourseOfferingID uint   `json:"courseOfferingId"`
	ClassID          uint   `json:"classId"`
	CourseID         uint   `json:"courseId"`
	ClassDate        string `json:"classDate"`
	Note             string `json:"note"`
}

type AttendanceQRRequest struct {
	Code   string `json:"code"`
	QRCode string `json:"qrCode"`
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
		code = strings.TrimSpace(req.QRCode)
	}
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Mã QR không được để trống"})
		return
	}

	var session models.AttendanceSession
	if err := config.DB.Preload("Class").Preload("Course").Preload("Schedule").Preload("CourseOffering.Semester").Where("code = ?", code).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Mã QR không tồn tại"})
		return
	}

	now := attendanceNow()
	if closeAttendanceSessionWhenLessonEnded(&session, now) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Phiên điểm danh đã đóng vì tiết học đã kết thúc"})
		return
	}
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
	enrollmentQuery := config.DB.Where("student_id = ? AND status <> ?", student.ID, "cancelled")
	if session.CourseOfferingID != nil {
		enrollmentQuery = enrollmentQuery.Where("course_offering_id = ?", *session.CourseOfferingID)
	} else {
		enrollmentQuery = enrollmentQuery.Where("class_id = ? AND course_id = ?", session.ClassID, session.CourseID)
	}
	if err := enrollmentQuery.First(&enrollment).Error; err != nil {
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
		Joins("JOIN course_offerings ON course_offerings.class_id = classes.id").
		Where("course_offerings.teacher_id = ? AND course_offerings.status = ?", teacher.ID, "open").
		Distinct("classes.*").
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

	var offering models.CourseOffering
	query := config.DB.Preload("Class").Preload("Course").Preload("Semester").Preload("Schedules").
		Where("teacher_id = ? AND status = ?", teacher.ID, "open")
	if req.CourseOfferingID > 0 {
		query = query.Where("id = ?", req.CourseOfferingID)
	} else {
		query = query.Where("class_id = ? AND course_id = ?", req.ClassID, req.CourseID)
	}
	if err := query.First(&offering).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"message": "Giảng viên không được phân công học phần này"})
		return
	}
	if len(offering.Schedules) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Học phần chưa có lịch dạy"})
		return
	}

	now := attendanceNow()
	var schedule *models.Schedule
	for i := range offering.Schedules {
		if scheduleMatchesDate(offering.Schedules[i], now) {
			schedule = &offering.Schedules[i]
			break
		}
	}
	if schedule == nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Hôm nay giảng viên không có lịch dạy học phần này"})
		return
	}
	lessonStart, lessonEnd, err := attendanceLessonWindow(*schedule, now)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Khung giờ của tiết học không hợp lệ"})
		return
	}
	if now.Before(lessonStart.Add(-15 * time.Minute)) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Chỉ được mở điểm danh từ 15 phút trước giờ học"})
		return
	}
	if now.After(lessonEnd) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Không thể mở điểm danh vì tiết học đã kết thúc"})
		return
	}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if dayStart.Before(offering.Semester.StartDate) || dayStart.After(offering.Semester.EndDate) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Hôm nay nằm ngoài thời gian học kỳ"})
		return
	}

	var enrollments []models.Enrollment
	if err := config.DB.Where("course_offering_id = ? AND status <> ?", offering.ID, "cancelled").
		Find(&enrollments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách sinh viên của lớp", "error": err.Error()})
		return
	}
	if len(enrollments) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Lớp này chưa có sinh viên đăng ký môn học đã chọn"})
		return
	}

	code, err := generateAttendanceCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tạo được mã QR", "error": err.Error()})
		return
	}

	offeringID := offering.ID
	scheduleID := schedule.ID
	session := models.AttendanceSession{
		CourseOfferingID: &offeringID,
		ScheduleID:       &scheduleID,
		ClassID:          offering.ClassID,
		CourseID:         offering.CourseID,
		TeacherID:        teacher.ID,
		Code:             code,
		ClassDate:        dayStart,
		ExpiresAt:        now.Add(attendanceQRDuration),
		IsActive:         true,
		Note:             strings.TrimSpace(req.Note),
	}

	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.AttendanceSession{}).
			Where("course_offering_id = ? AND is_active = ?", offering.ID, true).
			Updates(map[string]interface{}{"is_active": false}).Error; err != nil {
			return err
		}
		if err := tx.Create(&session).Error; err != nil {
			return err
		}
		for _, enrollment := range enrollments {
			sessionID := session.ID
			attendance := models.Attendance{
				EnrollmentID:        enrollment.ID,
				AttendanceSessionID: &sessionID,
				ClassDate:           dayStart,
				Status:              "absent",
				Note:                "Chưa điểm danh",
			}
			if err := tx.Create(&attendance).Error; err != nil {
				return err
			}
		}
		return nil
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
	enrollmentQuery := config.DB.Preload("Student.User").Preload("Course").Where("status <> ?", "cancelled")
	if session.CourseOfferingID != nil {
		enrollmentQuery = enrollmentQuery.Where("course_offering_id = ?", *session.CourseOfferingID)
	} else {
		enrollmentQuery = enrollmentQuery.Where("class_id = ? AND course_id = ?", session.ClassID, session.CourseID)
	}
	if err := enrollmentQuery.Order("student_id").Find(&enrollments).Error; err != nil {
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
		if !found {
			sessionID := session.ID
			attendance = models.Attendance{
				EnrollmentID:        enrollment.ID,
				AttendanceSessionID: &sessionID,
				ClassDate:           session.ClassDate,
				Status:              "absent",
				Note:                "Chưa điểm danh",
			}
			if err := config.DB.Create(&attendance).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Không thể tạo bản ghi điểm danh mặc định", "error": err.Error()})
				return
			}
		}
		records = append(records, gin.H{
			"enrollment": enrollment,
			"attendance": attendance,
			"status":     attendance.Status,
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
	if err := config.DB.Preload("Enrollment.Class").Preload("AttendanceSession").First(&attendance, attendanceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy bản ghi điểm danh"})
		return
	}

	canUpdate := attendance.AttendanceSessionID != nil && attendance.AttendanceSession.TeacherID == teacher.ID
	if attendance.AttendanceSessionID == nil {
		canUpdate = attendance.Enrollment.Class.TeacherID == teacher.ID
	}
	if !canUpdate {
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

func ListAttendanceSessions(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	query := config.DB.Preload("Class").Preload("Course").
		Where("teacher_id = ?", teacher.ID)
	if classID := strings.TrimSpace(c.Query("classId")); classID != "" {
		query = query.Where("class_id = ?", classID)
	}
	if active := strings.TrimSpace(c.Query("active")); active != "" {
		if active == "true" || active == "1" {
			query = query.Where("is_active = ?", true)
		} else if active == "false" || active == "0" {
			query = query.Where("is_active = ?", false)
		}
	}

	var sessions []models.AttendanceSession
	if err := query.Preload("Schedule").Preload("CourseOffering.Semester").Order("created_at desc").Limit(100).Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách phiên điểm danh", "error": err.Error()})
		return
	}
	for i := range sessions {
		closeAttendanceSessionWhenLessonEnded(&sessions[i], attendanceNow())
	}
	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách phiên điểm danh thành công", "data": sessions})
}

func CloseAttendanceSession(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}
	session, ok := findTeacherAttendanceSession(c, teacher.ID)
	if !ok {
		return
	}
	if !session.IsActive {
		c.JSON(http.StatusOK, gin.H{"message": "Phiên điểm danh đã đóng", "data": session})
		return
	}
	session.IsActive = false
	if err := config.DB.Save(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không thể đóng phiên điểm danh", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã đóng phiên điểm danh", "data": session})
}

func findTeacherAttendanceSession(c *gin.Context, teacherID uint) (models.AttendanceSession, bool) {
	sessionID, err := strconv.Atoi(c.Param("id"))
	if err != nil || sessionID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID phiên điểm danh không hợp lệ"})
		return models.AttendanceSession{}, false
	}

	var session models.AttendanceSession
	if err := config.DB.Preload("Class").Preload("Course").Preload("Schedule").Preload("CourseOffering.Semester").First(&session, sessionID).Error; err != nil {
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

	now := attendanceNow()
	if closeAttendanceSessionWhenLessonEnded(session, now) {
		return false, nil
	}
	if now.Before(session.ExpiresAt) {
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
	session.ExpiresAt = attendanceNow().Add(attendanceQRDuration)
	session.IsActive = true

	return config.DB.Save(session).Error
}

func attendanceLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.Local
	}
	return location
}

func attendanceNow() time.Time { return time.Now().In(attendanceLocation()) }

func scheduleMatchesDate(schedule models.Schedule, date time.Time) bool {
	day, ok := normalizeScheduleDay(schedule.DayOfWeek)
	if !ok {
		return false
	}
	wanted := map[time.Weekday]string{
		time.Monday: "Mon", time.Tuesday: "Tue", time.Wednesday: "Wed", time.Thursday: "Thu",
		time.Friday: "Fri", time.Saturday: "Sat", time.Sunday: "Sun",
	}[date.Weekday()]
	return day == wanted
}

func attendanceLessonWindow(schedule models.Schedule, date time.Time) (time.Time, time.Time, error) {
	start := timeToMinutes(schedule.StartTime)
	end := timeToMinutes(schedule.EndTime)
	if start < 0 || end <= start {
		return time.Time{}, time.Time{}, errors.New("invalid lesson time")
	}
	location := date.Location()
	lessonStart := time.Date(date.Year(), date.Month(), date.Day(), start/60, start%60, 0, 0, location)
	lessonEnd := time.Date(date.Year(), date.Month(), date.Day(), end/60, end%60, 0, 0, location)
	return lessonStart, lessonEnd, nil
}

func closeAttendanceSessionWhenLessonEnded(session *models.AttendanceSession, now time.Time) bool {
	if !session.IsActive {
		return false
	}
	classDate := session.ClassDate.In(now.Location())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	sessionDay := time.Date(classDate.Year(), classDate.Month(), classDate.Day(), 0, 0, 0, 0, now.Location())
	ended := sessionDay.Before(today)
	if !ended && sessionDay.Equal(today) && session.Schedule != nil {
		_, lessonEnd, err := attendanceLessonWindow(*session.Schedule, now)
		ended = err == nil && now.After(lessonEnd)
	}
	if ended {
		session.IsActive = false
		_ = config.DB.Model(session).Update("is_active", false).Error
	}
	return ended
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
