package controllers

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"strings"
	"student-management/config"
	"student-management/models"
)

type RescheduleLessonRequest struct {
	TeachingDate      string `json:"teachingDate" binding:"required"`
	StartTime         string `json:"startTime" binding:"required"`
	EndTime           string `json:"endTime" binding:"required"`
	Session           string `json:"session"`
	Reason            string `json:"reason" binding:"required"`
	ExpectedDate      string `json:"expectedDate" binding:"required"`
	ExpectedStartTime string `json:"expectedStartTime" binding:"required"`
	ExpectedEndTime   string `json:"expectedEndTime" binding:"required"`
}

// RescheduleLesson moves a single accepted lesson, preserving its teacher,
// room, enrollment and schedule ID. Other lessons are never rewritten.
func RescheduleLesson(c *gin.Context) {
	classID, e1 := strconv.ParseUint(c.Param("id"), 10, 64)
	scheduleID, e2 := strconv.ParseUint(c.Param("scheduleId"), 10, 64)
	var req RescheduleLessonRequest
	if e1 != nil || e2 != nil || classID == 0 || scheduleID == 0 || c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"message": "Vui lòng chọn buổi học và nhập đầy đủ ngày, giờ học bù, lý do"})
		return
	}
	if strings.TrimSpace(req.Reason) == "" || len([]rune(strings.TrimSpace(req.Reason))) > 1000 {
		c.JSON(400, gin.H{"message": "Vui lòng ghi lý do đổi lịch (tối đa 1000 ký tự)"})
		return
	}
	candidates, err := assignmentSchedules(AssignCourseOfferingRequest{TeachingDate: req.TeachingDate, StartTime: req.StartTime, EndTime: req.EndTime, Session: req.Session})
	if err == nil {
		err = validateOfferFutureStarts(candidates, attendanceNow())
	}
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	candidate := candidates[0]
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		var class models.Class
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&class, classID).Error; err != nil {
			return err
		}
		if class.Status != "open" {
			return errors.New("Lớp đã đóng, không thể đổi lịch")
		}
		var schedule models.Schedule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND class_id = ?", scheduleID, classID).First(&schedule).Error; err != nil {
			return err
		}
		if schedule.CourseOfferingID == nil || schedule.FullDate == nil {
			return errors.New("Chỉ có thể đổi buổi học đã phân công theo ngày cụ thể")
		}
		if schedule.FullDate.Format("2006-01-02") != req.ExpectedDate || normalizeClock(schedule.StartTime) != normalizeClock(req.ExpectedStartTime) || normalizeClock(schedule.EndTime) != normalizeClock(req.ExpectedEndTime) {
			return errors.New("Lịch đã được người khác cập nhật. Vui lòng tải lại chi tiết lớp trước khi sửa")
		}
		var offering models.CourseOffering
		if err := tx.Preload("Semester").First(&offering, *schedule.CourseOfferingID).Error; err != nil {
			return err
		}
		if offering.Status != "open" || offering.Semester.Status == "closed" {
			return errors.New("Học phần hoặc học kỳ đã đóng, không thể đổi lịch")
		}
		if offering.ClassID != uint(classID) {
			return errors.New("Buổi học không thuộc lớp này")
		}
		if req.TeachingDate < offering.Semester.StartDate.Format("2006-01-02") || req.TeachingDate > offering.Semester.EndDate.Format("2006-01-02") {
			return errors.New("Ngày học bù phải nằm trong học kỳ của môn")
		}
		var count int64
		if err := tx.Model(&models.AttendanceSession{}).Where("schedule_id = ? OR (course_offering_id = ? AND DATE(class_date) = ?)", schedule.ID, offering.ID, schedule.FullDate.Format("2006-01-02")).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("Buổi học đã có phiên điểm danh, không thể dời lịch để tránh thay đổi dữ liệu chuyên cần")
		}
		var teacher models.Teacher
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&teacher, offering.TeacherID).Error; err != nil {
			return err
		}
		var room models.Room
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&room, offering.RoomID).Error; err != nil {
			return err
		}
		var siblings []models.Schedule
		if err := tx.Where("course_offering_id = ? AND id <> ?", offering.ID, schedule.ID).Find(&siblings).Error; err != nil {
			return err
		}
		for _, other := range siblings {
			if other.FullDate != nil && other.FullDate.Format("2006-01-02") == req.TeachingDate {
				return errors.New("Ngày học bù đã có buổi khác của môn này. Vui lòng chọn ngày khác")
			}
			if schedulesOverlap(other, candidate) {
				return errors.New("Lịch học bù trùng với buổi khác của môn này")
			}
		}
		if err := checkOfferingScheduleConflict(tx, offering.TeacherID, offering.RoomID, offering.SemesterID, uint(classID), offering.ID, candidate); err != nil {
			return err
		}
		original := schedule.OriginalDate
		if original == nil {
			original = schedule.FullDate
		}
		return tx.Model(&schedule).Updates(map[string]interface{}{"full_date": candidate.FullDate, "day_of_week": candidate.DayOfWeek, "session": candidate.Session, "start_time": candidate.StartTime, "end_time": candidate.EndTime, "original_date": original, "reschedule_reason": strings.TrimSpace(req.Reason)}).Error
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(404, gin.H{"message": "Không tìm thấy lớp hoặc buổi học"})
		return
	}
	if err != nil {
		c.JSON(400, gin.H{"message": fmt.Sprintf("Không thể đổi lịch: %s", err)})
		return
	}
	c.JSON(200, gin.H{"message": "Đã cập nhật buổi học bù vào lịch của lớp, giảng viên và sinh viên"})
}
