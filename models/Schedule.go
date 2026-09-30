package models

import "time"

type Schedule struct {
	ID               uint `gorm:"primaryKey"`
	ClassID          uint
	Class            Class           `gorm:"foreignKey:ClassID"`
	CourseOfferingID *uint           `gorm:"index"`
	CourseOffering   *CourseOffering `gorm:"foreignKey:CourseOfferingID"`
	DayOfWeek        string
	FullDate         *time.Time `gorm:"type:date"`
	Session          string
	StartTime        string
	EndTime          string
}
