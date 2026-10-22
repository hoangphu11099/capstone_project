package controllers

import (
	"errors"
	"fmt"
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

	normalizeExamSession(&req)
	if err := validateExamOffering(req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
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

	if hasClassExam(0, req.ClassID, req.CourseID, req.SemesterID) {
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

	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&exam).Error; err != nil {
			return err
		}
		return notifyExamClass(tx, exam)
	}); err != nil {
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

	normalizeExamSession(&req)
	if err := validateExamOffering(req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
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

	if hasClassExam(uint(id), req.ClassID, req.CourseID, req.SemesterID) {
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

	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&exam).Error; err != nil {
			return err
		}
		return notifyExamClass(tx, exam)
	}); err != nil {
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

func hasClassExam(currentID uint, classID uint, courseID uint, semesterID uint) bool {
	query := config.DB.Model(&models.ExamSchedule{}).Where("class_id = ? AND course_id = ? AND semester_id = ?", classID, courseID, semesterID)
	if currentID > 0 {
		query = query.Where("id <> ?", currentID)
	}
	var count int64
	query.Count(&count)
	return count > 0
}

func validateExamOffering(req ExamScheduleRequest) error {
	if req.Session != "Sáng" && req.Session != "Chiều" && req.Session != "Tối" {
		return errors.New("Ca thi chỉ gồm Sáng, Trưa / Chiều, Tối")
	}
	var offering models.CourseOffering
	if err := config.DB.Preload("Semester").Where("class_id = ? AND course_id = ? AND semester_id = ?", req.ClassID, req.CourseID, req.SemesterID).First(&offering).Error; err != nil {
		return errors.New("Môn chưa được mở cho lớp trong học kỳ đã chọn")
	}
	date, err := parseDateTime(req.ExamDate)
	if err != nil {
		return errors.New("Ngày thi không hợp lệ")
	}
	day := date.Format("2006-01-02")
	if offering.Semester.Status == "closed" || day < offering.Semester.StartDate.Format("2006-01-02") || day > offering.Semester.EndDate.Format("2006-01-02") {
		return errors.New("Ngày thi phải thuộc học kỳ chưa đóng")
	}
	if timeToMinutes(req.StartTime) < 0 || timeToMinutes(req.EndTime) <= timeToMinutes(req.StartTime) {
		return errors.New("Giờ thi không hợp lệ")
	}
	return nil
}

func normalizeExamSession(req *ExamScheduleRequest) {
	switch strings.TrimSpace(req.Session) {
	case "AM", "Sáng":
		req.Session = "Sáng"
		req.StartTime = "07:30"
		req.EndTime = "11:30"
	case "PM", "Trưa", "Chiều":
		req.Session = "Chiều"
		req.StartTime = "13:30"
		req.EndTime = "16:30"
	case "Tối":
		req.Session = "Tối"
		req.StartTime = "18:00"
		req.EndTime = "21:00"
	}
	if strings.TrimSpace(req.ExamType) == "" {
		req.ExamType = "Trực tiếp"
	}
}

func notifyExamClass(tx *gorm.DB, exam models.ExamSchedule) error {
	var offering models.CourseOffering
	if err := tx.Preload("Course").Preload("Class").Preload("Teacher").Where("class_id = ? AND course_id = ? AND semester_id = ?", exam.ClassID, exam.CourseID, exam.SemesterID).First(&offering).Error; err != nil {
		return err
	}
	var students []models.Student
	if err := tx.Joins("JOIN enrollments e ON e.student_id = students.id").Where("e.class_id = ? AND e.course_id = ? AND e.status <> ? AND (e.course_offering_id = ? OR e.course_offering_id IS NULL)", exam.ClassID, exam.CourseID, "cancelled", offering.ID).Distinct("students.*").Find(&students).Error; err != nil {
		return err
	}
	users := map[uint]bool{offering.Teacher.UserID: true}
	for _, student := range students {
		users[student.UserID] = true
	}
	for userID := range users {
		if userID == 0 {
			continue
		}
		recipient := userID
		notification := models.Notification{RecipientUserID: &recipient, Subject: "Lịch thi · " + offering.Course.Name, Content: fmt.Sprintf("Lớp %s · %s: thi ngày %s, ca %s (%s–%s), hình thức %s. %s", offering.Class.ClassCode, offering.Course.Name, exam.ExamDate.Format("02/01/2006"), exam.Session, exam.StartTime, exam.EndTime, exam.ExamType, exam.Note), TargetType: "user", Channel: "app", Status: "created"}
		if err := tx.Create(&notification).Error; err != nil {
			return err
		}
	}
	return nil
}
