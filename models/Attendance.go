package models

import "time"

type Attendance struct {
	ID uint `gorm:"primaryKey"`

	EnrollmentID uint
	Enrollment   Enrollment `gorm:"foreignKey:EnrollmentID"`

	ClassDate time.Time
	Status    string
	Note      string
}
