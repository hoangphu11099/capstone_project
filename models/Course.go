package models

import "gorm.io/gorm"

type Course struct {
	gorm.Model
	CourseCode string `gorm:"type:varchar(100);charset:utf8mb4"`
	CourseName string `gorm:"type:varchar(255);charset:utf8mb4"`
	Credits    int
	TeacherID  uint
}
