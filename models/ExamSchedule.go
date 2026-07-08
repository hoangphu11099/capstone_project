package models

import "time"

type ExamSchedule struct {
	ID uint `gorm:"primaryKey"`

	ClassID uint
	Class   Class `gorm:"foreignKey:ClassID"`

	CourseID uint
	Course   Course `gorm:"foreignKey:CourseID"`

	SemesterID uint
	Semester   Semester `gorm:"foreignKey:SemesterID"`

	RoomID uint
	Room   Room `gorm:"foreignKey:RoomID"`

	ExamDate  time.Time
	Session   string
	StartTime string
	EndTime   string
	ExamType  string
	Note      string

	CreatedAt time.Time
	UpdatedAt time.Time
}
