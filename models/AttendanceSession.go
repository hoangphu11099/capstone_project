package models

import "time"

type AttendanceSession struct {
	ID uint `gorm:"primaryKey"`

	ClassID uint
	Class   Class `gorm:"foreignKey:ClassID"`

	CourseID uint
	Course   Course `gorm:"foreignKey:CourseID"`

	TeacherID uint
	Teacher   Teacher `gorm:"foreignKey:TeacherID"`

	Code      string `gorm:"uniqueIndex;size:128;not null"`
	ClassDate time.Time
	ExpiresAt time.Time
	IsActive  bool `gorm:"default:true"`
	Note      string

	CreatedAt time.Time
	UpdatedAt time.Time

	Attendances []Attendance `gorm:"foreignKey:AttendanceSessionID"`
}
