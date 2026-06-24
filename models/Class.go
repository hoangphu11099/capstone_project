package models

import (
	"gorm.io/gorm"
	"time"
)

type Class struct {
	ClassName string
	Major     string
	TeacherID uint

	Semester string
	TimeSlot string
	Weekday  string

	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
