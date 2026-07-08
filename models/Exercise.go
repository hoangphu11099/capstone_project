package models

import "time"

type Exercise struct {
	ID uint `gorm:"primaryKey"`

	ClassID uint
	Class   Class `gorm:"foreignKey:ClassID"`

	TeacherID uint
	Teacher   Teacher `gorm:"foreignKey:TeacherID"`

	Title       string `gorm:"not null"`
	Description string `gorm:"type:text"`
	Attachment  string
	DueDate     time.Time
	Status      string `gorm:"default:open"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Submissions []Submission `gorm:"foreignKey:ExerciseID"`
}
