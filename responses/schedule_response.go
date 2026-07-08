package responses

type ScheduleResponse struct {
	CourseCode  string `json:"courseCode"`
	CourseName  string `json:"courseName"`
	TeacherName string `json:"teacherName"`
	Room        string `json:"room"`
	DayOfWeek   string `json:"dayOfWeek"`
	Period      string `json:"period"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
}
