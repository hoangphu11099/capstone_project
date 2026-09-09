package controllers

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"student-management/config"
	"student-management/middleware"
	"student-management/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TranscriptExportRow struct {
	StudentID       uint    `json:"studentId"`
	StudentCode     string  `json:"studentCode"`
	StudentName     string  `json:"studentName"`
	ClassCode       string  `json:"classCode"`
	CourseCode      string  `json:"courseCode"`
	CourseName      string  `json:"courseName"`
	Credits         int     `json:"credits"`
	AssignmentScore float64 `json:"assignmentScore"`
	MidtermScore    float64 `json:"midtermScore"`
	FinalScore      float64 `json:"finalScore"`
	TotalScore      float64 `json:"totalScore"`
	GradeLetter     string  `json:"gradeLetter"`
	Status          string  `json:"status"`
}

func ExportTranscript(c *gin.Context) {
	role := c.GetString(middleware.ContextRoleKey)
	studentCode := strings.TrimSpace(c.Query("studentCode"))
	studentIDParam := strings.TrimSpace(c.Query("studentId"))
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "json")))

	query := config.DB.Table("enrollments e").
		Select(`
			s.id AS student_id,
			s.student_code,
			u.full_name AS student_name,
			cl.class_code,
			c.code AS course_code,
			c.name AS course_name,
			c.credits,
			g.assignment_score,
			g.midterm_score,
			g.final_score,
			COALESCE(NULLIF(g.total_score, 0), g.score, 0) AS total_score,
			COALESCE(NULLIF(g.grade_letter, ''), g.letter_grade, '') AS grade_letter,
			COALESCE(NULLIF(g.status, ''), 'Draft') AS status
		`).
		Joins("JOIN students s ON s.id = e.student_id").
		Joins("JOIN users u ON u.id = s.user_id").
		Joins("JOIN classes cl ON cl.id = e.class_id").
		Joins("JOIN courses c ON c.id = e.course_id").
		Joins("LEFT JOIN grades g ON g.enrollment_id = e.id").
		Order("s.student_code, c.code")

	if studentCode != "" {
		query = query.Where("s.student_code = ?", studentCode)
	}

	if studentIDParam != "" {
		studentID, err := strconv.Atoi(studentIDParam)
		if err != nil || studentID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "studentId không hợp lệ"})
			return
		}
		query = query.Where("s.id = ?", studentID)
	}

	if role == "teacher" {
		teacher, ok := getCurrentTeacher(c)
		if !ok {
			return
		}
		query = query.Where("cl.teacher_id = ?", teacher.ID)
	}

	var rows []TranscriptExportRow
	if err := query.Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Xuất bảng điểm thất bại", "error": err.Error()})
		return
	}

	if format == "csv" {
		writeTranscriptCSV(c, rows)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Xuất bảng điểm thành công", "total": len(rows), "data": rows})
}

func writeTranscriptCSV(c *gin.Context, rows []TranscriptExportRow) {
	filename := "transcripts_" + time.Now().Format("20060102_150405") + ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+filename)

	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{
		"student_code", "student_name", "class_code", "course_code", "course_name", "credits",
		"assignment_score", "midterm_score", "final_score", "total_score", "grade_letter", "status",
	})

	for _, row := range rows {
		_ = writer.Write([]string{
			row.StudentCode,
			row.StudentName,
			row.ClassCode,
			row.CourseCode,
			row.CourseName,
			strconv.Itoa(row.Credits),
			formatFloat(row.AssignmentScore),
			formatFloat(row.MidtermScore),
			formatFloat(row.FinalScore),
			formatFloat(row.TotalScore),
			row.GradeLetter,
			row.Status,
		})
	}
	writer.Flush()
}

func rebuildApprovedTranscripts(c *gin.Context) {
	var grades []models.Grade
	if err := config.DB.Preload("Enrollment.Course").Preload("Enrollment.Class").Where("status = ?", "Approved").Find(&grades).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy điểm đã duyệt", "error": err.Error()})
		return
	}

	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		for i := range grades {
			if err := upsertTranscriptFromGrade(tx, &grades[i]); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Tạo lại transcript thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tạo lại transcript thành công", "total": len(grades)})
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
