package models

type Schedule struct {
	ID               uint `gorm:"primaryKey"`
	ClassID          uint
	Class            Class `gorm:"foreignKey:ClassID"`
	CourseOfferingID *uint           `gorm:"uniqueIndex"`
	CourseOffering   *CourseOffering `gorm:"foreignKey:CourseOfferingID"`
	DayOfWeek        string
	Session          string
	StartTime        string
	EndTime          string
}
