package models

type TranscriptResponse struct {
	CourseCode string
	CourseName string
	Credits    int

	AssignmentScore float64
	MidtermScore    float64
	FinalScore      float64

	TotalScore  float64
	GradeLetter string
}
