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

// TeacherScheduleResponse describes one recurring teaching slot assigned through
// a course offering. Keeping the offering information in the response prevents
// a class with several courses from being displayed as one ambiguous lesson.
type TeacherScheduleResponse struct {
	ID                uint   `json:"id"`
	CourseOfferingID  uint   `json:"courseOfferingId"`
	ClassID           uint   `json:"classId"`
	ClassCode         string `json:"classCode"`
	MajorName         string `json:"majorName"`
	CourseCode        string `json:"courseCode"`
	CourseName        string `json:"courseName"`
	RoomName          string `json:"roomName"`
	Building          string `json:"building"`
	DayOfWeek         string `json:"dayOfWeek"`
	Session           string `json:"session"`
	StartTime         string `json:"startTime"`
	EndTime           string `json:"endTime"`
	SemesterName      string `json:"semesterName"`
	SemesterStartDate string `json:"semesterStartDate"`
	SemesterEndDate   string `json:"semesterEndDate"`
	OfferingStatus    string `json:"offeringStatus"`
}
