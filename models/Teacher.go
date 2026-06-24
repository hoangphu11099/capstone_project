package models

import "gorm.io/gorm"

type Teacher struct {
	gorm.Model
	TeacherCode string
	FullName    string
	Department  string
}
