package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var warningReasonLabels = map[string]string{
	"unexcused":     "Nghỉ không phép",
	"absence_limit": "Nghỉ quá số buổi quy định",
	"failed_course": "Rớt môn",
}

type warningCandidate struct {
	EnrollmentID uint     `json:"enrollmentId"`
	StudentID    uint     `json:"studentId"`
	StudentCode  string   `json:"studentCode"`
	StudentName  string   `json:"studentName"`
	UserID       uint     `json:"-"`
	Email        string   `json:"-"`
	ClassID      uint     `json:"classId"`
	ClassCode    string   `json:"classCode"`
	CourseName   string   `json:"courseName"`
	SemesterName string   `json:"semesterName"`
	Unexcused    int      `json:"unexcused"`
	Missed       int      `json:"missed"`
	TotalLessons int      `json:"totalLessons"`
	Score        *float64 `json:"score"`
	Reasons      []string `json:"reasons"`
}

// Count the entire teaching plan, including future lessons, just as the class report does.
func warningLessonCount(offering *models.CourseOffering) int {
	if offering == nil {
		return 0
	}
	lessons := map[string]bool{}
	for _, schedule := range offering.Schedules {
		add := func(date time.Time) {
			lessons[date.Format("2006-01-02")+"|"+schedule.StartTime+"|"+schedule.EndTime+"|"+schedule.Session] = true
		}
		if schedule.FullDate != nil && !schedule.FullDate.IsZero() {
			add(*schedule.FullDate)
			continue
		}
		start, end := offering.Semester.StartDate, offering.Semester.EndDate
		if start.IsZero() || end.IsZero() {
			continue
		}
		for date, n := start, 0; !date.After(end) && n < 3660; date, n = date.AddDate(0, 0, 1), n+1 {
			if scheduleMatchesDate(schedule, date) {
				add(date)
			}
		}
	}
	return len(lessons)
}

func enrollmentWarning(e models.Enrollment, now time.Time) warningCandidate {
	r := warningCandidate{EnrollmentID: e.ID, StudentID: e.StudentID, StudentCode: e.Student.StudentCode, StudentName: e.Student.User.FullName, UserID: e.Student.UserID, Email: e.Student.User.Email, ClassID: e.ClassID, ClassCode: e.Class.ClassCode, CourseName: e.Course.Name, Reasons: []string{}}
	if e.CourseOffering != nil {
		r.SemesterName = e.CourseOffering.Semester.Name
	} else {
		r.SemesterName = e.Course.Semester.Name
	}
	r.TotalLessons = warningLessonCount(e.CourseOffering)
	for _, a := range e.Attendances {
		if a.ClassDate.After(now) {
			continue
		}
		if s := a.AttendanceSession; s != nil {
			if s.Status != "confirmed" {
				continue
			}
			if s.IsActive {
				if s.Schedule == nil {
					continue
				}
				_, end, err := attendanceLessonWindow(*s.Schedule, s.ClassDate)
				if err != nil || now.Before(end) {
					continue
				}
			}
		}
		if a.Status == "absent" {
			r.Unexcused++
			r.Missed++
		}
		if a.Status == "excused" {
			r.Missed++
		}
	}
	if r.Unexcused > 0 {
		r.Reasons = append(r.Reasons, "unexcused")
	}
	if r.TotalLessons > 0 && r.Missed*4 > r.TotalLessons {
		r.Reasons = append(r.Reasons, "absence_limit")
	}
	if e.Grade != nil && strings.EqualFold(e.Grade.Status, "approved") {
		score := e.Grade.TotalScore
		r.Score = &score
		if score < 40 {
			r.Reasons = append(r.Reasons, "failed_course")
		}
	}
	return r
}

func loadWarningCandidates(db *gorm.DB, semesterID, classID uint) ([]warningCandidate, error) {
	var enrollments []models.Enrollment
	q := db.Preload("Student.User").Preload("Class").Preload("Course.Semester").Preload("CourseOffering.Semester").Preload("CourseOffering.Schedules").Preload("Grade").Preload("Attendances.AttendanceSession.Schedule").
		Joins("JOIN courses ON courses.id = enrollments.course_id").
		Joins("LEFT JOIN course_offerings co ON co.id = enrollments.course_offering_id").
		Where("COALESCE(co.semester_id, courses.semester_id) = ? AND enrollments.status <> 'cancelled'", semesterID)
	if classID != 0 {
		q = q.Where("enrollments.class_id = ?", classID)
	}
	if err := q.Order("enrollments.student_id, enrollments.class_id, enrollments.id").Find(&enrollments).Error; err != nil {
		return nil, err
	}
	rows := []warningCandidate{}
	now := attendanceNow()
	for _, e := range enrollments {
		r := enrollmentWarning(e, now)
		if len(r.Reasons) > 0 {
			rows = append(rows, r)
		}
	}
	return rows, nil
}

func ListAcademicWarningCandidates(c *gin.Context) {
	semester, err := strconv.ParseUint(c.Query("semesterId"), 10, 32)
	if err != nil || semester == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Vui lòng chọn học kỳ"})
		return
	}
	var classID uint64
	if raw := c.Query("classId"); raw != "" {
		classID, err = strconv.ParseUint(raw, 10, 32)
		if err != nil || classID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Lớp không hợp lệ"})
			return
		}
	}
	rows, err := loadWarningCandidates(config.DB, uint(semester), uint(classID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tải được danh sách vi phạm"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func candidateHasReason(row warningCandidate, reason string) bool {
	if reason == "all" {
		return true
	}
	for _, code := range row.Reasons {
		if code == reason {
			return true
		}
	}
	return false
}

func warningDetail(row warningCandidate, reason string) string {
	parts := []string{}
	for _, code := range row.Reasons {
		if reason != "all" && code != reason {
			continue
		}
		switch code {
		case "unexcused":
			parts = append(parts, fmt.Sprintf("Nghỉ không phép: %d buổi", row.Unexcused))
		case "absence_limit":
			parts = append(parts, fmt.Sprintf("Nghỉ %d/%d buổi, vượt 25%% tổng số buổi (gồm nghỉ có phép và không phép)", row.Missed, row.TotalLessons))
		case "failed_course":
			parts = append(parts, fmt.Sprintf("Rớt môn: điểm đã duyệt %.2f/100 (dưới 40)", *row.Score))
		}
	}
	return fmt.Sprintf("- Lớp %s · Môn %s: %s.", row.ClassCode, row.CourseName, strings.Join(parts, "; "))
}

func SendAcademicWarnings(c *gin.Context) {
	adminID, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	var req struct {
		SemesterID    uint   `json:"semesterId" binding:"required"`
		ClassID       uint   `json:"classId"`
		Reason        string `json:"reason" binding:"required"`
		All           bool   `json:"all"`
		EnrollmentIDs []uint `json:"enrollmentIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu gửi cảnh báo không hợp lệ"})
		return
	}
	if _, valid := warningReasonLabels[req.Reason]; !valid && req.Reason != "all" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Lý do không hợp lệ"})
		return
	}
	if !req.All && len(req.EnrollmentIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Vui lòng chọn sinh viên"})
		return
	}
	rows, err := loadWarningCandidates(config.DB, req.SemesterID, req.ClassID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không kiểm tra được dữ liệu vi phạm"})
		return
	}
	selected := map[uint]bool{}
	for _, id := range req.EnrollmentIDs {
		selected[id] = true
	}
	groups := map[uint][]warningCandidate{}
	for _, row := range rows {
		if (req.All || selected[row.EnrollmentID]) && candidateHasReason(row, req.Reason) {
			groups[row.StudentID] = append(groups[row.StudentID], row)
			delete(selected, row.EnrollmentID)
		}
	}
	if !req.All && len(selected) != 0 {
		c.JSON(http.StatusConflict, gin.H{"message": "Dữ liệu vi phạm đã thay đổi. Vui lòng tải lại và chọn sinh viên."})
		return
	}
	if len(groups) == 0 || len(groups) > maxNotificationRecipients {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Không có sinh viên phù hợp hoặc vượt giới hạn 1.000 người nhận"})
		return
	}
	now := time.Now()
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		for _, violations := range groups {
			first := violations[0]
			var user models.User
			if err := tx.Where("id = ? AND is_active = ?", first.UserID, true).First(&user).Error; err != nil {
				return fmt.Errorf("Tài khoản sinh viên %s không hoạt động", first.StudentCode)
			}
			details := []string{}
			for _, row := range violations {
				details = append(details, warningDetail(row, req.Reason))
			}
			content := fmt.Sprintf("Chào %s (%s),\n\nPP Academy thông báo cảnh báo học vụ trong học kỳ %s:\n%s\n\nĐề nghị sinh viên kiểm tra kết quả học tập, chuyên cần và liên hệ giáo viên hoặc phòng đào tạo để được hướng dẫn khắc phục.\n\nTrân trọng,\nPhòng Đào tạo · PP Academy", first.StudentName, first.StudentCode, first.SemesterName, strings.Join(details, "\n"))
			notice := models.Notification{RecipientUserID: &first.UserID, RecipientEmail: first.Email, Subject: "Cảnh báo học vụ · " + first.SemesterName, Content: content, TargetType: "academic_warning", Channel: "app", Status: "sent", SentAt: &now, CreatedByID: &adminID}
			if err := tx.Create(&notice).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không gửi được cảnh báo. Chưa có thông báo nào được lưu."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": fmt.Sprintf("Đã gửi cảnh báo trong ứng dụng cho %d sinh viên", len(groups)), "count": len(groups)})
}
