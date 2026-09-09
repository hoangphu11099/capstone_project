package responses

type TranscriptItem struct {
	CourseCode string `json:"courseCode"`
	CourseName string `json:"courseName"`
	Credits    int    `json:"credits"`

	AssignmentScore float64 `json:"assignmentScore"`
	MidtermScore    float64 `json:"midtermScore"`
	FinalScore      float64 `json:"finalScore"`

	TotalScore  float64 `json:"totalScore"`
	GradeLetter string  `json:"gradeLetter"`
}

type TranscriptResponse struct {
	GPA         float64          `json:"gpa"`
	TotalCredit int              `json:"totalCredit"`
	Transcript  []TranscriptItem `json:"transcript"`
}
