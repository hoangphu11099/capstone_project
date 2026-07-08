package controllers

import (
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

type ScheduleInput struct {
	DayOfWeek string `json:"dayOfWeek"`
	Session   string `json:"session"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type CreateClassRequest struct {
	ClassCode   string          `json:"classCode" binding:"required"`
	MajorID     uint            `json:"majorId" binding:"required"`
	TeacherID   uint            `json:"teacherId"`
	SemesterID  uint            `json:"semesterId" binding:"required"`
	RoomID      uint            `json:"roomId" binding:"required"`
	MaxStudents int             `json:"maxStudents"`
	Status      string          `json:"status"`
	Schedules   []ScheduleInput `json:"schedules"`
}

type AssignTeacherRequest struct {
	TeacherID uint   `json:"teacherId" binding:"required"`
	Note      string `json:"note"`
}

func ListClasses(c *gin.Context) {
	var classes []models.Class
	query := config.DB.Preload("Major").Preload("Teacher.User").Preload("Room").Preload("Semester")

	if q := strings.TrimSpace(c.Query("q")); q != "" {
		query = query.Where("class_code LIKE ?", "%"+q+"%")
	}
	if semesterID := strings.TrimSpace(c.Query("semesterId")); semesterID != "" {
		query = query.Where("semester_id = ?", semesterID)
	}
	if teacherID := strings.TrimSpace(c.Query("teacherId")); teacherID != "" {
		query = query.Where("teacher_id = ?", teacherID)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("class_code").Find(&classes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách lớp", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách lớp thành công", "data": classes})
}

func CreateClass(c *gin.Context) {
	adminUserID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	var req CreateClassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu tạo lớp không hợp lệ", "error": err.Error()})
		return
	}

	if req.MaxStudents < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Số lượng sinh viên tối đa không hợp lệ"})
		return
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "open"
	}

	class := models.Class{
		ClassCode:   strings.TrimSpace(req.ClassCode),
		MajorID:     req.MajorID,
		TeacherID:   req.TeacherID,
		SemesterID:  req.SemesterID,
		RoomID:      req.RoomID,
		MaxStudents: req.MaxStudents,
		Status:      status,
	}

	var classTeacher *models.ClassTeacher
	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&class).Error; err != nil {
			return err
		}

		for _, scheduleInput := range req.Schedules {
			if strings.TrimSpace(scheduleInput.DayOfWeek) == "" || strings.TrimSpace(scheduleInput.Session) == "" {
				continue
			}
			schedule := models.Schedule{
				ClassID:   class.ID,
				DayOfWeek: strings.TrimSpace(scheduleInput.DayOfWeek),
				Session:   strings.TrimSpace(scheduleInput.Session),
				StartTime: strings.TrimSpace(scheduleInput.StartTime),
				EndTime:   strings.TrimSpace(scheduleInput.EndTime),
			}
			if err := tx.Create(&schedule).Error; err != nil {
				return err
			}
		}

		if req.TeacherID > 0 {
			ct := models.ClassTeacher{
				ClassID:          class.ID,
				TeacherID:        req.TeacherID,
				AssignedByUserID: adminUserID,
				Status:           "active",
				AssignedAt:       time.Now(),
				Note:             "Phân công khi tạo lớp",
			}
			if err := tx.Create(&ct).Error; err != nil {
				return err
			}
			classTeacher = &ct
		}

		return nil
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Tạo lớp thất bại. Mã lớp có thể đã tồn tại", "error": err.Error()})
		return
	}

	config.DB.Preload("Major").Preload("Teacher.User").Preload("Room").Preload("Semester").Preload("Schedules").First(&class, class.ID)
	c.JSON(http.StatusCreated, gin.H{"message": "Tạo lớp thành công", "data": gin.H{"class": class, "classTeacher": classTeacher}})
}

func AssignTeacher(c *gin.Context) {
	adminUserID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	classID, err := strconv.Atoi(c.Param("id"))
	if err != nil || classID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID lớp không hợp lệ"})
		return
	}

	var req AssignTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu phân công giảng viên không hợp lệ", "error": err.Error()})
		return
	}

	var class models.Class
	if err := config.DB.First(&class, classID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy lớp"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy lớp", "error": err.Error()})
		return
	}

	var teacher models.Teacher
	if err := config.DB.First(&teacher, req.TeacherID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy giảng viên"})
		return
	}

	var classTeacher models.ClassTeacher
	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		class.TeacherID = teacher.ID
		if err := tx.Save(&class).Error; err != nil {
			return err
		}

		if err := tx.Model(&models.ClassTeacher{}).
			Where("class_id = ? AND status = ?", class.ID, "active").
			Update("status", "replaced").Error; err != nil {
			return err
		}

		classTeacher = models.ClassTeacher{
			ClassID:          class.ID,
			TeacherID:        teacher.ID,
			AssignedByUserID: adminUserID,
			Status:           "active",
			AssignedAt:       time.Now(),
			Note:             req.Note,
		}
		return tx.Create(&classTeacher).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Phân công giảng viên thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Phân công giảng viên thành công", "data": gin.H{"class": class, "classTeacher": classTeacher}})
}
