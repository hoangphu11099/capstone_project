package models

import "time"

// ClassTeacher lưu lịch sử phân công giảng viên cho lớp.
type ClassTeacher struct {
	ID uint `gorm:"primaryKey"`

	ClassID uint
	Class   Class `gorm:"foreignKey:ClassID"`

	TeacherID uint
	Teacher   Teacher `gorm:"foreignKey:TeacherID"`

	AssignedByUserID uint
	AssignedByUser   User `gorm:"foreignKey:AssignedByUserID"`

	Status     string `gorm:"default:active"`
	AssignedAt time.Time
	Note       string

	CreatedAt time.Time
	UpdatedAt time.Time
}
