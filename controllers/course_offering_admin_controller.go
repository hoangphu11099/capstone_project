package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AssignCourseOfferingRequest struct {
	SemesterID    uint     `json:"semesterId" binding:"required"`
	CourseID      uint     `json:"courseId" binding:"required"`
	TeacherID     uint     `json:"teacherId" binding:"required"`
	RoomID        uint     `json:"roomId" binding:"required"`
	DayOfWeek     string   `json:"dayOfWeek"`
	TeachingDate  string   `json:"teachingDate"`
	TeachingDates []string `json:"teachingDates"`
	Session       string   `json:"session"`
	StartTime     string   `json:"startTime" binding:"required"`
	EndTime       string   `json:"endTime" binding:"required"`
}

// AssignCourseOffering nối đầy đủ lớp, môn, giảng viên, học kỳ, phòng và lịch.
// Nếu học phần đã tồn tại thì đây là thao tác phân công/cập nhật lại lịch.
func AssignCourseOffering(c *gin.Context) {
	classID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || classID64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID lớp không hợp lệ"})
		return
	}

	var req AssignCourseOfferingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu phân công môn dạy không hợp lệ", "error": err.Error()})
		return
	}

	candidates, err := assignmentSchedules(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	if err := validateFutureTeachingDates(candidates, attendanceNow()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	classID := uint(classID64)
	var savedOffering models.CourseOffering
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		var class models.Class
		if err := tx.Preload("Semester").First(&class, classID).Error; err != nil {
			return err
		}
		var semester models.Semester
		if err := tx.First(&semester, req.SemesterID).Error; err != nil {
			return err
		}
		if semester.Status == "closed" {
			return errors.New("học kỳ đã đóng; hãy chọn học kỳ đang mở hoặc sắp tới")
		}
		if class.Status != "open" {
			return errors.New("lớp đã đóng")
		}
		var course models.Course
		if err := tx.First(&course, req.CourseID).Error; err != nil {
			return err
		}
		if course.MajorID != class.MajorID {
			return errors.New("môn học không thuộc chuyên ngành của lớp")
		}
		var teacher models.Teacher
		if err := tx.First(&teacher, req.TeacherID).Error; err != nil {
			return err
		}
		if teacher.MajorID != 0 && teacher.MajorID != class.MajorID {
			return errors.New("giảng viên không thuộc chuyên ngành của lớp")
		}
		var room models.Room
		if err := tx.First(&room, req.RoomID).Error; err != nil {
			return err
		}

		var offering models.CourseOffering
		lookup := tx.Where("class_id = ? AND course_id = ? AND semester_id = ?", class.ID, course.ID, req.SemesterID).First(&offering)
		if lookup.Error != nil && !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return lookup.Error
		}
		excludeID := offering.ID
		var retained []models.Schedule
		if offering.ID != 0 {
			if err := tx.Where("course_offering_id = ?", offering.ID).Find(&retained).Error; err != nil {
				return err
			}
		}
		for _, existing := range retained {
			replaced := false
			for _, candidate := range candidates {
				if (existing.FullDate == nil && candidate.FullDate == nil) || (existing.FullDate != nil && candidate.FullDate != nil && existing.FullDate.Format("2006-01-02") == candidate.FullDate.Format("2006-01-02")) {
					replaced = true
					break
				}
			}
			if replaced {
				continue
			}
			for _, candidate := range candidates {
				if schedulesOverlap(existing, candidate) {
					return errors.New("lịch mới trùng với lịch hiện có của học phần")
				}
			}
			if err := checkOfferingScheduleConflict(tx, req.TeacherID, req.RoomID, req.SemesterID, class.ID, excludeID, existing); err != nil {
				return err
			}
		}
		for _, candidate := range candidates {
			if candidate.FullDate != nil && (candidate.FullDate.Format("2006-01-02") < semester.StartDate.Format("2006-01-02") || candidate.FullDate.Format("2006-01-02") > semester.EndDate.Format("2006-01-02")) {
				return errors.New("ngày dạy phải nằm trong học kỳ")
			}
			if err := checkOfferingScheduleConflict(tx, req.TeacherID, req.RoomID, req.SemesterID, class.ID, excludeID, candidate); err != nil {
				return err
			}
		}

		offering.CourseID = course.ID
		offering.ClassID = class.ID
		offering.TeacherID = teacher.ID
		offering.SemesterID = req.SemesterID
		offering.RoomID = room.ID
		offering.Status = "open"
		if offering.ID == 0 {
			if err := tx.Create(&offering).Error; err != nil {
				return err
			}
		} else if err := tx.Save(&offering).Error; err != nil {
			return err
		}

		offeringID := offering.ID
		for _, candidate := range candidates {
			var schedule models.Schedule
			query := tx.Where("course_offering_id = ?", offering.ID)
			if candidate.FullDate != nil {
				query = query.Where("full_date = ?", candidate.FullDate.Format("2006-01-02"))
			} else {
				query = query.Where("full_date IS NULL")
			}
			lookup := query.First(&schedule)
			if lookup.Error != nil && !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
				return lookup.Error
			}
			candidate.ID = schedule.ID
			candidate.ClassID = class.ID
			candidate.CourseOfferingID = &offeringID
			if err := tx.Save(&candidate).Error; err != nil {
				return err
			}
		}

		var students []models.Student
		if err := tx.Where("class_id = ?", class.ID).Find(&students).Error; err != nil {
			return err
		}
		for _, student := range students {
			if err := ensureEnrollment(tx, student.ID, offering); err != nil {
				return err
			}
		}
		savedOffering = offering
		return nil
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"message": "Phân công môn dạy thất bại: " + err.Error(), "error": err.Error()})
		return
	}

	config.DB.Preload("Course").Preload("Teacher.User").Preload("Room").Preload("Semester").Preload("Schedules").First(&savedOffering, savedOffering.ID)
	c.JSON(http.StatusOK, gin.H{"message": "Đã phân công môn dạy và đồng bộ danh sách sinh viên", "data": savedOffering})
}

func normalizeScheduleDay(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "mon", "monday", "thứ 2", "thứ hai":
		return "Mon", true
	case "tue", "tuesday", "thứ 3", "thứ ba":
		return "Tue", true
	case "wed", "wednesday", "thứ 4", "thứ tư":
		return "Wed", true
	case "thu", "thursday", "thứ 5", "thứ năm":
		return "Thu", true
	case "fri", "friday", "thứ 6", "thứ sáu":
		return "Fri", true
	case "sat", "saturday", "thứ 7", "thứ bảy":
		return "Sat", true
	case "sun", "sunday", "chủ nhật", "cn":
		return "Sun", true
	default:
		return "", false
	}
}

func normalizeClock(value string) string {
	minutes := timeToMinutes(value)
	if minutes < 0 {
		return ""
	}
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

func checkOfferingScheduleConflict(tx *gorm.DB, teacherID, roomID, semesterID, classID, excludeOfferingID uint, candidate models.Schedule) error {
	var semester models.Semester
	if err := tx.First(&semester, semesterID).Error; err != nil {
		return err
	}
	var schedules []models.Schedule
	query := tx.Preload("CourseOffering.Class").Preload("CourseOffering.Semester").
		Joins("JOIN course_offerings co ON co.id = schedules.course_offering_id").
		Where("co.status = ? AND (co.teacher_id = ? OR co.room_id = ? OR co.class_id = ?)", "open", teacherID, roomID, classID)
	if excludeOfferingID > 0 {
		query = query.Where("co.id <> ?", excludeOfferingID)
	}
	if err := query.Find(&schedules).Error; err != nil {
		return err
	}
	for _, existing := range schedules {
		other := existing.CourseOffering
		if other == nil || other.Semester.EndDate.Before(semester.StartDate) || semester.EndDate.Before(other.Semester.StartDate) {
			continue
		}
		if candidate.FullDate != nil && (candidate.FullDate.Format("2006-01-02") < other.Semester.StartDate.Format("2006-01-02") || candidate.FullDate.Format("2006-01-02") > other.Semester.EndDate.Format("2006-01-02")) {
			continue
		}
		if existing.FullDate != nil && (existing.FullDate.Format("2006-01-02") < semester.StartDate.Format("2006-01-02") || existing.FullDate.Format("2006-01-02") > semester.EndDate.Format("2006-01-02")) {
			continue
		}
		if !schedulesOverlap(existing, candidate) {
			continue
		}
		if other.ClassID == classID {
			return fmt.Errorf("lớp %s đã có môn học trùng giờ", other.Class.ClassCode)
		}
		if other.TeacherID == teacherID {
			return fmt.Errorf("giảng viên đã có lịch trùng với lớp %s (%s %s-%s)", other.Class.ClassCode, existing.DayOfWeek, existing.StartTime, existing.EndTime)
		}
		if other.RoomID == roomID {
			return fmt.Errorf("phòng học đã được dùng bởi lớp %s (%s %s-%s)", other.Class.ClassCode, existing.DayOfWeek, existing.StartTime, existing.EndTime)
		}
	}
	return nil
}

func ensureEnrollment(tx *gorm.DB, studentID uint, offering models.CourseOffering) error {
	offeringID := offering.ID
	var enrollment models.Enrollment
	err := tx.Where("student_id = ? AND course_offering_id = ?", studentID, offering.ID).First(&enrollment).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err == nil {
		return nil
	}
	enrollment.CourseOfferingID = &offeringID
	enrollment.StudentID = studentID
	enrollment.ClassID = offering.ClassID
	enrollment.CourseID = offering.CourseID
	if enrollment.EnrollDate.IsZero() {
		enrollment.EnrollDate = time.Now()
	}
	enrollment.Status = "enrolled"
	if enrollment.ID == 0 {
		return tx.Create(&enrollment).Error
	}
	return tx.Save(&enrollment).Error
}

func syncStudentClassEnrollments(tx *gorm.DB, studentID, classID uint) error {
	var offerings []models.CourseOffering
	if err := tx.Where("class_id = ? AND status = ? AND semester_id IN (SELECT id FROM semesters WHERE status <> ? AND end_date >= ?)", classID, "open", "closed", attendanceNow().Format("2006-01-02")).Find(&offerings).Error; err != nil {
		return err
	}
	for _, offering := range offerings {
		if err := ensureEnrollment(tx, studentID, offering); err != nil {
			return err
		}
	}
	return nil
}

// assignmentSchedules keeps calendar dates intact; legacy clients may still send a weekday.
func assignmentSchedules(req AssignCourseOfferingRequest) ([]models.Schedule, error) {
	start, end := normalizeClock(req.StartTime), normalizeClock(req.EndTime)
	if start == "" || end == "" || timeToMinutes(start) >= timeToMinutes(end) {
		return nil, errors.New("giờ bắt đầu/kết thúc không hợp lệ")
	}
	dates := append([]string{}, req.TeachingDates...)
	if req.TeachingDate != "" {
		dates = append(dates, req.TeachingDate)
	}
	base := models.Schedule{Session: strings.TrimSpace(req.Session), StartTime: start, EndTime: end}
	if len(dates) == 0 {
		day, ok := normalizeScheduleDay(req.DayOfWeek)
		if !ok {
			return nil, errors.New("vui lòng chọn ngày dạy hợp lệ")
		}
		base.DayOfWeek = day
		return []models.Schedule{base}, nil
	}
	schedules := []models.Schedule{}
	seen := map[string]bool{}
	for _, value := range dates {
		date, err := time.ParseInLocation("2006-01-02", value, attendanceLocation())
		if err != nil {
			return nil, errors.New("ngày dạy phải có định dạng YYYY-MM-DD")
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		candidate := base
		candidate.FullDate = &date
		candidate.DayOfWeek = date.Format("Mon")
		schedules = append(schedules, candidate)
	}
	return schedules, nil
}

// Compare calendar dates in the school's timezone, allowing any lesson today.
func validateFutureTeachingDates(schedules []models.Schedule, now time.Time) error {
	today := now.In(attendanceLocation()).Format("2006-01-02")
	for _, schedule := range schedules {
		if schedule.FullDate != nil && schedule.FullDate.Format("2006-01-02") < today {
			return errors.New("không được tạo hoặc sửa lịch dạy vào ngày trong quá khứ")
		}
	}
	return nil
}
