package controllers

import (
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"strconv"
	"strings"
	"student-management/config"
	"student-management/models"
	"time"
)

type PeriodRequest struct {
	Name           string `json:"name" binding:"required"`
	StartDate      string `json:"startDate" binding:"required"`
	EndDate        string `json:"endDate" binding:"required"`
	AcademicYearID uint   `json:"academicYearId"`
	Status         string `json:"status"`
}

func periodDates(req PeriodRequest) (time.Time, time.Time, error) {
	start, e1 := time.ParseInLocation("2006-01-02", req.StartDate, attendanceLocation())
	end, e2 := time.ParseInLocation("2006-01-02", req.EndDate, attendanceLocation())
	if strings.TrimSpace(req.Name) == "" || e1 != nil || e2 != nil || end.Before(start) {
		return start, end, errors.New("Tên và khoảng ngày không hợp lệ; ngày kết thúc phải từ ngày bắt đầu trở đi")
	}
	return start, end, nil
}
func ListAcademicYears(c *gin.Context) {
	var rows []models.AcademicYear
	if err := config.DB.Order("start_date desc").Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"data": rows})
}
func SaveAcademicYear(c *gin.Context) {
	var req PeriodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "Vui lòng nhập tên và ngày bắt đầu/kết thúc"})
		return
	}
	start, end, err := periodDates(req)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	var row models.AcademicYear
	if c.Param("id") != "" {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 || config.DB.First(&row, id).Error != nil {
			c.JSON(404, gin.H{"message": "Không tìm thấy năm học"})
			return
		}
		var count int64
		if err := config.DB.Model(&models.Semester{}).Where("academic_year_id = ? AND (DATE(start_date) < ? OR DATE(end_date) > ?)", row.ID, req.StartDate, req.EndDate).Count(&count).Error; err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		if count > 0 {
			c.JSON(400, gin.H{"message": "Khoảng năm học phải bao gồm các học kỳ đã tạo"})
			return
		}
	}
	row.Name = strings.TrimSpace(req.Name)
	row.StartDate = start
	row.EndDate = end
	if err := config.DB.Save(&row).Error; err != nil {
		c.JSON(400, gin.H{"message": "Không lưu được năm học; kiểm tra tên trùng", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã lưu năm học", "data": row})
}
func ListSemesters(c *gin.Context) {
	var rows []models.Semester
	if err := config.DB.Preload("AcademicYear").Order("start_date desc").Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"data": rows})
}
func SaveSemester(c *gin.Context) {
	var req PeriodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "Vui lòng nhập đủ thông tin học kỳ"})
		return
	}
	start, end, err := periodDates(req)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	if req.AcademicYearID == 0 {
		c.JSON(400, gin.H{"message": "Vui lòng chọn năm học"})
		return
	}
	if req.Status == "" {
		req.Status = "planned"
	}
	if req.Status != "planned" && req.Status != "active" && req.Status != "closed" {
		c.JSON(400, gin.H{"message": "Trạng thái phải là planned, active hoặc closed"})
		return
	}
	var row models.Semester
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		var year models.AcademicYear
		if err := tx.First(&year, req.AcademicYearID).Error; err != nil {
			return err
		}
		if req.StartDate < year.StartDate.Format("2006-01-02") || req.EndDate > year.EndDate.Format("2006-01-02") {
			return errors.New("Ngày học kỳ phải nằm trong năm học")
		}
		if c.Param("id") != "" {
			id, err := strconv.ParseUint(c.Param("id"), 10, 64)
			if err != nil || id == 0 {
				return errors.New("ID học kỳ không hợp lệ")
			}
			if err := tx.First(&row, id).Error; err != nil {
				return err
			}
			var count int64
			if err := tx.Model(&models.Schedule{}).Joins("JOIN course_offerings co ON co.id = schedules.course_offering_id").Where("co.semester_id = ? AND (full_date < ? OR full_date > ?)", row.ID, req.StartDate, req.EndDate).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return errors.New("Không thể thu hẹp học kỳ vì có ngày dạy nằm ngoài khoảng mới")
			}
			if err := tx.Model(&models.ExamSchedule{}).Where("semester_id = ? AND (DATE(exam_date) < ? OR DATE(exam_date) > ?)", row.ID, req.StartDate, req.EndDate).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return errors.New("Không thể thu hẹp học kỳ vì có lịch thi nằm ngoài khoảng mới")
			}
		}
		row.Name = strings.TrimSpace(req.Name)
		row.StartDate = start
		row.EndDate = end
		row.Status = req.Status
		row.AcademicYearID = &req.AcademicYearID
		return tx.Save(&row).Error
	})
	if err != nil {
		c.JSON(400, gin.H{"message": "Không lưu được học kỳ: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Đã lưu học kỳ", "data": row})
}
func ListAdminCourseOfferings(c *gin.Context) {
	var rows []models.CourseOffering
	query := config.DB.Preload("Class").Preload("Course").Preload("Teacher.User").Preload("Semester").Preload("Room").Preload("Schedules")
	if id := c.Query("semesterId"); id != "" {
		query = query.Where("semester_id = ?", id)
	}
	if err := query.Order("semester_id desc, class_id, course_id").Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"data": rows})
}
