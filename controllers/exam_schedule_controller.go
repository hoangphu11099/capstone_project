package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ExamScheduleRequest struct {
	ClassID    uint   `json:"classId" binding:"required"`
	CourseID   uint   `json:"courseId" binding:"required"`
	SemesterID uint   `json:"semesterId" binding:"required"`
	RoomID     uint   `json:"roomId" binding:"required"`
	ExamDate   string `json:"examDate" binding:"required"`
	Session    string `json:"session" binding:"required"`
	StartTime  string `json:"startTime" binding:"required"`
	EndTime    string `json:"endTime" binding:"required"`
	ExamType   string `json:"examType"`
	Note       string `json:"note"`
}

func ListExamSchedules(c *gin.Context) {
	var schedules []models.ExamSchedule
	query := config.DB.Preload("Class").Preload("Course").Preload("Semester").Preload("Room")

	if classID := strings.TrimSpace(c.Query("classId")); classID != "" {
		query = query.Where("class_id = ?", classID)
	}
	if semesterID := strings.TrimSpace(c.Query("semesterId")); semesterID != "" {
		query = query.Where("semester_id = ?", semesterID)
	}

	if err := query.Order("exam_date asc, start_time asc").Find(&schedules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy lịch thi", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy lịch thi thành công", "data": schedules})
}

func CreateExamSchedule(c *gin.Context) {
	var req ExamScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu lịch thi không hợp lệ", "error": err.Error()})
		return
	}

	examDate, err := parseDateTime(req.ExamDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ngày thi không hợp lệ. Dùng dạng 2026-07-10 hoặc RFC3339"})
		return
	}

	if hasExamConflict(0, req.RoomID, examDate, req.StartTime, req.EndTime) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Phòng thi đã có lịch trong thời gian này"})
		return
	}

	if hasClassExam(0, req.ClassID, req.CourseID) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Lớp học phần đã có lịch thi cho môn này"})
		return
	}

	exam := models.ExamSchedule{
		ClassID:    req.ClassID,
		CourseID:   req.CourseID,
		SemesterID: req.SemesterID,
		RoomID:     req.RoomID,
		ExamDate:   examDate,
		Session:    strings.TrimSpace(req.Session),
		StartTime:  strings.TrimSpace(req.StartTime),
		EndTime:    strings.TrimSpace(req.EndTime),
		ExamType:   strings.TrimSpace(req.ExamType),
		Note:       strings.TrimSpace(req.Note),
	}

	if err := config.DB.Create(&exam).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Tạo lịch thi thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Tạo lịch thi thành công", "data": exam})
}

func UpdateExamSchedule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID lịch thi không hợp lệ"})
		return
	}

	var req ExamScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu lịch thi không hợp lệ", "error": err.Error()})
		return
	}

	examDate, err := parseDateTime(req.ExamDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ngày thi không hợp lệ"})
		return
	}

	var exam models.ExamSchedule
	if err := config.DB.First(&exam, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy lịch thi"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy lịch thi", "error": err.Error()})
		return
	}

	if hasExamConflict(uint(id), req.RoomID, examDate, req.StartTime, req.EndTime) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Phòng thi đã có lịch trong thời gian này"})
		return
	}

	if hasClassExam(uint(id), req.ClassID, req.CourseID) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Lớp học phần đã có lịch thi cho môn này"})
		return
	}

	exam.ClassID = req.ClassID
	exam.CourseID = req.CourseID
	exam.SemesterID = req.SemesterID
	exam.RoomID = req.RoomID
	exam.ExamDate = examDate
	exam.Session = strings.TrimSpace(req.Session)
	exam.StartTime = strings.TrimSpace(req.StartTime)
	exam.EndTime = strings.TrimSpace(req.EndTime)
	exam.ExamType = strings.TrimSpace(req.ExamType)
	exam.Note = strings.TrimSpace(req.Note)

	if err := config.DB.Save(&exam).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Cập nhật lịch thi thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cập nhật lịch thi thành công", "data": exam})
}

func DeleteExamSchedule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID lịch thi không hợp lệ"})
		return
	}

	if err := config.DB.Delete(&models.ExamSchedule{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Xóa lịch thi thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Xóa lịch thi thành công"})
}

func hasExamConflict(currentID uint, roomID uint, examDate interface{}, startTime string, endTime string) bool {
	query := config.DB.Model(&models.ExamSchedule{}).
		Where("room_id = ? AND DATE(exam_date) = DATE(?)", roomID, examDate).
		Where("NOT (end_time <= ? OR start_time >= ?)", startTime, endTime)

	if currentID > 0 {
		query = query.Where("id <> ?", currentID)
	}

	var count int64
	query.Count(&count)
	return count > 0
}

func hasClassExam(currentID uint, classID uint, courseID uint) bool {
	query := config.DB.Model(&models.ExamSchedule{}).Where("class_id = ? AND course_id = ?", classID, courseID)
	if currentID > 0 {
		query = query.Where("id <> ?", currentID)
	}
	var count int64
	query.Count(&count)
	return count > 0
}
