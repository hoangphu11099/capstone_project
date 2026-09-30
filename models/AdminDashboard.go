package models

import "time"

// AdminDashboard lưu snapshot thống kê nếu cần lưu lại báo cáo dashboard.
type AdminDashboard struct {
	ID uint `gorm:"primaryKey"`

	TotalStudents      int
	TotalTeachers      int
	TotalClasses       int
	PendingClassOffers int
	NewNotifications   int
	GeneratedAt        time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}
