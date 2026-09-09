package models

import "time"

type Attendance struct {
	ID uint `gorm:"primaryKey"`

	EnrollmentID uint
	Enrollment   Enrollment `gorm:"foreignKey:EnrollmentID"`

	AttendanceSessionID *uint
	AttendanceSession   *AttendanceSession `gorm:"foreignKey:AttendanceSessionID"`

	ClassDate   time.Time
	Status      string
	Note        string
	CheckedInAt *time.Time
}
