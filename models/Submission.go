package models

import "time"

type Submission struct {
	ID uint `gorm:"primaryKey"`

	ExerciseID uint
	Exercise   Exercise `gorm:"foreignKey:ExerciseID"`

	StudentID uint
	Student   Student `gorm:"foreignKey:StudentID"`

	Content     string `gorm:"type:text"`
	FileURL     string
	SubmittedAt time.Time
	Status      string `gorm:"default:submitted"`
	Score       *float64
	Feedback    string `gorm:"type:text"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
