package controllers

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GradeRequest struct {
	EnrollmentID    uint    `json:"enrollmentId"`
	AssignmentScore float64 `json:"assignmentScore"`
	MidtermScore    float64 `json:"midtermScore"`
	FinalScore      float64 `json:"finalScore"`
	Remark          string  `json:"remark"`
}

func ListTeacherClasses(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	var classes []models.Class
	if err := config.DB.Preload("Major").Preload("Room").Preload("Semester").
		Where("teacher_id = ?", teacher.ID).
		Order("class_code").
		Find(&classes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy lớp giảng dạy", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách lớp giảng dạy thành công", "data": classes})
}

func ListClassGrades(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	classID, err := strconv.Atoi(c.Param("classID"))
	if err != nil || classID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID lớp không hợp lệ"})
		return
	}

	if !teacherOwnsClass(uint(classID), teacher.ID) {
		c.JSON(http.StatusForbidden, gin.H{"message": "Giảng viên không được phân công lớp này"})
		return
	}

	var enrollments []models.Enrollment
	if err := config.DB.Preload("Student.User").Preload("Course").Preload("Grade").
		Where("class_id = ?", classID).
		Order("student_id, course_id").
		Find(&enrollments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách điểm", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách điểm thành công", "data": enrollments})
}

func UpsertGrade(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	var req GradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu điểm không hợp lệ", "error": err.Error()})
		return
	}

	if req.EnrollmentID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Vui lòng chọn đăng ký học phần"})
		return
	}

	if !validScore(req.AssignmentScore) || !validScore(req.MidtermScore) || !validScore(req.FinalScore) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Điểm phải nằm trong khoảng 0 đến 100"})
		return
	}

	var enrollment models.Enrollment
	if err := config.DB.Preload("Class").First(&enrollment, req.EnrollmentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy đăng ký học phần"})
		return
	}

	if enrollment.Class.TeacherID != teacher.ID {
		c.JSON(http.StatusForbidden, gin.H{"message": "Giảng viên không được nhập điểm cho lớp này"})
		return
	}

	totalScore := roundGrade(req.AssignmentScore*0.3 + req.MidtermScore*0.3 + req.FinalScore*0.4)
	letter := letterGrade(totalScore)

	var grade models.Grade
	err := config.DB.Where("enrollment_id = ?", req.EnrollmentID).First(&grade).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy điểm", "error": err.Error()})
		return
	}

	grade.EnrollmentID = req.EnrollmentID
	grade.AssignmentScore = req.AssignmentScore
	grade.MidtermScore = req.MidtermScore
	grade.FinalScore = req.FinalScore
	grade.TotalScore = totalScore
	grade.GradeLetter = letter
	grade.Score = totalScore
	grade.LetterGrade = letter
	grade.Remark = strings.TrimSpace(req.Remark)
	grade.Status = "Draft"

	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = config.DB.Create(&grade).Error
	} else {
		err = config.DB.Save(&grade).Error
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lưu điểm thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lưu điểm thành công", "data": grade})
}

func UpdateGrade(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	gradeID, err := strconv.Atoi(c.Param("id"))
	if err != nil || gradeID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID điểm không hợp lệ"})
		return
	}

	var req GradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu điểm không hợp lệ", "error": err.Error()})
		return
	}

	if !validScore(req.AssignmentScore) || !validScore(req.MidtermScore) || !validScore(req.FinalScore) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Điểm phải nằm trong khoảng 0 đến 100"})
		return
	}

	var grade models.Grade
	if err := config.DB.Preload("Enrollment.Class").First(&grade, gradeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy điểm"})
		return
	}

	if grade.Enrollment.Class.TeacherID != teacher.ID {
		c.JSON(http.StatusForbidden, gin.H{"message": "Giảng viên không được sửa điểm của lớp này"})
		return
	}

	if grade.Status == "Approved" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Điểm đã duyệt nên không thể sửa"})
		return
	}

	totalScore := roundGrade(req.AssignmentScore*0.3 + req.MidtermScore*0.3 + req.FinalScore*0.4)
	letter := letterGrade(totalScore)

	grade.AssignmentScore = req.AssignmentScore
	grade.MidtermScore = req.MidtermScore
	grade.FinalScore = req.FinalScore
	grade.TotalScore = totalScore
	grade.GradeLetter = letter
	grade.Score = totalScore
	grade.LetterGrade = letter
	grade.Remark = strings.TrimSpace(req.Remark)
	grade.Status = "Draft"

	if err := config.DB.Save(&grade).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Cập nhật điểm thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cập nhật điểm thành công", "data": grade})
}

func ApproveGrade(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	gradeID, err := strconv.Atoi(c.Param("id"))
	if err != nil || gradeID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID điểm không hợp lệ"})
		return
	}

	var grade models.Grade
	if err := config.DB.Preload("Enrollment.Class").First(&grade, gradeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy điểm"})
		return
	}

	if grade.Enrollment.Class.TeacherID != teacher.ID {
		c.JSON(http.StatusForbidden, gin.H{"message": "Giảng viên không được duyệt điểm của lớp này"})
		return
	}

	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		grade.Status = "Approved"
		if err := tx.Save(&grade).Error; err != nil {
			return err
		}

		if err := upsertTranscriptFromGrade(tx, &grade); err != nil {
			return err
		}

		approval := models.GradeApproval{
			GradeID:    grade.ID,
			TeacherID:  teacher.ID,
			Status:     "approved",
			Note:       "Duyệt điểm",
			ApprovedAt: time.Now(),
		}
		return tx.Create(&approval).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Duyệt điểm thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Duyệt điểm thành công", "data": grade})
}

func ApproveClassGrades(c *gin.Context) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	classID, err := strconv.Atoi(c.Param("classID"))
	if err != nil || classID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID lớp không hợp lệ"})
		return
	}

	if !teacherOwnsClass(uint(classID), teacher.ID) {
		c.JSON(http.StatusForbidden, gin.H{"message": "Giảng viên không được phân công lớp này"})
		return
	}

	var enrollments []models.Enrollment
	if err := config.DB.Preload("Grade").Where("class_id = ?", classID).Find(&enrollments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách điểm", "error": err.Error()})
		return
	}

	if len(enrollments) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Lớp chưa có sinh viên đăng ký"})
		return
	}

	for _, enrollment := range enrollments {
		if enrollment.Grade == nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Còn sinh viên chưa được nhập điểm đầy đủ"})
			return
		}
	}

	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		for _, enrollment := range enrollments {
			grade := *enrollment.Grade
			grade.Enrollment = enrollment
			grade.Status = "Approved"
			if err := tx.Save(&grade).Error; err != nil {
				return err
			}
			if err := upsertTranscriptFromGrade(tx, &grade); err != nil {
				return err
			}

			approval := models.GradeApproval{
				GradeID:    grade.ID,
				TeacherID:  teacher.ID,
				Status:     "approved",
				Note:       "Duyệt điểm toàn lớp",
				ApprovedAt: time.Now(),
			}
			if err := tx.Create(&approval).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Duyệt điểm lớp thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Duyệt điểm toàn lớp thành công"})
}

func SubmitGrade(c *gin.Context) {
	UpsertGrade(c)
}

func upsertTranscriptFromGrade(tx *gorm.DB, grade *models.Grade) error {
	var enrollment models.Enrollment
	if grade.Enrollment.ID != 0 {
		enrollment = grade.Enrollment
	}
	if enrollment.ID == 0 || enrollment.Course.ID == 0 || enrollment.Class.ID == 0 {
		if err := tx.Preload("Course").Preload("Class").First(&enrollment, grade.EnrollmentID).Error; err != nil {
			return err
		}
	}

	semesterID := enrollment.Course.SemesterID
	if semesterID == 0 {
		semesterID = enrollment.Class.SemesterID
	}

	transcript := models.Transcript{
		StudentID:       enrollment.StudentID,
		CourseID:        enrollment.CourseID,
		EnrollmentID:    enrollment.ID,
		GradeID:         grade.ID,
		SemesterID:      semesterID,
		AssignmentScore: grade.AssignmentScore,
		MidtermScore:    grade.MidtermScore,
		FinalScore:      grade.FinalScore,
		TotalScore:      grade.TotalScore,
		GradeLetter:     grade.GradeLetter,
		Status:          "Approved",
	}

	return tx.Where("enrollment_id = ?", enrollment.ID).Assign(transcript).FirstOrCreate(&transcript).Error
}

func teacherOwnsClass(classID uint, teacherID uint) bool {
	var count int64
	config.DB.Model(&models.Class{}).Where("id = ? AND teacher_id = ?", classID, teacherID).Count(&count)
	return count > 0
}

func validScore(score float64) bool {
	return score >= 0 && score <= 100
}

func roundGrade(score float64) float64 {
	return math.Round(score*100) / 100
}

func letterGrade(score float64) string {
	if score >= 85 {
		return "A"
	}
	if score >= 80 {
		return "B+"
	}
	if score >= 70 {
		return "B"
	}
	if score >= 65 {
		return "C+"
	}
	if score >= 55 {
		return "C"
	}
	if score >= 50 {
		return "D+"
	}
	if score >= 40 {
		return "D"
	}
	return "F"
}
