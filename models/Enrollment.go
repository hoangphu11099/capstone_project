package models

import "time"

type Enrollment struct {
	ID uint `gorm:"primaryKey"`

	StudentID uint
	Student   Student `gorm:"foreignKey:StudentID"`

	ClassID uint
	Class   Class `gorm:"foreignKey:ClassID"`

	CourseID uint
	Course   Course `gorm:"foreignKey:CourseID"`

	EnrollDate time.Time
	Status     string `gorm:"default:enrolled"`

	Grade       *Grade       `gorm:"foreignKey:EnrollmentID"`
	Attendances []Attendance `gorm:"foreignKey:EnrollmentID"`
}
