package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AddStudentsToClassRequest supports both the camelCase payload used by the
// web client and the snake_case payload used by the first API draft.
type AddStudentsToClassRequest struct {
	StudentIDs       []uint `json:"studentIds"`
	LegacyStudentIDs []uint `json:"student_ids"`
}

// AddStudentsToClass assigns one or more existing students to a class.
//
// POST /classes/:id/students
// Body: {"studentIds": [1, 2, 3]}
func AddStudentsToClass(c *gin.Context) {
	classID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || classID64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID lớp không hợp lệ"})
		return
	}
	classID := uint(classID64)

	var req AddStudentsToClassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Danh sách sinh viên không hợp lệ", "error": err.Error()})
		return
	}

	studentIDs := req.StudentIDs
	if len(studentIDs) == 0 {
		studentIDs = req.LegacyStudentIDs
	}
	studentIDs = uniquePositiveIDs(studentIDs)
	if len(studentIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Vui lòng chọn ít nhất một sinh viên"})
		return
	}

	var class models.Class
	var students []models.Student
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&class, classID).Error; err != nil {
			return err
		}

		if err := tx.Where("id IN ?", studentIDs).Find(&students).Error; err != nil {
			return err
		}
		if len(students) != len(studentIDs) {
			return errStudentsNotFound
		}

		if class.MaxStudents > 0 {
			var currentStudents int64
			if err := tx.Model(&models.Student{}).
				Where("class_id = ?", class.ID).
				Count(&currentStudents).Error; err != nil {
				return err
			}

			newStudents := 0
			for _, student := range students {
				if student.ClassID != class.ID {
					newStudents++
				}
			}
			if currentStudents+int64(newStudents) > int64(class.MaxStudents) {
				return errClassCapacityExceeded
			}
		}

		if err := tx.Model(&models.Student{}).
			Where("id IN ?", studentIDs).
			Update("class_id", class.ID).Error; err != nil {
			return err
		}
		for _, student := range students {
			if err := syncStudentClassEnrollments(tx, student.ID, class.ID); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy lớp"})
		case errors.Is(err, errStudentsNotFound):
			c.JSON(http.StatusNotFound, gin.H{"message": "Một hoặc nhiều sinh viên không tồn tại"})
		case errors.Is(err, errClassCapacityExceeded):
			c.JSON(http.StatusBadRequest, gin.H{"message": "Số sinh viên vượt quá sĩ số tối đa của lớp"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Thêm sinh viên vào lớp thất bại", "error": err.Error()})
		}
		return
	}

	students = nil
	if err := config.DB.Preload("User").Where("id IN ?", studentIDs).Find(&students).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Đã thêm sinh viên nhưng không đọc lại được dữ liệu", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Đã xếp sinh viên vào lớp và đồng bộ các học phần",
		"data": gin.H{
			"classId":      class.ID,
			"updatedCount": len(students),
			"students":     students,
		},
	})
}

var (
	errStudentsNotFound      = errors.New("one or more students do not exist")
	errClassCapacityExceeded = errors.New("class capacity exceeded")
)

func uniquePositiveIDs(ids []uint) []uint {
	result := make([]uint, 0, len(ids))
	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
