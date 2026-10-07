package controllers

import (
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"strconv"
	"strings"
	"student-management/config"
	"student-management/models"
)

type RoomRequest struct {
	Name        string `json:"name" binding:"required"`
	Building    string `json:"building"`
	Capacity    int    `json:"capacity" binding:"required"`
	Description string `json:"description"`
}

func ListRooms(c *gin.Context) {
	var rooms []models.Room
	if err := config.DB.Order("building, name").Find(&rooms).Error; err != nil {
		c.JSON(500, gin.H{"message": "Không tải được danh sách phòng"})
		return
	}
	c.JSON(200, gin.H{"data": rooms})
}

func SaveRoom(c *gin.Context) {
	var req RoomRequest
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" || req.Capacity <= 0 {
		c.JSON(400, gin.H{"message": "Nhập tên phòng và sức chứa là số nguyên lớn hơn 0"})
		return
	}
	var room models.Room
	creating := c.Param("id") == ""
	if !creating {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 {
			c.JSON(400, gin.H{"message": "ID phòng không hợp lệ"})
			return
		}
		if err := config.DB.First(&room, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(404, gin.H{"message": "Không tìm thấy phòng"})
			} else {
				c.JSON(500, gin.H{"message": "Không tải được phòng"})
			}
			return
		}
	}
	room.Name = strings.TrimSpace(req.Name)
	room.Building = strings.TrimSpace(req.Building)
	room.Capacity = req.Capacity
	room.Description = strings.TrimSpace(req.Description)
	var count int64
	if err := config.DB.Model(&models.Room{}).Where("name = ? AND id <> ?", room.Name, room.ID).Count(&count).Error; err != nil {
		c.JSON(500, gin.H{"message": "Không kiểm tra được tên phòng"})
		return
	}
	if count > 0 {
		c.JSON(400, gin.H{"message": "Tên phòng đã tồn tại. Vui lòng dùng tên khác"})
		return
	}
	var err error
	if creating {
		room.IsActive = true
		err = config.DB.Create(&room).Error
	} else {
		err = config.DB.Model(&models.Room{}).Where("id = ?", room.ID).Updates(map[string]interface{}{"name": room.Name, "building": room.Building, "capacity": room.Capacity, "description": room.Description}).Error
	}
	if err != nil {
		c.JSON(400, gin.H{"message": "Không lưu được phòng. Kiểm tra tên phòng có bị trùng"})
		return
	}
	status := 200
	if creating {
		status = 201
	}
	c.JSON(status, gin.H{"message": "Đã lưu phòng học", "data": room})
}
