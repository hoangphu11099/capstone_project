package models

import "time"

// CourseRegistration lưu lịch sử đăng ký/hủy học phần của sinh viên.
// Project vẫn dùng Enrollment làm bảng đăng ký học chính; model này giúp đúng entity/course_registrations trong tài liệu.
type CourseRegistration struct {
	ID                 uint            `gorm:"primaryKey"`
	CourseOfferingID   *uint           `gorm:"index"`
	CourseOffering     *CourseOffering `gorm:"foreignKey:CourseOfferingID"`
	PaymentConfirmedAt *time.Time
	PaymentConfirmedBy *uint
	ScheduledAt        *time.Time

	StudentID uint
	Student   Student `gorm:"foreignKey:StudentID"`

	CourseID uint
	Course   Course `gorm:"foreignKey:CourseID"`

	ClassID uint
	Class   Class `gorm:"foreignKey:ClassID"`

	EnrollmentID *uint
	Enrollment   *Enrollment `gorm:"foreignKey:EnrollmentID"`

	Status       string `gorm:"default:registered"`
	RegisteredAt time.Time
	CanceledAt   *time.Time
	Note         string

	CreatedAt time.Time
	UpdatedAt time.Time
}
