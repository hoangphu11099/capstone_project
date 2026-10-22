package controllers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"student-management/config"
	"student-management/models"
	"time"
)

type CreateClassOfferRequest struct {
	ClassID       uint     `json:"classId" binding:"required"`
	TeacherID     uint     `json:"teacherId" binding:"required"`
	Title         string   `json:"title"`
	Content       string   `json:"content"`
	Message       string   `json:"message"`
	SemesterID    uint     `json:"semesterId" binding:"required"`
	CourseID      uint     `json:"courseId" binding:"required"`
	RoomID        uint     `json:"roomId" binding:"required"`
	TeachingDates []string `json:"teachingDates" binding:"required"`
	Session       string   `json:"session"`
	StartTime     string   `json:"startTime" binding:"required"`
	EndTime       string   `json:"endTime" binding:"required"`
}
type ClassOfferResponseRequest struct {
	Note string `json:"note"`
}

// The plan records exactly what the teacher is agreeing to.
type teachingOfferPlan struct {
	Assignment  AssignCourseOfferingRequest `json:"assignment"`
	Schedules   []models.Schedule           `json:"schedules"`
	BaseVersion string                      `json:"baseVersion"`
}

func offeringVersion(tx *gorm.DB, classID, courseID, semesterID uint) (string, []models.Schedule, error) {
	var offering models.CourseOffering
	err := tx.Where("class_id = ? AND course_id = ? AND semester_id = ?", classID, courseID, semesterID).First(&offering).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "new", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	var schedules []models.Schedule
	if err := tx.Where("course_offering_id = ?", offering.ID).Order("id").Find(&schedules).Error; err != nil {
		return "", nil, err
	}
	snapshot := struct {
		ID, TeacherID, RoomID uint
		Status                string
		Schedules             []models.Schedule
	}{offering.ID, offering.TeacherID, offering.RoomID, offering.Status, schedules}
	data, err := json.Marshal(snapshot)
	return fmt.Sprintf("%x", sha256.Sum256(data)), schedules, err
}

func mergeOfferSchedules(retained, candidates []models.Schedule) []models.Schedule {
	result := append([]models.Schedule{}, candidates...)
	for _, old := range retained {
		replaced := false
		for _, next := range candidates {
			if old.FullDate != nil && next.FullDate != nil && old.FullDate.Format("2006-01-02") == next.FullDate.Format("2006-01-02") {
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, old)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].FullDate == nil {
			return false
		}
		if result[j].FullDate == nil {
			return true
		}
		if result[i].FullDate.Equal(*result[j].FullDate) {
			return result[i].StartTime < result[j].StartTime
		}
		return result[i].FullDate.Before(*result[j].FullDate)
	})
	return result
}

func CreateClassOffer(c *gin.Context) {
	adminID, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	var req CreateClassOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "Vui lòng chọn lớp, môn, học kỳ, giáo viên, phòng và lịch dạy đầy đủ"})
		return
	}
	if len(req.TeachingDates) == 0 {
		c.JSON(400, gin.H{"message": "Cần ít nhất một ngày dạy"})
		return
	}
	assignment := AssignCourseOfferingRequest{CourseID: req.CourseID, SemesterID: req.SemesterID, TeacherID: req.TeacherID, RoomID: req.RoomID, TeachingDates: req.TeachingDates, Session: req.Session, StartTime: req.StartTime, EndTime: req.EndTime}
	candidates, err := assignmentSchedules(assignment)
	if err == nil {
		err = validateFutureTeachingDates(candidates, attendanceNow())
	}
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	var offer models.ClassOffer
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		var existing models.CourseOffering
		if err := applyCourseAssignment(tx, req.ClassID, assignment, candidates, &existing, true); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.ClassOffer{}).Where("class_id = ? AND course_id = ? AND semester_id = ? AND status = ?", req.ClassID, req.CourseID, req.SemesterID, "pending").Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("Môn này đã có đề xuất đang chờ trong học kỳ; hãy xử lý đề xuất trước")
		}
		version, retained, err := offeringVersion(tx, req.ClassID, req.CourseID, req.SemesterID)
		if err != nil {
			return err
		}
		schedules := mergeOfferSchedules(retained, candidates)
		for _, schedule := range schedules {
			if schedule.FullDate == nil {
				return errors.New("Học phần đang có lịch lặp theo tuần; hãy chuyển sang lịch ngày cụ thể trước khi gửi đề xuất")
			}
		}
		plan, err := json.Marshal(teachingOfferPlan{Assignment: assignment, Schedules: schedules, BaseVersion: version})
		if err != nil {
			return err
		}
		title, content := strings.TrimSpace(req.Title), strings.TrimSpace(req.Content)
		if title == "" {
			title = "Đề xuất phân công giảng dạy"
		}
		if content == "" {
			content = strings.TrimSpace(req.Message)
		}
		offer = models.ClassOffer{ClassID: req.ClassID, TeacherID: req.TeacherID, CourseID: &req.CourseID, SemesterID: &req.SemesterID, RoomID: &req.RoomID, PlanJSON: string(plan), OfferedByUserID: adminID, Status: "pending", Title: title, Content: content, Message: content, OfferedAt: time.Now()}
		return tx.Create(&offer).Error
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		c.JSON(400, gin.H{"message": "Không thể gửi đề xuất: " + err.Error()})
		return
	}
	c.JSON(201, gin.H{"message": "Đã gửi kế hoạch giảng dạy; lịch sẽ được xếp khi giáo viên chấp nhận", "data": offer})
}

func ViewSuggestClass(c *gin.Context) {
	query := config.DB.Preload("Class.Major").Preload("Class.Room").Preload("Class.Semester").Preload("Teacher.User").Preload("OfferedByUser").Preload("Course").Preload("Semester").Preload("Room").Order("created_at desc, id desc")
	if c.GetString("role") == "teacher" {
		teacher, ok := getCurrentTeacher(c)
		if !ok {
			return
		}
		query = query.Where("teacher_id = ?", teacher.ID)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	var offers []models.ClassOffer
	if err := query.Find(&offers).Error; err != nil {
		c.JSON(500, gin.H{"message": "Không tải được đề xuất"})
		return
	}
	c.JSON(200, gin.H{"data": offers})
}

func AcceptClassOffer(c *gin.Context) { respondClassOffer(c, "accepted") }
func RejectClassOffer(c *gin.Context) { respondClassOffer(c, "rejected") }

func respondClassOffer(c *gin.Context, status string) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"message": "ID đề xuất không hợp lệ"})
		return
	}
	var req ClassOfferResponseRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"message": "Phản hồi không hợp lệ"})
			return
		}
	}
	var offer models.ClassOffer
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND teacher_id = ?", id, teacher.ID).First(&offer).Error; err != nil {
			return err
		}
		if offer.Status != "pending" {
			return errors.New("Đề xuất này đã được xử lý")
		}
		if status == "accepted" {
			var plan teachingOfferPlan
			if offer.PlanJSON == "" || json.Unmarshal([]byte(offer.PlanJSON), &plan) != nil {
				return errors.New("Đề xuất cũ chưa có môn và lịch chi tiết; yêu cầu admin gửi lại kế hoạch đầy đủ")
			}
			var class models.Class
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&class, offer.ClassID).Error; err != nil {
				return err
			}
			version, _, err := offeringVersion(tx, offer.ClassID, plan.Assignment.CourseID, plan.Assignment.SemesterID)
			if err != nil {
				return err
			}
			if version != plan.BaseVersion {
				return errors.New("Học phần hoặc lịch đã thay đổi từ lúc đề xuất; yêu cầu admin gửi lại để xem lịch chính xác")
			}
			candidates, err := assignmentSchedules(plan.Assignment)
			if err != nil {
				return err
			}
			if err := validateFutureTeachingDates(candidates, attendanceNow()); err != nil {
				return err
			}
			plan.Assignment.TeacherID = teacher.ID
			var saved models.CourseOffering
			if err := applyCourseAssignment(tx, offer.ClassID, plan.Assignment, candidates, &saved, false); err != nil {
				return err
			}
		}
		now := time.Now()
		return tx.Model(&offer).Updates(map[string]interface{}{"status": status, "response_note": strings.TrimSpace(req.Note), "responded_at": now}).Error
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy đề xuất của giảng viên"})
		return
	}
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	message := "Đã từ chối đề xuất"
	if status == "accepted" {
		message = "Đã chấp nhận và tự xếp lịch vào Lớp của tôi / Lịch dạy"
	}
	c.JSON(200, gin.H{"message": message})
}
