package controllers

import (
	"math"
	"net/http"

	"student-management/config"
	"student-management/middleware"
	"student-management/models"
	"student-management/responses"

	"github.com/gin-gonic/gin"
)

func ViewTranscript(c *gin.Context) {
	userID, exists := c.Get(middleware.ContextUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Vui lòng đăng nhập",
		})
		return
	}

	var student models.Student
	if err := config.DB.Where("user_id = ?", userID).First(&student).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "Không phải sinh viên",
		})
		return
	}

	var transcript []responses.TranscriptItem
	err := config.DB.Table("enrollments e").
		Select(`
			c.code AS course_code,
			c.name AS course_name,
			c.credits,
			g.assignment_score,
			g.midterm_score,
			g.final_score,
			COALESCE(NULLIF(g.total_score, 0), g.score, 0) AS total_score,
			COALESCE(NULLIF(g.grade_letter, ''), g.letter_grade, '') AS grade_letter
		`).
		Joins("JOIN courses c ON e.course_id = c.id").
		Joins("JOIN grades g ON g.enrollment_id = e.id").
		Where("e.student_id = ?", student.ID).
		Where("(g.status = ? OR g.status IS NULL OR g.status = '')", "Approved").
		Order("c.code").
		Scan(&transcript).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Lỗi truy vấn dữ liệu",
			"error":   err.Error(),
		})
		return
	}

	if len(transcript) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Chưa có điểm được công bố",
		})
		return
	}

	var totalCredits float64
	var totalPoints float64

	for _, item := range transcript {
		point := gradePoint(item.GradeLetter)
		totalCredits += float64(item.Credits)
		totalPoints += point * float64(item.Credits)
	}

	gpa := 0.0
	if totalCredits > 0 {
		gpa = math.Round((totalPoints/totalCredits)*100) / 100
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Lấy bảng điểm thành công",
		"gpa":         gpa,
		"totalCredit": int(totalCredits),
		"transcript":  transcript,
	})
}

func gradePoint(letter string) float64 {
	switch letter {
	case "A":
		return 4.0
	case "B+":
		return 3.5
	case "B":
		return 3.0
	case "C+":
		return 2.5
	case "C":
		return 2.0
	case "D+":
		return 1.5
	case "D":
		return 1.0
	default:
		return 0
	}
}
