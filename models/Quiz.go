package models

import "time"

type Quiz struct {
	ID               uint `gorm:"primaryKey"`
	CourseOfferingID uint `gorm:"index"`
	CourseOffering   CourseOffering
	Title            string
	Kind             string
	Status           string `gorm:"default:draft"`
	DueDate          time.Time
	CreatedAt        time.Time
}
type QuizQuestion struct {
	ID      uint `gorm:"primaryKey"`
	QuizID  uint `gorm:"uniqueIndex:idx_quiz_number"`
	Quiz    Quiz
	Number  int    `gorm:"uniqueIndex:idx_quiz_number"`
	Content string `gorm:"type:text"`
	A       string `gorm:"type:text"`
	B       string `gorm:"type:text"`
	C       string `gorm:"type:text"`
	D       string `gorm:"type:text"`
	Answer  string `json:"-"`
}
type QuizAttempt struct {
	ID          uint `gorm:"primaryKey"`
	QuizID      uint `gorm:"uniqueIndex:idx_quiz_student"`
	Quiz        Quiz
	StudentID   uint `gorm:"uniqueIndex:idx_quiz_student"`
	Student     Student
	Answers     string `gorm:"type:longtext" json:"-"`
	Score       float64
	SubmittedAt time.Time
}
