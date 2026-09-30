package models

import "time"

type Transcript struct {
	ID uint `gorm:"primaryKey"`

	StudentID uint
	Student   Student `gorm:"foreignKey:StudentID"`

	CourseID uint
	Course   Course `gorm:"foreignKey:CourseID"`

	EnrollmentID uint
	Enrollment   Enrollment `gorm:"foreignKey:EnrollmentID"`

	GradeID uint
	Grade   Grade `gorm:"foreignKey:GradeID"`

	SemesterID uint
	Semester   Semester `gorm:"foreignKey:SemesterID"`

	AssignmentScore float64
	MidtermScore    float64
	FinalScore      float64
	TotalScore      float64
	GradeLetter     string
	Status          string `gorm:"default:Approved"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
