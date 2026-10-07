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
	"gorm.io/gorm/clause"
)

type ExerciseRequest struct {
	CourseOfferingID uint   `json:"courseOfferingId"`
	ClassID          uint   `json:"classId"`
	Title            string `json:"title" binding:"required"`
	Description      string `json:"description"`
	Attachment       string `json:"attachment"`
	DueDate          string `json:"dueDate" binding:"required"`
}

type SubmissionRequest struct {
	Content string `json:"content" form:"content"`
	FileURL string `json:"fileUrl" form:"fileUrl"`
}

type GradeSubmissionRequest struct {
	Score       *float64 `json:"score" binding:"required"`
	SubmittedAt string   `json:"submittedAt"`
	Feedback    string   `json:"feedback"`
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

	var offering models.CourseOffering
	if req.CourseOfferingID == 0 {
		c.JSON(400, gin.H{"message": "Vui lòng chọn học phần cho bài tập"})
		return
	}
	if err := config.DB.Preload("Semester").Where("id = ? AND teacher_id = ? AND status = ?", req.CourseOfferingID, teacher.ID, "open").First(&offering).Error; err != nil {
		c.JSON(403, gin.H{"message": "Không được phân công học phần này"})
		return
	}
	if offering.Semester.Status == "closed" {
		c.JSON(400, gin.H{"message": "Học kỳ đã đóng"})
		return
	}
	req.ClassID = offering.ClassID

	dueDate, err := parseExerciseDeadline(req.DueDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Hạn nộp không hợp lệ. Dùng dạng 2026-07-10 23:59 hoặc RFC3339"})
		return
	}

	if strings.TrimSpace(req.Title) == "" || !dueDate.After(attendanceNow()) {
		c.JSON(400, gin.H{"message": "Tiêu đề không được để trống và hạn nộp phải ở tương lai"})
		return
	}
	exercise := models.Exercise{
		CourseOfferingID: &offering.ID,
		ClassID:          req.ClassID,
		TeacherID:        teacher.ID,
		Title:            strings.TrimSpace(req.Title),
		Description:      strings.TrimSpace(req.Description),
		Attachment:       strings.TrimSpace(req.Attachment),
		DueDate:          dueDate,
		Status:           "open",
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
	query := config.DB.Preload("Class").Preload("CourseOffering.Course").Preload("CourseOffering.Semester").Where("teacher_id = ?", teacher.ID)

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
		Where("student_id = ? AND status <> ?", student.ID, "cancelled").
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
		Where("(course_offering_id IS NULL AND class_id IN ?) OR course_offering_id IN (SELECT course_offering_id FROM enrollments WHERE student_id = ? AND status <> ?)", classIDs, student.ID, "cancelled").
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

	var submission models.Submission
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		var exercise models.Exercise
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&exercise, exerciseID).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.Enrollment{}).Where("student_id = ? AND class_id = ? AND status <> ?", student.ID, exercise.ClassID, "cancelled").Where("(? IS NULL OR course_offering_id = ?)", exercise.CourseOfferingID, exercise.CourseOfferingID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errExerciseForbidden
		}
		if !exerciseAcceptsSubmission(exercise, attendanceNow()) {
			return errExerciseLocked
		}
		lookup := tx.Where("exercise_id = ? AND student_id = ?", exercise.ID, student.ID).First(&submission).Error
		if lookup != nil && !errors.Is(lookup, gorm.ErrRecordNotFound) {
			return lookup
		}
		submission.Score = nil
		submission.ExerciseID = exercise.ID
		submission.StudentID = student.ID
		submission.Content = strings.TrimSpace(req.Content)
		submission.FileURL = strings.TrimSpace(req.FileURL)
		submission.SubmittedAt = attendanceNow()
		submission.Status = "submitted"
		submission.Feedback = ""
		return tx.Save(&submission).Error
	})
	if err != nil {
		exerciseActionError(c, err)
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

func GradeSubmission(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	submissionID, err := strconv.Atoi(c.Param("id"))
	if err != nil || submissionID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID bài nộp không hợp lệ"})
		return
	}

	var req GradeSubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu chấm bài không hợp lệ", "error": err.Error()})
		return
	}
	if *req.Score < 0 || *req.Score > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Điểm bài tập phải nằm trong khoảng 0 đến 100"})
		return
	}

	var submission models.Submission
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&submission, submissionID).Error; err != nil {
			return err
		}
		var exercise models.Exercise
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&exercise, submission.ExerciseID).Error; err != nil {
			return err
		}
		if exercise.TeacherID != teacher.ID {
			return errExerciseForbidden
		}
		if err := tx.First(&submission, submissionID).Error; err != nil {
			return err
		}
		if req.SubmittedAt != "" {
			revision, err := time.Parse(time.RFC3339Nano, req.SubmittedAt)
			if err != nil || !revision.Equal(submission.SubmittedAt) {
				return errors.New("Sinh viên vừa nộp lại bài. Hãy tải lại bài nộp trước khi chấm")
			}
		}
		// Update only grading fields; never write an old copy over a student's content.
		score := *req.Score
		return tx.Model(&models.Submission{}).Where("id = ?", submission.ID).Updates(map[string]interface{}{"score": score, "feedback": strings.TrimSpace(req.Feedback), "status": "graded"}).Error
	})
	if err != nil {
		exerciseActionError(c, err)
		return
	}
	config.DB.First(&submission, submissionID)

	c.JSON(http.StatusOK, gin.H{"message": "Đã lưu điểm bài nộp", "data": submission})
}

var errExerciseForbidden = errors.New("Bạn không có quyền thực hiện thao tác với bài tập này")
var errExerciseLocked = errors.New("Bài tập đã khóa hoặc hết hạn nộp. Cần giảng viên mở lại với hạn nộp mới")

func exerciseAcceptsSubmission(exercise models.Exercise, now time.Time) bool {
	return exercise.Status == "open" && !exercise.DueDate.IsZero() && now.Before(exercise.DueDate)
}

// Unzoned input is school local time, independent of the server timezone.
func parseExerciseDeadline(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if date, err := time.Parse(time.RFC3339, value); err == nil {
		return date, nil
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if date, err := time.ParseInLocation(layout, value, attendanceLocation()); err == nil {
			return date, nil
		}
	}
	return time.Time{}, errors.New("Hạn nộp không hợp lệ")
}

type ExerciseAvailabilityRequest struct {
	Status  string `json:"status"`
	DueDate string `json:"dueDate"`
}

func UpdateExerciseAvailability(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"message": "ID bài tập không hợp lệ"})
		return
	}
	var req ExerciseAvailabilityRequest
	if c.ShouldBindJSON(&req) != nil || (req.Status != "open" && req.Status != "closed") {
		c.JSON(400, gin.H{"message": "Trạng thái phải là open hoặc closed"})
		return
	}
	var exercise models.Exercise
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&exercise, id).Error; err != nil {
			return err
		}
		if exercise.TeacherID != teacher.ID {
			return errExerciseForbidden
		}
		if req.Status == "open" {
			deadline, err := parseExerciseDeadline(req.DueDate)
			if err != nil || !deadline.After(attendanceNow()) {
				return errors.New("Chọn hạn nộp mới ở tương lai để mở lại bài tập")
			}
			exercise.DueDate = deadline
		}
		exercise.Status = req.Status
		return tx.Model(&models.Exercise{}).Where("id = ?", id).Updates(map[string]interface{}{"status": exercise.Status, "due_date": exercise.DueDate}).Error
	})
	if err != nil {
		exerciseActionError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Đã cập nhật thời gian nhận bài", "data": exercise})
}

func exerciseActionError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, gorm.ErrRecordNotFound) {
		status = http.StatusNotFound
	}
	if errors.Is(err, errExerciseForbidden) {
		status = http.StatusForbidden
	}
	if errors.Is(err, errExerciseLocked) {
		status = http.StatusConflict
	}
	c.JSON(status, gin.H{"message": err.Error()})
}
