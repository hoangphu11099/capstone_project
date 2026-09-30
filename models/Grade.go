package models

type Grade struct {
	ID uint `gorm:"primaryKey"`

	EnrollmentID uint
	Enrollment   Enrollment `gorm:"foreignKey:EnrollmentID"`

	AssignmentScore float64
	MidtermScore    float64
	FinalScore      float64
	TotalScore      float64
	GradeLetter     string
	Status          string `gorm:"default:Approved"`
	Remark          string

	// Giữ lại 2 cột cũ để không mất dữ liệu nếu database đã từng seed bằng phiên bản trước.
	Score       float64
	LetterGrade string
}
