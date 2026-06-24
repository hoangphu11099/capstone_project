package models

import "gorm.io/gorm"

type Student struct {
	gorm.Model
	StudentCode string
	FullName    string
	BirthDate   string
	Gender      string
	ClassID     uint
}
