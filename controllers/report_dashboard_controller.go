package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"student-management/config"
	"student-management/middleware"
	"student-management/models"
	"time"
)

type reportFilter struct {
	IDs      map[string]uint
	From, To string
}

func parseReportFilter(c *gin.Context) (reportFilter, error) {
	f := reportFilter{IDs: map[string]uint{}, From: c.Query("from"), To: c.Query("to")}
	for _, key := range []string{"academicYearId", "semesterId", "classId", "courseId", "majorId", "cohort"} {
		if raw := c.Query(key); raw != "" {
			n, err := strconv.ParseUint(raw, 10, 32)
			if err != nil || n == 0 {
				return f, fmt.Errorf("Bộ lọc %s không hợp lệ", key)
			}
			f.IDs[key] = uint(n)
		}
	}
	for _, value := range []string{f.From, f.To} {
		if value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				return f, fmt.Errorf("Ngày lọc phải có dạng YYYY-MM-DD")
			}
		}
	}
	if f.From != "" && f.To != "" && f.From > f.To {
		return f, fmt.Errorf("Từ ngày không được lớn hơn đến ngày")
	}
	return f, nil
}
func (f reportFilter) scope(db *gorm.DB, teacherID uint) *gorm.DB {
	q := db.Table("course_offerings o").Joins("JOIN classes cl ON cl.id=o.class_id").Joins("JOIN semesters sm ON sm.id=o.semester_id")
	for key, column := range map[string]string{"academicYearId": "sm.academic_year_id", "semesterId": "o.semester_id", "classId": "o.class_id", "courseId": "o.course_id", "majorId": "cl.major_id", "cohort": "cl.cohort_year"} {
		if id := f.IDs[key]; id > 0 {
			q = q.Where(column+"=?", id)
		}
	}
	if teacherID > 0 {
		q = q.Where("o.teacher_id=?", teacherID)
	}
	if f.From != "" || f.To != "" {
		sub := db.Table("schedules rs").Select("1").Where("rs.course_offering_id=o.id")
		if f.From != "" {
			sub = sub.Where("rs.full_date>=?", f.From)
		}
		if f.To != "" {
			sub = sub.Where("rs.full_date<=?", f.To)
		}
		q = q.Where("EXISTS (?)", sub)
	}
	return q
}
func (f reportFilter) inDate(date time.Time) bool {
	d := date.Format("2006-01-02")
	return (f.From == "" || d >= f.From) && (f.To == "" || d <= f.To)
}

type reportOption struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	ParentID uint   `json:"parentId,omitempty"`
	Cohort   int    `json:"cohort,omitempty"`
}
type reportEnrollment struct {
	ID, StudentID, OfferingID         uint
	Name, Code, ClassCode, CourseName string
	GradeID                           uint
	GradeStatus                       string
	Score                             float64
}
type reportAttendance struct {
	EnrollmentID uint
	ClassDate    time.Time
	Status       string
}
type reportTeaching struct {
	ID        uint     `json:"id"`
	Class     string   `json:"class"`
	Course    string   `json:"course"`
	Semester  string   `json:"semester"`
	Teacher   string   `json:"teacher"`
	Room      string   `json:"room"`
	Students  int      `json:"students"`
	Lessons   int      `json:"lessons"`
	Elapsed   int      `json:"elapsed"`
	Confirmed int      `json:"confirmed"`
	Hours     float64  `json:"hours"`
	First     string   `json:"first"`
	Last      string   `json:"last"`
	Present   int      `json:"present"`
	Absent    int      `json:"absent"`
	Late      int      `json:"late"`
	Excused   int      `json:"excused"`
	Approved  int      `json:"approved"`
	Passed    int      `json:"passed"`
	Draft     int      `json:"draft"`
	Average   *float64 `json:"average"`
}

func reportRound(n float64) float64 { return math.Round(n*100) / 100 }
func reportBand(score float64) int {
	if score >= 85 {
		return 0
	}
	if score >= 80 {
		return 1
	}
	if score >= 65 {
		return 2
	}
	if score >= 40 {
		return 3
	}
	return 4
}

func ViewStatisticalReport(c *gin.Context) {
	role := c.GetString(middleware.ContextRoleKey)
	if role != "admin" && role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"message": "Không có quyền xem báo cáo"})
		return
	}
	f, err := parseReportFilter(c)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	var teacherID uint
	if role == "teacher" {
		teacher, ok := getCurrentTeacher(c)
		if !ok {
			return
		}
		teacherID = teacher.ID
	}
	data, err := buildDashboardReport(config.DB, f, teacherID)
	if err != nil {
		c.JSON(500, gin.H{"message": "Không tải được báo cáo. Vui lòng thử lại."})
		return
	}
	data["scope"] = role
	data["generatedAt"] = time.Now()
	c.JSON(200, gin.H{"message": "Lấy báo cáo thành công", "data": data})
}

func buildDashboardReport(db *gorm.DB, f reportFilter, teacherID uint) (gin.H, error) {
	result := gin.H{}
	// Options come from the same authorized scope, independently of the active filters.
	var allowed []models.CourseOffering
	q := db.Preload("Class.Major").Preload("Course").Preload("Semester.AcademicYear")
	if teacherID > 0 {
		q = q.Where("teacher_id=?", teacherID)
	}
	if err := q.Find(&allowed).Error; err != nil {
		return nil, err
	}
	options := map[string][]reportOption{}
	for _, key := range []string{"academicYears", "semesters", "classes", "courses", "majors", "cohorts"} {
		options[key] = []reportOption{}
	}
	seen := map[string]map[uint]bool{}
	add := func(key string, option reportOption) {
		if option.ID == 0 {
			return
		}
		if seen[key] == nil {
			seen[key] = map[uint]bool{}
		}
		if !seen[key][option.ID] {
			seen[key][option.ID] = true
			options[key] = append(options[key], option)
		}
	}
	for _, o := range allowed {
		add("classes", reportOption{ID: o.ClassID, Name: o.Class.ClassCode, ParentID: o.Class.MajorID, Cohort: o.Class.CohortYear})
		add("courses", reportOption{ID: o.CourseID, Name: o.Course.Code + " · " + o.Course.Name, ParentID: o.Course.MajorID})
		add("majors", reportOption{ID: o.Class.MajorID, Name: o.Class.Major.Name})
		add("cohorts", reportOption{ID: uint(o.Class.CohortYear), Name: fmt.Sprintf("K%d", o.Class.CohortYear%100)})
		yearID := uint(0)
		if o.Semester.AcademicYearID != nil {
			yearID = *o.Semester.AcademicYearID
		}
		add("semesters", reportOption{ID: o.SemesterID, Name: o.Semester.Name, ParentID: yearID})
		if o.Semester.AcademicYear != nil {
			add("academicYears", reportOption{ID: yearID, Name: o.Semester.AcademicYear.Name})
		}
	}
	if teacherID == 0 {
		var classes []models.Class
		var courses []models.Course
		var terms []models.Semester
		var years []models.AcademicYear
		if err := db.Preload("Major").Find(&classes).Error; err != nil {
			return nil, err
		}
		if err := db.Find(&courses).Error; err != nil {
			return nil, err
		}
		if err := db.Find(&terms).Error; err != nil {
			return nil, err
		}
		if err := db.Find(&years).Error; err != nil {
			return nil, err
		}
		for _, cl := range classes {
			add("classes", reportOption{ID: cl.ID, Name: cl.ClassCode, ParentID: cl.MajorID, Cohort: cl.CohortYear})
			add("majors", reportOption{ID: cl.MajorID, Name: cl.Major.Name})
			add("cohorts", reportOption{ID: uint(cl.CohortYear), Name: fmt.Sprintf("K%d", cl.CohortYear%100)})
		}
		for _, cr := range courses {
			add("courses", reportOption{ID: cr.ID, Name: cr.Code + " · " + cr.Name, ParentID: cr.MajorID})
		}
		for _, sm := range terms {
			p := uint(0)
			if sm.AcademicYearID != nil {
				p = *sm.AcademicYearID
			}
			add("semesters", reportOption{ID: sm.ID, Name: sm.Name, ParentID: p})
		}
		for _, y := range years {
			add("academicYears", reportOption{ID: y.ID, Name: y.Name})
		}
	}
	for key := range options {
		sort.Slice(options[key], func(i, j int) bool { return options[key][i].Name < options[key][j].Name })
	}
	result["options"] = options
	var ids []uint
	if err := f.scope(db, teacherID).Pluck("o.id", &ids).Error; err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		ids = []uint{0}
	}
	var offerings []models.CourseOffering
	if err := db.Where("id IN ?", ids).Preload("Class").Preload("Course").Preload("Teacher.User").Preload("Room").Preload("Semester").Order("class_id,semester_id,course_id").Find(&offerings).Error; err != nil {
		return nil, err
	}
	var enrollments []reportEnrollment
	if err := db.Table("enrollments e").Select("e.id,e.student_id,e.course_offering_id AS offering_id,u.full_name AS name,s.student_code AS code,cl.class_code,cr.name AS course_name,COALESCE(g.id,0) AS grade_id,COALESCE(LOWER(g.status),'') AS grade_status,COALESCE(g.total_score,0) AS score").Joins("JOIN students s ON s.id=e.student_id").Joins("JOIN users u ON u.id=s.user_id").Joins("JOIN classes cl ON cl.id=e.class_id").Joins("JOIN courses cr ON cr.id=e.course_id").Joins("LEFT JOIN grades g ON g.enrollment_id=e.id").Where("e.course_offering_id IN ? AND e.status<>?", ids, "cancelled").Scan(&enrollments).Error; err != nil {
		return nil, err
	}
	var attendances []reportAttendance
	aq := db.Table("attendances a").Select("a.enrollment_id,a.class_date,a.status").Joins("JOIN enrollments e ON e.id=a.enrollment_id").Joins("LEFT JOIN attendance_sessions s ON s.id=a.attendance_session_id").Where("e.course_offering_id IN ? AND e.status<>?", ids, "cancelled").Where("a.attendance_session_id IS NULL OR s.status=?", "confirmed")
	if f.From != "" {
		aq = aq.Where("a.class_date>=?", f.From)
	}
	if f.To != "" {
		aq = aq.Where("a.class_date<DATE_ADD(?, INTERVAL 1 DAY)", f.To)
	}
	if err := aq.Scan(&attendances).Error; err != nil {
		return nil, err
	}
	var schedules []models.Schedule
	sq := db.Where("course_offering_id IN ?", ids)
	if f.From != "" {
		sq = sq.Where("full_date>=?", f.From)
	}
	if f.To != "" {
		sq = sq.Where("full_date<=?", f.To)
	}
	if err := sq.Order("full_date,start_time").Find(&schedules).Error; err != nil {
		return nil, err
	}
	var confirmed []uint
	cq := db.Model(&models.AttendanceSession{}).Where("course_offering_id IN ? AND status=?", ids, "confirmed")
	if f.From != "" {
		cq = cq.Where("class_date>=?", f.From)
	}
	if f.To != "" {
		cq = cq.Where("class_date<DATE_ADD(?,INTERVAL 1 DAY)", f.To)
	}
	if err := cq.Where("schedule_id IS NOT NULL").Distinct("schedule_id").Pluck("schedule_id", &confirmed).Error; err != nil {
		return nil, err
	}
	confirmedSet := map[uint]bool{}
	for _, id := range confirmed {
		confirmedSet[id] = true
	}
	rows := []reportTeaching{}
	rowIndex := map[uint]int{}
	classSet := map[uint]bool{}
	teacherSet := map[uint]bool{}
	courseSet := map[uint]bool{}
	studentSet := map[uint]bool{}
	for _, o := range offerings {
		rowIndex[o.ID] = len(rows)
		rows = append(rows, reportTeaching{ID: o.ID, Class: o.Class.ClassCode, Course: o.Course.Name, Semester: o.Semester.Name, Teacher: o.Teacher.User.FullName, Room: o.Room.Name})
		classSet[o.ClassID] = true
		if o.TeacherID > 0 {
			teacherSet[o.TeacherID] = true
		}
		courseSet[o.CourseID] = true
	}
	attendance := map[string]int{"present": 0, "absent": 0, "late": 0, "excused": 0}
	monthly := map[string]map[string]int{}
	byEnrollment := map[uint]map[string]int{}
	enrollmentIndex := map[uint]reportEnrollment{}
	scores := map[uint]float64{}
	approved, passed, draft := 0, 0, 0
	scoreSum := 0.0
	bands := []int{0, 0, 0, 0, 0}
	letters := map[string]int{}
	for _, e := range enrollments {
		studentSet[e.StudentID] = true
		enrollmentIndex[e.ID] = e
		r := &rows[rowIndex[e.OfferingID]]
		r.Students++
		if e.GradeStatus == "approved" {
			letters[letterGrade(e.Score)]++
			approved++
			r.Approved++
			scoreSum += e.Score
			scores[e.OfferingID] += e.Score
			bands[reportBand(e.Score)]++
			if e.Score >= 40 {
				passed++
				r.Passed++
			}
		} else if e.GradeID > 0 {
			draft++
			r.Draft++
		}
	}
	for i := range rows {
		if rows[i].Approved > 0 {
			avg := reportRound(scores[rows[i].ID] / float64(rows[i].Approved))
			rows[i].Average = &avg
		}
	}
	for _, a := range attendances {
		if _, ok := attendance[a.Status]; !ok {
			continue
		}
		attendance[a.Status]++
		month := a.ClassDate.Format("2006-01")
		if monthly[month] == nil {
			monthly[month] = map[string]int{"present": 0, "absent": 0, "late": 0, "excused": 0}
		}
		monthly[month][a.Status]++
		if byEnrollment[a.EnrollmentID] == nil {
			byEnrollment[a.EnrollmentID] = map[string]int{}
		}
		byEnrollment[a.EnrollmentID][a.Status]++
		e := enrollmentIndex[a.EnrollmentID]
		r := &rows[rowIndex[e.OfferingID]]
		switch a.Status {
		case "present":
			r.Present++
		case "absent":
			r.Absent++
		case "late":
			r.Late++
		case "excused":
			r.Excused++
		}
	}
	roomData := map[string]gin.H{}
	now := time.Now().In(attendanceLocation())
	elapsed, totalHours := 0, 0.0
	for _, s := range schedules {
		if s.CourseOfferingID == nil || s.FullDate == nil {
			continue
		}
		r := &rows[rowIndex[*s.CourseOfferingID]]
		r.Lessons++
		date := s.FullDate.Format("2006-01-02")
		if r.First == "" {
			r.First = date
		}
		r.Last = date
		if date < now.Format("2006-01-02") || (date == now.Format("2006-01-02") && s.EndTime <= now.Format("15:04")) {
			r.Elapsed++
			elapsed++
		}
		if confirmedSet[s.ID] {
			r.Confirmed++
		}
		start, e1 := time.Parse("15:04", s.StartTime[:min(5, len(s.StartTime))])
		end, e2 := time.Parse("15:04", s.EndTime[:min(5, len(s.EndTime))])
		hours := 0.0
		if e1 == nil && e2 == nil && end.After(start) {
			hours = end.Sub(start).Hours()
		}
		r.Hours = reportRound(r.Hours + hours)
		totalHours += hours
		if r.Room != "" {
			if roomData[r.Room] == nil {
				roomData[r.Room] = gin.H{"room": r.Room, "lessons": 0, "hours": float64(0)}
			}
			rd := roomData[r.Room]
			rd["lessons"] = rd["lessons"].(int) + 1
			rd["hours"] = reportRound(rd["hours"].(float64) + hours)
		}
	}
	months := []gin.H{}
	monthKeys := []string{}
	for month := range monthly {
		monthKeys = append(monthKeys, month)
	}
	sort.Strings(monthKeys)
	for _, month := range monthKeys {
		m := monthly[month]
		total := m["present"] + m["absent"] + m["late"] + m["excused"]
		months = append(months, gin.H{"month": month, "present": m["present"], "absent": m["absent"], "late": m["late"], "excused": m["excused"], "total": total, "attendanceRate": reportRound(float64(m["present"]+m["late"]) * 100 / float64(total))})
	}
	rooms := []gin.H{}
	if teacherID == 0 {
		var allRooms []models.Room
		if err := db.Find(&allRooms).Error; err != nil {
			return nil, err
		}
		for _, room := range allRooms {
			rd := roomData[room.Name]
			if rd == nil {
				rd = gin.H{"room": room.Name, "lessons": 0, "hours": 0}
			}
			rooms = append(rooms, rd)
		}
	}
	risks := []gin.H{}
	for _, e := range enrollments {
		a := byEnrollment[e.ID]
		total := a["present"] + a["absent"] + a["late"] + a["excused"]
		absenceRate := 0.0
		if total > 0 {
			absenceRate = reportRound(float64(a["absent"]+a["excused"]) * 100 / float64(total))
		}
		reasons := []string{}
		if total > 0 && absenceRate > 25 {
			reasons = append(reasons, "Vắng quá 25% số buổi đã chốt")
		}
		if e.GradeStatus == "approved" && e.Score < 40 {
			reasons = append(reasons, "Điểm môn đã duyệt dưới 40")
		}
		if len(reasons) > 0 {
			risks = append(risks, gin.H{"student": e.Name, "code": e.Code, "class": e.ClassCode, "course": e.CourseName, "absenceRate": absenceRate, "reason": strings.Join(reasons, "; ")})
		}
	}
	sort.Slice(risks, func(i, j int) bool { return risks[i]["code"].(string) < risks[j]["code"].(string) })
	avg := interface{}(nil)
	if approved > 0 {
		avg = reportRound(scoreSum / float64(approved))
	}
	result["totalStudents"] = len(studentSet)
	result["totalTeachers"] = len(teacherSet)
	result["totalClasses"] = len(classSet)
	result["totalCourses"] = len(courseSet)
	result["totalEnrollments"] = len(enrollments)
	result["totalWarnings"] = len(risks)
	result["attendance"] = attendance
	result["attendanceMonthly"] = months
	result["grades"] = gin.H{"totalGrades": approved + draft, "approved": approved, "draft": draft, "averageScore": avg, "passed": passed, "failed": approved - passed, "distribution": bands, "byLetter": letters}
	result["teaching"] = rows
	result["risks"] = risks
	result["rooms"] = rooms
	result["scheduleSummary"] = gin.H{"lessons": len(schedules), "elapsed": elapsed, "confirmed": len(confirmed), "hours": reportRound(totalHours)}
	if teacherID == 0 && len(f.IDs) == 0 && f.From == "" && f.To == "" {
		for key, model := range map[string]interface{}{"totalStudents": &models.Student{}, "totalTeachers": &models.Teacher{}, "totalClasses": &models.Class{}, "totalCourses": &models.Course{}} {
			var n int64
			if err := db.Model(model).Count(&n).Error; err != nil {
				return nil, err
			}
			result[key] = n
		}
	}
	// Comparison is only meaningful for the preceding semester of the same cohort.
	result["comparison"] = gin.H{"available": false, "message": "Chọn một học kỳ để so sánh với học kỳ trước của cùng khóa."}
	if f.From != "" || f.To != "" {
		result["comparison"] = gin.H{"available": false, "message": "Bỏ khoảng ngày và chọn một học kỳ để so sánh trọn kỳ với kỳ trước cùng khóa."}
	}
	if sid := f.IDs["semesterId"]; sid > 0 && f.From == "" && f.To == "" {
		var term models.Semester
		if err := db.First(&term, sid).Error; err == nil {
			parts := strings.Split(term.Name, " - ")
			if len(parts) == 2 {
				var number int
				fmt.Sscanf(parts[0], "HK%d", &number)
				if number == 1 {
					result["comparison"] = gin.H{"available": false, "message": "Đây là học kỳ đầu tiên của khóa, chưa có kỳ trước để so sánh."}
				}
				if number > 1 {
					var previous models.Semester
					if err := db.Where("name=?", fmt.Sprintf("HK%d - %s", number-1, parts[1])).First(&previous).Error; err == nil {
						pf := f
						pf.IDs = map[string]uint{}
						for k, v := range f.IDs {
							if k != "academicYearId" {
								pf.IDs[k] = v
							}
						}
						pf.IDs["semesterId"] = previous.ID
						var n int64
						if err := db.Table("enrollments e").Where("e.status<>? AND e.course_offering_id IN (?)", "cancelled", pf.scope(db, teacherID).Select("o.id")).Count(&n).Error; err != nil {
							return nil, err
						}
						result["comparison"] = gin.H{"available": true, "semester": previous.Name, "enrollments": n, "enrollmentChange": len(enrollments) - int(n)}
					}
				}
			}
		}
	}
	if teacherID == 0 {
		warnings := []struct {
			Student       string  `json:"student"`
			Code          string  `json:"code"`
			Class         string  `json:"class"`
			Semester      string  `json:"semester"`
			GPA           float64 `json:"gpa"`
			FailedCourses int     `json:"failedCourses"`
			FailedCredits int     `json:"failedCredits"`
			Reason        string  `json:"reason"`
			Status        string  `json:"status"`
		}{}
		wq := db.Table("academic_warnings w").Select("u.full_name AS student,s.student_code AS code,cl.class_code AS class,sm.name AS semester,w.gpa,w.failed_courses,w.failed_credits,w.reason,w.status").Joins("JOIN students s ON s.id=w.student_id").Joins("JOIN users u ON u.id=s.user_id").Joins("JOIN classes cl ON cl.id=s.class_id").Joins("JOIN semesters sm ON sm.id=w.semester_id")
		for key, column := range map[string]string{"academicYearId": "sm.academic_year_id", "semesterId": "w.semester_id", "classId": "s.class_id", "majorId": "cl.major_id", "cohort": "cl.cohort_year"} {
			if id := f.IDs[key]; id > 0 {
				wq = wq.Where(column+"=?", id)
			}
		}
		if f.IDs["courseId"] > 0 || f.From != "" || f.To != "" {
			wq = wq.Where("EXISTS (SELECT 1 FROM enrollments we JOIN course_offerings wo ON wo.id=we.course_offering_id WHERE we.student_id=w.student_id AND wo.semester_id=w.semester_id AND we.status<>? AND wo.id IN ?)", "cancelled", ids)
		}
		if err := wq.Order("s.student_code,w.semester_id").Scan(&warnings).Error; err != nil {
			return nil, err
		}
		result["academicWarnings"] = warnings
		activeWarnings := 0
		for _, w := range warnings {
			if strings.EqualFold(w.Status, "active") {
				activeWarnings++
			}
		}
		result["totalWarnings"] = activeWarnings
		extra, err := reportAssignments(db, f, offerings, schedules)
		if err != nil {
			return nil, err
		}
		result["assignments"] = extra
	}
	return result, nil
}

func reportAssignments(db *gorm.DB, f reportFilter, offerings []models.CourseOffering, schedules []models.Schedule) (gin.H, error) {
	q := db.Model(&models.Class{})
	for k, column := range map[string]string{"classId": "id", "majorId": "major_id", "cohort": "cohort_year"} {
		if id := f.IDs[k]; id > 0 {
			q = q.Where(column+"=?", id)
		}
	}
	var classes []models.Class
	if err := q.Find(&classes).Error; err != nil {
		return nil, err
	}
	assigned := map[uint]bool{}
	for _, o := range offerings {
		if o.TeacherID > 0 {
			assigned[o.ClassID] = true
		}
	}
	unassigned := []string{}
	for _, cl := range classes {
		if !assigned[cl.ID] {
			unassigned = append(unassigned, cl.ClassCode)
		}
	}
	var offers []models.ClassOffer
	oq := db.Preload("Class").Preload("Course").Preload("Teacher.User").Preload("Semester").Where("status IN ?", []string{"pending", "accepted"})
	for key, column := range map[string]string{"classId": "class_id", "courseId": "course_id", "semesterId": "semester_id"} {
		if id := f.IDs[key]; id > 0 {
			oq = oq.Where(column+"=?", id)
		}
	}
	if err := oq.Find(&offers).Error; err != nil {
		return nil, err
	}
	pending := []gin.H{}
	notice := []gin.H{}
	for _, o := range offers {
		if id := f.IDs["majorId"]; id > 0 && o.Class.MajorID != id {
			continue
		}
		if id := f.IDs["cohort"]; id > 0 && uint(o.Class.CohortYear) != id {
			continue
		}
		if id := f.IDs["academicYearId"]; id > 0 && (o.Semester.AcademicYearID == nil || *o.Semester.AcademicYearID != id) {
			continue
		}
		var plan struct {
			Schedules []models.Schedule `json:"schedules"`
			Urgent    bool              `json:"urgentException"`
			Reason    string            `json:"exceptionReason"`
		}
		if json.Unmarshal([]byte(o.PlanJSON), &plan) != nil {
			continue
		}
		first := ""
		matches := false
		for _, s := range plan.Schedules {
			if s.FullDate != nil {
				date := s.FullDate.Format("2006-01-02")
				if first == "" || date < first {
					first = date
				}
				if f.inDate(*s.FullDate) {
					matches = true
				}
			}
		}
		if !matches {
			continue
		}
		row := gin.H{"class": o.Class.ClassCode, "teacher": o.Teacher.User.FullName, "first": first, "urgent": plan.Urgent, "reason": plan.Reason, "status": o.Status}
		if o.CourseID != nil {
			row["course"] = o.Course.Name
		}
		if o.Status == "pending" {
			pending = append(pending, row)
		}
		if first != "" && first < o.OfferedAt.AddDate(0, 0, 7).Format("2006-01-02") {
			notice = append(notice, row)
		}
	}
	type slot struct {
		s models.Schedule
		o models.CourseOffering
	}
	lookup := map[uint]models.CourseOffering{}
	for _, o := range offerings {
		lookup[o.ID] = o
	}
	byDay := map[string][]slot{}
	conflicts := []gin.H{}
	for _, s := range schedules {
		if s.FullDate == nil || s.CourseOfferingID == nil {
			continue
		}
		day := s.FullDate.Format("2006-01-02")
		o := lookup[*s.CourseOfferingID]
		for _, other := range byDay[day] {
			if s.StartTime < other.s.EndTime && other.s.StartTime < s.EndTime {
				reason := ""
				if o.TeacherID > 0 && o.TeacherID == other.o.TeacherID {
					reason = "Giảng viên"
				} else if o.RoomID > 0 && o.RoomID == other.o.RoomID {
					reason = "Phòng học"
				} else if o.ClassID == other.o.ClassID {
					reason = "Lớp học"
				}
				if reason != "" {
					conflicts = append(conflicts, gin.H{"date": day, "first": other.o.Class.ClassCode + " · " + other.o.Course.Name, "second": o.Class.ClassCode + " · " + o.Course.Name, "reason": reason})
				}
			}
		}
		byDay[day] = append(byDay[day], slot{s, o})
	}
	return gin.H{"assignedClasses": len(assigned), "unassignedClasses": unassigned, "pending": pending, "shortNotice": notice, "conflicts": conflicts}, nil
}
