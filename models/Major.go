package models

import "time"

type Major struct {
	ID          uint   `gorm:"primaryKey"`
	Code        string `gorm:"unique;not null"`
	Name        string `gorm:"not null"`
	Description string
	IsActive    bool `gorm:"default:true"`

	Courses []Course
	Classes []Class
}

type Semester struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"unique;not null"`
	StartDate time.Time
	EndDate   time.Time
	Status    string

	Courses []Course
	Classes []Class
}
