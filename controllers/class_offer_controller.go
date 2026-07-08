package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CreateClassOfferRequest struct {
	ClassID   uint   `json:"classId" binding:"required"`
	TeacherID uint   `json:"teacherId" binding:"required"`
	Message   string `json:"message"`
}

type ClassOfferResponseRequest struct {
	Note string `json:"note"`
}

func CreateClassOffer(c *gin.Context) {
	adminUserID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	var req CreateClassOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu đề xuất lớp không hợp lệ", "error": err.Error()})
		return
	}

	var class models.Class
	if err := config.DB.First(&class, req.ClassID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy lớp"})
		return
	}

	var teacher models.Teacher
	if err := config.DB.First(&teacher, req.TeacherID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy giảng viên"})
		return
	}

	var existed int64
	if err := config.DB.Model(&models.ClassOffer{}).
		Where("class_id = ? AND teacher_id = ? AND status = ?", req.ClassID, req.TeacherID, "pending").
		Count(&existed).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi kiểm tra đề xuất lớp", "error": err.Error()})
		return
	}
	if existed > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Đã có đề xuất đang chờ cho lớp và giảng viên này"})
		return
	}

	offer := models.ClassOffer{
		ClassID:         req.ClassID,
		TeacherID:       req.TeacherID,
		OfferedByUserID: adminUserID,
		Status:          "pending",
		Message:         strings.TrimSpace(req.Message),
		OfferedAt:       time.Now(),
	}

	if err := config.DB.Create(&offer).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Tạo đề xuất lớp thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Tạo đề xuất lớp thành công", "data": offer})
}

func ViewSuggestClass(c *gin.Context) {
	role := c.GetString("role")
	query := config.DB.Preload("Class.Major").Preload("Class.Room").Preload("Class.Semester").Preload("Teacher.User").Preload("OfferedByUser").Order("created_at desc")

	if role == "teacher" {
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
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách lớp đề xuất", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách lớp đề xuất thành công", "data": offers})
}

func AcceptClassOffer(c *gin.Context) {
	respondClassOffer(c, "accepted")
}

func RejectClassOffer(c *gin.Context) {
	respondClassOffer(c, "rejected")
}

func respondClassOffer(c *gin.Context, status string) {
	teacher, ok := getCurrentTeacher(c)
	if !ok {
		return
	}

	offerID, err := strconv.Atoi(c.Param("id"))
	if err != nil || offerID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID đề xuất lớp không hợp lệ"})
		return
	}

	var req ClassOfferResponseRequest
	_ = c.ShouldBindJSON(&req)

	var offer models.ClassOffer
	if err := config.DB.Where("id = ? AND teacher_id = ?", offerID, teacher.ID).First(&offer).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy đề xuất lớp"})
		return
	}

	if offer.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Đề xuất lớp này đã được xử lý"})
		return
	}

	now := time.Now()
	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		offer.Status = status
		offer.ResponseNote = strings.TrimSpace(req.Note)
		offer.RespondedAt = &now
		if err := tx.Save(&offer).Error; err != nil {
			return err
		}

		if status == "accepted" {
			var class models.Class
			if err := tx.First(&class, offer.ClassID).Error; err != nil {
				return err
			}
			class.TeacherID = teacher.ID
			if err := tx.Save(&class).Error; err != nil {
				return err
			}

			if err := tx.Model(&models.ClassTeacher{}).
				Where("class_id = ? AND status = ?", class.ID, "active").
				Update("status", "replaced").Error; err != nil {
				return err
			}

			classTeacher := models.ClassTeacher{
				ClassID:          class.ID,
				TeacherID:        teacher.ID,
				AssignedByUserID: offer.OfferedByUserID,
				Status:           "active",
				AssignedAt:       now,
				Note:             "Giảng viên chấp nhận đề xuất lớp",
			}
			if err := tx.Create(&classTeacher).Error; err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Xử lý đề xuất lớp thất bại", "error": err.Error()})
		return
	}

	message := "Từ chối đề xuất lớp thành công"
	if status == "accepted" {
		message = "Chấp nhận đề xuất lớp thành công"
	}
	c.JSON(http.StatusOK, gin.H{"message": message, "data": offer})
}
