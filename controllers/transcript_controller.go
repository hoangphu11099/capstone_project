package controllers

import (
	"math"
	"net/http"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
)

func ViewTranscript(c *gin.Context) {

	userId, exists := c.Get("userId")

	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Vui lòng đăng nhập",
		})
		return
	}

	var student models.Student

	err := config.DB.
		Where("user_id = ?", userId).
		First(&student).Error

	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "Không phải sinh viên",
		})
		return
	}

	var transcript []models.TranscriptResponse

	err = config.DB.Table("grades g").
		Select(`
        c.course_code,
        c.course_name,
        c.credits,
        g.assignment_score,
        g.midterm_score,
        g.final_score,
        g.total_score,
        g.grade_letter
    `).
		Joins("JOIN courses c ON g.course_id = c.course_id").
		Where("g.student_id = ?", student.StudentCode).
		Where("g.status = ?", "Approved").
		Scan(&transcript).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Lỗi truy vấn dữ liệu",
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

		point := 0.0

		switch item.GradeLetter {
		case "A":
			point = 4.0
		case "B+":
			point = 3.5
		case "B":
			point = 3.0
		case "C+":
			point = 2.5
		case "C":
			point = 2.0
		case "D+":
			point = 1.5
		case "D":
			point = 1.0
		default:
			point = 0
		}

		totalCredits += float64(item.Credits)
		totalPoints += point * float64(item.Credits)
	}

	gpa := math.Round((totalPoints/totalCredits)*100) / 100

	c.JSON(http.StatusOK, gin.H{
		"gpa":        gpa,
		"transcript": transcript,
	})

}
