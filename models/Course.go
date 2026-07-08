package models

type Course struct {
	ID         uint   `gorm:"primaryKey"`
	Code       string `gorm:"unique;not null"`
	Name       string `gorm:"not null"`
	Credits    int
	MajorID    uint
	Major      Major `gorm:"foreignKey:MajorID"`
	SemesterID uint
	Semester   Semester `gorm:"foreignKey:SemesterID"`
	IsActive   bool     `gorm:"default:true"`

	Enrollments []Enrollment
}
