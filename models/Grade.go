package models

import "gorm.io/gorm"

type Grade struct {
	gorm.Model
	StudentID uint
	CourseID  uint
	Score     float64
}
