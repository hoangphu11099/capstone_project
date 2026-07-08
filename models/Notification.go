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
	Channel        string `gorm:"default:email"`
	Status         string `gorm:"default:created"`
	SentAt         *time.Time
	CreatedByID    *uint
	CreatedBy      *User `gorm:"foreignKey:CreatedByID"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
