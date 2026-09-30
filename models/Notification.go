package models

import "time"

// Notification lưu thông báo/email gửi cho người dùng.
type Notification struct {
	ID uint `gorm:"primaryKey"`

	RecipientUserID *uint
	RecipientUser   *User `gorm:"foreignKey:RecipientUserID"`

	RecipientEmail string
	Subject        string `gorm:"not null"`
	Content        string `gorm:"type:text;not null"`
	TargetType     string `gorm:"size:32;default:user"`
	Channel        string `gorm:"size:32;default:app"`
	Status         string `gorm:"default:created"`
	SentAt         *time.Time
	IsRead         bool `gorm:"default:false;index"`
	ReadAt         *time.Time
	CreatedByID    *uint
	CreatedBy      *User `gorm:"foreignKey:CreatedByID"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
