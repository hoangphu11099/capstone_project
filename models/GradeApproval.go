package models

import "time"

type GradeApproval struct {
	ID uint `gorm:"primaryKey"`

	GradeID uint
	Grade   Grade `gorm:"foreignKey:GradeID"`

	TeacherID uint
	Teacher   Teacher `gorm:"foreignKey:TeacherID"`

	Status     string `gorm:"default:approved"`
	Note       string
	ApprovedAt time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}
