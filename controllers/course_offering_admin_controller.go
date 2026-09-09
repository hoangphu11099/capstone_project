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
	CourseID  uint   `json:"courseId" binding:"required"`
	TeacherID uint   `json:"teacherId" binding:"required"`
	RoomID    uint   `json:"roomId" binding:"required"`
	DayOfWeek string `json:"dayOfWeek" binding:"required"`
	Session   string `json:"session"`
	StartTime string `json:"startTime" binding:"required"`
	EndTime   string `json:"endTime" binding:"required"`
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

	day, ok := normalizeScheduleDay(req.DayOfWeek)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Thứ trong tuần không hợp lệ"})
		return
	}
	startTime := normalizeClock(req.StartTime)
	endTime := normalizeClock(req.EndTime)
	if startTime == "" || endTime == "" || timeToMinutes(startTime) >= timeToMinutes(endTime) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Giờ bắt đầu/kết thúc không hợp lệ"})
		return
	}

	classID := uint(classID64)
	var savedOffering models.CourseOffering
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		var class models.Class
		if err := tx.Preload("Semester").First(&class, classID).Error; err != nil {
			return err
		}
		var course models.Course
		if err := tx.First(&course, req.CourseID).Error; err != nil {
			return err
		}
		if course.MajorID != class.MajorID || course.SemesterID != class.SemesterID {
			return errors.New("môn học không thuộc đúng chuyên ngành và học kỳ của lớp")
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
		lookup := tx.Where("class_id = ? AND course_id = ? AND semester_id = ?", class.ID, course.ID, class.SemesterID).First(&offering)
		if lookup.Error != nil && !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return lookup.Error
		}
		excludeID := offering.ID
		candidate := models.Schedule{DayOfWeek: day, StartTime: startTime, EndTime: endTime}
		if err := checkOfferingScheduleConflict(tx, req.TeacherID, req.RoomID, class.SemesterID, excludeID, candidate); err != nil {
			return err
		}

		offering.CourseID = course.ID
		offering.ClassID = class.ID
		offering.TeacherID = teacher.ID
		offering.SemesterID = class.SemesterID
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
		var schedule models.Schedule
		scheduleLookup := tx.Where("course_offering_id = ?", offering.ID).First(&schedule)
		if scheduleLookup.Error != nil && !errors.Is(scheduleLookup.Error, gorm.ErrRecordNotFound) {
			return scheduleLookup.Error
		}
		schedule.ClassID = class.ID
		schedule.CourseOfferingID = &offeringID
		schedule.DayOfWeek = day
		schedule.Session = strings.TrimSpace(req.Session)
		schedule.StartTime = startTime
		schedule.EndTime = endTime
		if schedule.ID == 0 {
			if err := tx.Create(&schedule).Error; err != nil {
				return err
			}
		} else if err := tx.Save(&schedule).Error; err != nil {
			return err
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

func checkOfferingScheduleConflict(tx *gorm.DB, teacherID, roomID, semesterID, excludeOfferingID uint, candidate models.Schedule) error {
	var semester models.Semester
	if err := tx.First(&semester, semesterID).Error; err != nil {
		return err
	}
	var schedules []models.Schedule
	query := tx.Preload("CourseOffering.Class").Preload("CourseOffering.Semester").
		Joins("JOIN course_offerings co ON co.id = schedules.course_offering_id").
		Where("co.status = ? AND (co.teacher_id = ? OR co.room_id = ?)", "open", teacherID, roomID)
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
		if !schedulesOverlap(existing, candidate) {
			continue
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
	if err := tx.Model(&models.Enrollment{}).
		Where("student_id = ? AND class_id <> ? AND status <> ?", studentID, classID, "cancelled").
		Update("status", "cancelled").Error; err != nil {
		return err
	}
	var offerings []models.CourseOffering
	if err := tx.Where("class_id = ? AND status = ?", classID, "open").Find(&offerings).Error; err != nil {
		return err
	}
	for _, offering := range offerings {
		if err := ensureEnrollment(tx, studentID, offering); err != nil {
			return err
		}
	}
	return nil
}
