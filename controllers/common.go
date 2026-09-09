package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"student-management/config"
	"student-management/middleware"
	"student-management/models"

	"github.com/gin-gonic/gin"
)

func getCurrentUserID(c *gin.Context) (uint, bool) {
	value, exists := c.Get(middleware.ContextUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Vui lòng đăng nhập"})
		return 0, false
	}

	switch v := value.(type) {
	case uint:
		return v, true
	case int:
		return uint(v), true
	case int64:
		return uint(v), true
	case float64:
		return uint(v), true
	default:
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Token không hợp lệ"})
		return 0, false
	}
}

func getCurrentStudent(c *gin.Context) (models.Student, bool) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		return models.Student{}, false
	}

	var student models.Student
	if err := config.DB.Where("user_id = ?", userID).First(&student).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"message": "Tài khoản không phải sinh viên"})
		return models.Student{}, false
	}

	return student, true
}

func getCurrentTeacher(c *gin.Context) (models.Teacher, bool) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		return models.Teacher{}, false
	}

	var teacher models.Teacher
	if err := config.DB.Where("user_id = ?", userID).First(&teacher).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"message": "Tài khoản không phải giảng viên"})
		return models.Teacher{}, false
	}

	return teacher, true
}

func parseDateTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)

	layouts := []string{
		time.RFC3339,

		// YYYY-MM-DD
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",

		// Định dạng Excel kiểu Mỹ: MM/DD/YYYY
		"1/2/2006",
		"01/02/2006",

		// Định dạng Việt Nam: DD/MM/YYYY
		"2/1/2006",
		"02/01/2006",
	}

	var lastErr error

	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(
			layout,
			value,
			time.Local,
		)

		if err == nil {
			return parsed, nil
		}

		lastErr = err
	}

	return time.Time{}, lastErr
}

// timeToMinutes chuyển "HH:MM" hoặc "H:MM" thành số phút từ 00:00. Trả về -1 nếu không parse được.
func timeToMinutes(t string) int {
	t = strings.TrimSpace(t)
	if t == "" {
		return -1
	}
	parts := strings.Split(t, ":")
	if len(parts) < 2 {
		return -1
	}
	h, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	m, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return -1
	}
	return h*60 + m
}

// schedulesOverlap kiểm tra hai lịch có trùng ngày và giao khoảng thời gian không.
func schedulesOverlap(a, b models.Schedule) bool {
	if strings.EqualFold(strings.TrimSpace(a.DayOfWeek), strings.TrimSpace(b.DayOfWeek)) == false {
		return false
	}
	// Ưu tiên StartTime/EndTime; nếu thiếu thì coi Session giống nhau là trùng.
	as := timeToMinutes(a.StartTime)
	ae := timeToMinutes(a.EndTime)
	bs := timeToMinutes(b.StartTime)
	be := timeToMinutes(b.EndTime)
	if as >= 0 && ae >= 0 && bs >= 0 && be >= 0 {
		// Giao nhau: as < be && bs < ae
		return as < be && bs < ae
	}
	// Fallback theo Session
	return strings.EqualFold(strings.TrimSpace(a.Session), strings.TrimSpace(b.Session)) &&
		strings.TrimSpace(a.Session) != ""
}

// checkTeacherScheduleConflict trả về thông báo lỗi nếu giáo viên đã có lớp trùng lịch với schedules của lớp mới.
// excludeClassID: bỏ qua lớp đang được gán lại (0 nếu tạo lớp mới).
func checkTeacherScheduleConflict(teacherID uint, excludeClassID uint, newSchedules []models.Schedule) error {
	if teacherID == 0 || len(newSchedules) == 0 {
		return nil
	}

	// Lấy tất cả lớp hiện tại của giáo viên (trừ lớp đang gán lại)
	var existingClasses []models.Class
	query := config.DB.Preload("Schedules").Where("teacher_id = ?", teacherID)
	if excludeClassID > 0 {
		query = query.Where("id <> ?", excludeClassID)
	}
	if err := query.Find(&existingClasses).Error; err != nil {
		return err
	}

	for _, cls := range existingClasses {
		for _, existSch := range cls.Schedules {
			for _, newSch := range newSchedules {
				if schedulesOverlap(existSch, newSch) {
					return fmt.Errorf(
						"giảng viên đã có lịch trùng: lớp %s (%s %s-%s) trùng với lịch mới (%s %s-%s)",
						cls.ClassCode,
						existSch.DayOfWeek, existSch.StartTime, existSch.EndTime,
						newSch.DayOfWeek, newSch.StartTime, newSch.EndTime,
					)
				}
			}
		}
	}
	return nil
}

// checkRoomScheduleConflict trả về thông báo lỗi nếu phòng học đã được dùng bởi lớp khác trùng lịch.
// excludeClassID: bỏ qua lớp đang được tạo/cập nhật (0 nếu tạo lớp mới).
func checkRoomScheduleConflict(roomID uint, excludeClassID uint, newSchedules []models.Schedule) error {
	if roomID == 0 || len(newSchedules) == 0 {
		return nil
	}

	var existingClasses []models.Class
	query := config.DB.Preload("Schedules").Where("room_id = ?", roomID)
	if excludeClassID > 0 {
		query = query.Where("id <> ?", excludeClassID)
	}
	if err := query.Find(&existingClasses).Error; err != nil {
		return err
	}

	for _, cls := range existingClasses {
		for _, existSch := range cls.Schedules {
			for _, newSch := range newSchedules {
				if schedulesOverlap(existSch, newSch) {
					return fmt.Errorf(
						"phòng học đã được sử dụng: lớp %s (%s %s-%s) trùng với lịch mới (%s %s-%s)",
						cls.ClassCode,
						existSch.DayOfWeek, existSch.StartTime, existSch.EndTime,
						newSch.DayOfWeek, newSch.StartTime, newSch.EndTime,
					)
				}
			}
		}
	}
	return nil
}
