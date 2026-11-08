package models

import (
	"gorm.io/gorm"
	"time"
)

type Student struct {
	ID             uint      `gorm:"primaryKey"`
	StudentCode    string    `gorm:"unique;not null"`
	UserID         uint      `gorm:"unique;not null"`
	User           User      `gorm:"foreignKey:UserID"`
	ClassID        uint      `gorm:"default:null"`
	Class          Class     `gorm:"foreignKey:ClassID"`
	MajorID        uint      `gorm:"index;default:null"`
	Major          Major     `gorm:"foreignKey:MajorID"`
	TrainingSystem string    `gorm:"size:16;default:CQ"`
	DateOfBirth    time.Time `gorm:"default:null"`
	Gender         string
	Phone          string
	Address        string
	EnrollmentDate time.Time
	Status         string `gorm:"default:active"`

	Enrollments []Enrollment
}

// Keep the program registered at admission independently of later class changes.
func (s *Student) BeforeCreate(tx *gorm.DB) error {
	if s.MajorID != 0 || s.ClassID == 0 {
		return nil
	}
	var class Class
	if err := tx.Select("id", "major_id").First(&class, s.ClassID).Error; err != nil {
		return err
	}
	s.MajorID = class.MajorID
	return nil
}
