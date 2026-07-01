package responses

type TranscriptItem struct {
	CourseCode string `json:"course_code"`
	CourseName string `json:"course_name"`
	Credits    int    `json:"credits"`

	AssignmentScore float64 `json:"assignment_score"`
	MidtermScore    float64 `json:"midterm_score"`
	FinalScore      float64 `json:"final_score"`

	TotalScore  float64 `json:"total_score"`
	GradeLetter string  `json:"grade_letter"`
}

type TranscriptResponse struct {
	StudentID   uint   `json:"student_id"`
	StudentCode string `json:"student_code"`
	StudentName string `json:"student_name"`

	SemesterID   uint   `json:"semester_id"`
	SemesterName string `json:"semester_name"`

	SemesterGPA float64          `json:"semester_gpa"`
	TotalGPA    float64          `json:"total_gpa"`
	TotalCredit int              `json:"total_credit"`
	Courses     []TranscriptItem `json:"courses"`
}
