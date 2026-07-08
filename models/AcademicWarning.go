package models

import "time"

type AcademicWarning struct {
	ID uint `gorm:"primaryKey"`

	StudentID uint
	Student   Student `gorm:"foreignKey:StudentID"`

	SemesterID uint
	Semester   Semester `gorm:"foreignKey:SemesterID"`

	GPA           float64
	FailedCourses int
	FailedCredits int
	Reason        string `gorm:"type:text"`
	Status        string `gorm:"default:active"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
