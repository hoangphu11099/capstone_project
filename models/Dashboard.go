package models

import "time"

// Dashboard lưu snapshot dashboard cá nhân theo user nếu cần.
type Dashboard struct {
	ID uint `gorm:"primaryKey"`

	UserID uint
	User   User `gorm:"foreignKey:UserID"`

	Role        string
	SummaryJSON string `gorm:"type:text"`
	GeneratedAt time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}
