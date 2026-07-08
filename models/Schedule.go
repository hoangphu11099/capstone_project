package models

type Schedule struct {
	ID        uint `gorm:"primaryKey"`
	ClassID   uint
	Class     Class `gorm:"foreignKey:ClassID"`
	DayOfWeek string
	Session   string
	StartTime string
	EndTime   string
}
