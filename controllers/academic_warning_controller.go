package controllers

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AcademicWarningRequest struct {
	SemesterID       uint    `json:"semesterId" binding:"required"`
	MinGPA           float64 `json:"minGpa"`
	MaxFailedCourses int     `json:"maxFailedCourses"`
}

func ListAcademicWarnings(c *gin.Context) {
	var warnings []models.AcademicWarning
	query := config.DB.Preload("Student.User").Preload("Semester")

	if semesterID := strings.TrimSpace(c.Query("semesterId")); semesterID != "" {
		query = query.Where("semester_id = ?", semesterID)
	}
	if studentID := strings.TrimSpace(c.Query("studentId")); studentID != "" {
		query = query.Where("student_id = ?", studentID)
	}

	if err := query.Order("created_at desc").Find(&warnings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách cảnh báo học vụ", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách cảnh báo học vụ thành công", "data": warnings})
}

func GenerateAcademicWarnings(c *gin.Context) {
	var req AcademicWarningRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu cảnh báo không hợp lệ", "error": err.Error()})
		return
	}

	minGPA := req.MinGPA
	if minGPA <= 0 {
		minGPA = 2.0
	}

	maxFailedCourses := req.MaxFailedCourses
	if maxFailedCourses <= 0 {
		maxFailedCourses = 2
	}

	var semester models.Semester
	if err := config.DB.First(&semester, req.SemesterID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy học kỳ"})
		return
	}

	var students []models.Student
	if err := config.DB.Find(&students).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách sinh viên", "error": err.Error()})
		return
	}

	createdWarnings := make([]models.AcademicWarning, 0)

	err := config.DB.Transaction(func(tx *gorm.DB) error {
		for _, student := range students {
			var enrollments []models.Enrollment
			if err := tx.Preload("Course").Preload("Grade").
				Joins("JOIN courses ON courses.id = enrollments.course_id").
				Where("enrollments.student_id = ? AND courses.semester_id = ?", student.ID, req.SemesterID).
				Find(&enrollments).Error; err != nil {
				return err
			}

			if len(enrollments) == 0 {
				continue
			}

			totalCredits := 0
			totalPoints := 0.0
			failedCredits := 0
			failedCourses := 0
			approvedGradeCount := 0

			for _, enrollment := range enrollments {
				if enrollment.Grade == nil || enrollment.Grade.Status != "Approved" {
					continue
				}

				approvedGradeCount++
				credits := enrollment.Course.Credits
				letter := enrollment.Grade.GradeLetter
				if letter == "" {
					letter = enrollment.Grade.LetterGrade
				}
				point := gradePoint(letter)
				totalCredits += credits
				totalPoints += point * float64(credits)

				if point == 0 || enrollment.Grade.TotalScore < 40 {
					failedCourses++
					failedCredits += credits
				}
			}

			if approvedGradeCount == 0 || totalCredits == 0 {
				continue
			}

			gpa := math.Round((totalPoints/float64(totalCredits))*100) / 100
			if gpa >= minGPA && failedCourses < maxFailedCourses {
				continue
			}

			reasonParts := []string{}
			if gpa < minGPA {
				reasonParts = append(reasonParts, "GPA thấp hơn mức quy định")
			}
			if failedCourses >= maxFailedCourses {
				reasonParts = append(reasonParts, "Số môn không đạt vượt mức quy định")
			}

			warning := models.AcademicWarning{
				StudentID:     student.ID,
				SemesterID:    req.SemesterID,
				GPA:           gpa,
				FailedCourses: failedCourses,
				FailedCredits: failedCredits,
				Reason:        strings.Join(reasonParts, "; "),
				Status:        "active",
			}

			var existing models.AcademicWarning
			err := tx.Where("student_id = ? AND semester_id = ?", student.ID, req.SemesterID).First(&existing).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&warning).Error; err != nil {
					return err
				}
			} else {
				existing.GPA = warning.GPA
				existing.FailedCourses = warning.FailedCourses
				existing.FailedCredits = warning.FailedCredits
				existing.Reason = warning.Reason
				existing.Status = "active"
				if err := tx.Save(&existing).Error; err != nil {
					return err
				}
				warning.ID = existing.ID
			}

			createdWarnings = append(createdWarnings, warning)
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Tạo cảnh báo học vụ thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Tạo cảnh báo học vụ thành công",
		"semesterId": semester.ID,
		"count":      len(createdWarnings),
		"data":       createdWarnings,
	})
}

func DeleteAcademicWarning(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID cảnh báo không hợp lệ"})
		return
	}

	if err := config.DB.Delete(&models.AcademicWarning{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Xóa cảnh báo thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Xóa cảnh báo thành công"})
}
