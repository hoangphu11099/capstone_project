package models

import "time"

// ClassOffer là đề xuất/phân công lớp cho giảng viên chấp nhận hoặc từ chối.
type ClassOffer struct {
	ID uint `gorm:"primaryKey"`

	ClassID uint
	Class   Class `gorm:"foreignKey:ClassID"`

	CourseID   *uint
	Course     Course `gorm:"foreignKey:CourseID"`
	SemesterID *uint
	Semester   Semester `gorm:"foreignKey:SemesterID"`
	RoomID     *uint
	Room       Room   `gorm:"foreignKey:RoomID"`
	PlanJSON   string `gorm:"type:longtext"`

	TeacherID uint
	Teacher   Teacher `gorm:"foreignKey:TeacherID"`

	OfferedByUserID uint
	OfferedByUser   User `gorm:"foreignKey:OfferedByUserID"`

	Status       string `gorm:"default:pending"`
	Title        string `gorm:"size:255"`
	Content      string `gorm:"type:text"`
	Message      string `gorm:"type:text"`
	ResponseNote string `gorm:"type:text"`
	OfferedAt    time.Time
	RespondedAt  *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}
