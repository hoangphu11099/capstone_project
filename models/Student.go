package models

import "time"

type Student struct {
	ID             uint   `gorm:"primaryKey"`
	StudentCode    string `gorm:"unique;not null"`
	UserID         uint   `gorm:"unique;not null"`
	User           User   `gorm:"foreignKey:UserID"`
	ClassID        uint
	Class          Class `gorm:"foreignKey:ClassID"`
	DateOfBirth    time.Time
	Gender         string
	Phone          string
	Address        string
	EnrollmentDate time.Time
	Status         string `gorm:"default:active"`

	Enrollments []Enrollment
}
