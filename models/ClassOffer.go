package models

import "time"

// ClassOffer là đề xuất/phân công lớp cho giảng viên chấp nhận hoặc từ chối.
type ClassOffer struct {
	ID uint `gorm:"primaryKey"`

	ClassID uint
	Class   Class `gorm:"foreignKey:ClassID"`

	TeacherID uint
	Teacher   Teacher `gorm:"foreignKey:TeacherID"`

	OfferedByUserID uint
	OfferedByUser   User `gorm:"foreignKey:OfferedByUserID"`

	Status       string `gorm:"default:pending"`
	Message      string `gorm:"type:text"`
	ResponseNote string `gorm:"type:text"`
	OfferedAt    time.Time
	RespondedAt  *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}
