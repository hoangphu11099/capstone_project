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

type ExerciseRequest struct {
	ClassID     uint   `json:"classId" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Attachment  string `json:"attachment"`
	DueDate     string `json:"dueDate" binding:"required"`
}

type SubmissionRequest struct {
	Content string `json:"content"`
	FileURL string `json:"fileUrl"`
}

func CreateExercise(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	var req ExerciseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu bài tập không hợp lệ", "error": err.Error()})
		return
	}

	if !teacherOwnsClass(req.ClassID, teacher.ID) {
		c.JSON(http.StatusForbidden, gin.H{"message": "Giảng viên không được phân công lớp này"})
		return
	}

	dueDate, err := parseDateTime(req.DueDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Hạn nộp không hợp lệ. Dùng dạng 2026-07-10 23:59 hoặc RFC3339"})
		return
	}

	exercise := models.Exercise{
		ClassID:     req.ClassID,
		TeacherID:   teacher.ID,
		Title:       strings.TrimSpace(req.Title),
		Description: strings.TrimSpace(req.Description),
		Attachment:  strings.TrimSpace(req.Attachment),
		DueDate:     dueDate,
		Status:      "open",
	}

	if err := config.DB.Create(&exercise).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Tạo bài tập thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Tạo bài tập thành công", "data": exercise})
}

func ListTeacherExercises(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	var exercises []models.Exercise
	query := config.DB.Preload("Class").Where("teacher_id = ?", teacher.ID)

	if classID := strings.TrimSpace(c.Query("classId")); classID != "" {
		query = query.Where("class_id = ?", classID)
	}

	if err := query.Order("due_date desc").Find(&exercises).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách bài tập", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách bài tập thành công", "data": exercises})
}

func ListStudentExercises(c *gin.Context) {
	student, ok := getCurrentStudent(c)
	if !ok {
		return
	}

	var classIDs []uint
	if err := config.DB.Model(&models.Enrollment{}).
		Where("student_id = ?", student.ID).
		Distinct("class_id").
		Pluck("class_id", &classIDs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy lớp đã đăng ký", "error": err.Error()})
		return
	}

	if len(classIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "Sinh viên chưa có lớp học phần", "data": []models.Exercise{}})
		return
	}

	var exercises []models.Exercise
	if err := config.DB.Preload("Class").Preload("Teacher.User").
		Where("class_id IN ?", classIDs).
		Order("due_date desc").
		Find(&exercises).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy bài tập", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách bài tập thành công", "data": exercises})
}

func SubmitExercise(c *gin.Context) {
	student, ok := getCurrentStudent(c)
	if !ok {
		return
	}

	exerciseID, err := strconv.Atoi(c.Param("id"))
	if err != nil || exerciseID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID bài tập không hợp lệ"})
		return
	}

	var req SubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu bài nộp không hợp lệ", "error": err.Error()})
		return
	}

	if strings.TrimSpace(req.Content) == "" && strings.TrimSpace(req.FileURL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Cần nhập nội dung hoặc đường dẫn file bài làm"})
		return
	}

	var exercise models.Exercise
	if err := config.DB.First(&exercise, exerciseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy bài tập"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy bài tập", "error": err.Error()})
		return
	}

	var count int64
	if err := config.DB.Model(&models.Enrollment{}).
		Where("student_id = ? AND class_id = ?", student.ID, exercise.ClassID).
		Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi kiểm tra lớp học phần", "error": err.Error()})
		return
	}

	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"message": "Sinh viên không thuộc lớp học phần của bài tập này"})
		return
	}

	status := "submitted"
	if !exercise.DueDate.IsZero() && time.Now().After(exercise.DueDate) {
		status = "late"
	}

	var submission models.Submission
	err = config.DB.Where("exercise_id = ? AND student_id = ?", exercise.ID, student.ID).First(&submission).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi kiểm tra bài nộp", "error": err.Error()})
		return
	}

	submission.ExerciseID = exercise.ID
	submission.StudentID = student.ID
	submission.Content = strings.TrimSpace(req.Content)
	submission.FileURL = strings.TrimSpace(req.FileURL)
	submission.SubmittedAt = time.Now()
	submission.Status = status

	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = config.DB.Create(&submission).Error
	} else {
		err = config.DB.Save(&submission).Error
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Nộp bài thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Nộp bài thành công", "data": submission})
}

func ListMySubmissions(c *gin.Context) {
	student, ok := getCurrentStudent(c)
	if !ok {
		return
	}

	var submissions []models.Submission
	if err := config.DB.Preload("Exercise").Preload("Exercise.Class").
		Where("student_id = ?", student.ID).
		Order("submitted_at desc").
		Find(&submissions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy bài đã nộp", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách bài đã nộp thành công", "data": submissions})
}

func ListExerciseSubmissions(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	exerciseID, err := strconv.Atoi(c.Param("id"))
	if err != nil || exerciseID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID bài tập không hợp lệ"})
		return
	}

	var exercise models.Exercise
	if err := config.DB.First(&exercise, exerciseID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy bài tập"})
		return
	}

	if exercise.TeacherID != teacher.ID {
		c.JSON(http.StatusForbidden, gin.H{"message": "Giảng viên không được xem bài nộp của bài tập này"})
		return
	}

	var submissions []models.Submission
	if err := config.DB.Preload("Student.User").
		Where("exercise_id = ?", exercise.ID).
		Order("submitted_at desc").
		Find(&submissions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách bài nộp", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách bài nộp thành công", "data": submissions})
}
