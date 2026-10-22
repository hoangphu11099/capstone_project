package models

import "time"

type Teacher struct {
	ID            uint   `gorm:"primaryKey"`
	TeacherCode   string `gorm:"unique;not null"`
	UserID        uint   `gorm:"unique;not null"`
	User          User   `gorm:"foreignKey:UserID"`
	Phone         string
	Address       string
	Qualification string
	Biography     string `gorm:"type:text"`
	AvatarURL     string `gorm:"size:2048"`
	HireDate      time.Time
	MajorID       uint  `gorm:"default:null"`
	Major         Major `gorm:"foreignKey:MajorID"`

	Classes         []Class
	CourseOfferings []CourseOffering
}
