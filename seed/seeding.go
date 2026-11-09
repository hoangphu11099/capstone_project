package seed

import (
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"os"
	"sort"
	"strings"
	"student-management/config"
	"student-management/models"
	"student-management/utils"
	"time"
)

type Options struct {
	Now      time.Time
	Password string
}
type Summary struct {
	Counts        map[string]int64 `json:"counts"`
	ReferenceDate string           `json:"referenceDate"`
}
type builder struct {
	db                  *gorm.DB
	err                 error
	now                 time.Time
	hash                string
	admin               models.User
	start, end, session string
}

func (b *builder) create(value interface{}) {
	if b.err == nil {
		if err := b.db.Create(value).Error; err != nil {
			b.err = fmt.Errorf("seed %T: %w", value, err)
		}
	}
}
func (b *builder) user(username, name string, role uint, first bool, email ...string) models.User {
	address := username + "@gmail.com"
	if len(email) > 0 {
		address = email[0]
	}
	u := models.User{Username: username, FullName: name, Email: address, Password: b.hash, RoleID: role, IsActive: true, FirstLogin: first}
	b.create(&u)
	// GORM's bool default would otherwise replace an explicit false with true.
	if b.err == nil {
		b.err = b.db.Model(&u).Update("first_login", first).Error
	}
	return u
}
func ptr[T any](v T) *T      { return &v }
func day(t time.Time) string { return t.Format("2006-01-02") }
func schoolDate(now time.Time) time.Time {
	loc := time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
	v := now.In(loc)
	return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, loc)
}
func termFor(date time.Time) (time.Time, time.Time, int, int) {
	y := date.Year()
	if date.Month() < 9 {
		y--
	}
	if date.Month() >= 9 || date.Month() == 1 {
		return time.Date(y, 9, 1, 0, 0, 0, 0, date.Location()), time.Date(y+1, 1, 31, 0, 0, 0, 0, date.Location()), y, 1
	}
	return time.Date(y+1, 2, 1, 0, 0, 0, 0, date.Location()), time.Date(y+1, 8, 31, 0, 0, 0, 0, date.Location()), y, 2
}
func weeklyDates(from, end time.Time, weekday time.Weekday, count int) []time.Time {
	dates := []time.Time{}
	for d := from; !d.After(end) && len(dates) < count; d = d.AddDate(0, 0, 1) {
		if d.Weekday() == weekday {
			dates = append(dates, d)
		}
	}
	return dates
}
func clockOn(date time.Time, clock string) time.Time {
	v, err := time.ParseInLocation("2006-01-02 15:04", day(date)+" "+clock, date.Location())
	if err != nil {
		panic(err)
	}
	return v
}

// SeedDatabase requires an empty, migrated database. It never overwrites live users.
func SeedDatabase(db *gorm.DB, options Options) error {
	if db == nil {
		return fmt.Errorf("database is not connected")
	}
	if options.Password == "" {
		options.Password = "123456"
	}
	if len(options.Password) < 6 || len(options.Password) > 72 {
		return fmt.Errorf("SEED_PASSWORD must contain 6 to 72 bytes")
	}
	if options.Now.IsZero() {
		options.Now = time.Now()
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(options.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.User{}).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("database is not empty; use the backup/reset command")
		}
		b := builder{db: tx, now: schoolDate(options.Now), hash: string(hash)}
		roles := map[string]uint{}
		for _, name := range []string{"admin", "teacher", "student"} {
			r := models.Role{Name: name, Description: "Tài khoản mẫu " + name}
			b.create(&r)
			roles[name] = r.ID
		}
		b.admin = b.user("admin", "Quản trị viên PP Academy", roles["admin"], false)
		b.schoolDataset(roles)
		if b.err == nil {
			b.err = NormalizeCohortSemesters(tx)
		}
		if b.err == nil {
			b.err = ConfigureK26Demo(tx, b.now)
		}
		b.publicWebsite()
		if b.err != nil {
			return b.err
		}
		_, err := Validate(tx, b.now)
		return err
	})
}

// k26Weekdays spreads classes across recurring two/three-day weekly patterns.
func k26Weekdays(index int) []time.Weekday {
	return [][]time.Weekday{
		{time.Monday, time.Wednesday},
		{time.Monday, time.Wednesday, time.Friday},
		{time.Tuesday, time.Thursday, time.Saturday},
		{time.Tuesday, time.Thursday},
	}[index/2%2+2*(index%2)]
}

func k26Dates(start, end, from time.Time, index int) map[int][]time.Time {
	grouped := map[int][]time.Time{}
	anchor := start.AddDate(0, 0, -(int(start.Weekday())+6)%7)
	for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
		if date.Before(from) {
			continue
		}
		for _, weekday := range k26Weekdays(index) {
			if date.Weekday() == weekday {
				week := int(date.Sub(anchor).Hours()/24) / 7
				grouped[week] = append(grouped[week], date)
				break
			}
		}
	}
	return grouped
}

// ConfigureK26Demo prepares recurring afternoon lessons through the fourth semester.
// It preserves past lessons and refuses to replace lessons that already have attendance sessions.
// NormalizeCohortSemesters separates calendar periods into four semesters per cohort.
func NormalizeCohortSemesters(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var oldTerms []models.Semester
		if err := tx.Where("name IN ?", []string{"HK1", "HK2", "HK3", "HK4", "HK5", "HK6", "HK7", "HK8"}).Find(&oldTerms).Error; err != nil {
			return err
		}
		for cohort := 2024; cohort <= 2026; cohort++ {
			for part := 0; part < 4; part++ {
				start := time.Date(cohort, 9, 1, 0, 0, 0, 0, schoolDate(time.Now()).Location()).AddDate(0, part*6, 0)
				var source models.Semester
				if err := tx.Where("start_date = ?", start).First(&source).Error; err != nil {
					return err
				}
				name := fmt.Sprintf("HK%d - K%d", part+1, cohort%100)
				target := models.Semester{Name: name, AcademicYearID: source.AcademicYearID, StartDate: start, EndDate: start.AddDate(0, 6, -1), Status: source.Status}
				if err := tx.Where("name = ?", name).FirstOrCreate(&target).Error; err != nil {
					return err
				}
				for _, old := range oldTerms {
					if !old.StartDate.Equal(start) {
						continue
					}
					for _, table := range []string{"course_offerings", "class_offers", "exam_schedules"} {
						query := "UPDATE " + table + " x JOIN classes c ON c.id=x.class_id SET x.semester_id=? WHERE c.cohort_year=? AND x.semester_id=?"
						if err := tx.Exec(query, target.ID, cohort, old.ID).Error; err != nil {
							return err
						}
					}
					for _, table := range []string{"transcripts", "academic_warnings"} {
						query := "UPDATE " + table + " x JOIN students s ON s.id=x.student_id JOIN classes c ON c.id=s.class_id SET x.semester_id=? WHERE c.cohort_year=? AND x.semester_id=?"
						if err := tx.Exec(query, target.ID, cohort, old.ID).Error; err != nil {
							return err
						}
					}
					if err := tx.Model(&models.Class{}).Where("cohort_year=? AND semester_id=?", cohort, old.ID).Update("semester_id", target.ID).Error; err != nil {
						return err
					}
					if cohort == 2024 {
						if err := tx.Model(&models.Course{}).Where("semester_id=?", old.ID).Update("semester_id", target.ID).Error; err != nil {
							return err
						}
					}
				}
			}
		}
		var offers []models.ClassOffer
		if err := tx.Find(&offers).Error; err != nil {
			return err
		}
		for _, offer := range offers {
			if offer.SemesterID == nil {
				continue
			}
			var plan map[string]interface{}
			if err := json.Unmarshal([]byte(offer.PlanJSON), &plan); err != nil {
				return err
			}
			if assignment, ok := plan["assignment"].(map[string]interface{}); ok {
				assignment["semesterId"] = *offer.SemesterID
			}
			encoded, err := json.Marshal(plan)
			if err != nil {
				return err
			}
			if err := tx.Model(&offer).Update("plan_json", string(encoded)).Error; err != nil {
				return err
			}
		}
		for _, old := range oldTerms {
			if err := tx.Delete(&models.Semester{}, old.ID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func ConfigureK26Demo(db *gorm.DB, now time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		from := schoolDate(now)
		for from.Weekday() != time.Wednesday {
			from = from.AddDate(0, 0, 1)
		}
		until := time.Date(2028, 8, 31, 0, 0, 0, 0, from.Location())
		if from.After(until) {
			return fmt.Errorf("K26 demo period has ended")
		}
		var classes []models.Class
		if err := tx.Where("cohort_year = ?", 2026).Order("class_code").Find(&classes).Error; err != nil {
			return err
		}
		if len(classes) != 8 {
			return fmt.Errorf("K26 demo requires 8 classes")
		}
		var rooms []models.Room
		if err := tx.Where(`NOT EXISTS (SELECT 1 FROM schedules s
			JOIN course_offerings o ON o.id=s.course_offering_id JOIN classes c ON c.id=s.class_id
			WHERE o.room_id=rooms.id AND c.cohort_year<>2026 AND s.full_date>=? AND s.full_date<=?
			AND s.start_time<'16:30' AND s.end_time>'13:30')`, from, until).Order("name").Limit(8).Find(&rooms).Error; err != nil {
			return err
		}
		if len(rooms) != 8 {
			return fmt.Errorf("K26 demo requires 8 rooms")
		}
		var admin models.User
		if err := tx.Where("username = ?", "admin").First(&admin).Error; err != nil {
			return err
		}
		for index, class := range classes {
			var sessions int64
			if err := tx.Model(&models.AttendanceSession{}).Where("class_id = ? AND class_date >= ? AND class_date <= ?", class.ID, schoolDate(now), until).Count(&sessions).Error; err != nil {
				return err
			}
			if sessions > 0 {
				return fmt.Errorf("class %s already has attendance sessions in demo period", class.ClassCode)
			}
			if err := tx.Model(&models.ClassOffer{}).Where("class_id = ?", class.ID).Update("plan_json", gorm.Expr("REPLACE(REPLACE(plan_json, ?, ?), ?, ?)", `"12:00"`, `"13:30"`, `"13:00"`, `"16:30"`)).Error; err != nil {
				return err
			}
			if err := tx.Where("class_id = ? AND full_date >= ? AND full_date <= ?", class.ID, schoolDate(now), until).Delete(&models.Schedule{}).Error; err != nil {
				return err
			}
			var teachers []models.Teacher
			if err := tx.Where("major_id = ?", class.MajorID).Order("teacher_code").Find(&teachers).Error; err != nil {
				return err
			}
			if len(teachers) < 2 {
				return fmt.Errorf("missing K26 teachers")
			}
			teacher := teachers[index%2]
			var courses []models.Course
			if err := tx.Where("major_id = ?", class.MajorID).Order("code").Find(&courses).Error; err != nil {
				return err
			}
			if len(courses) != 30 {
				return fmt.Errorf("missing K26 curriculum")
			}
			var students []models.Student
			if err := tx.Where("class_id = ?", class.ID).Find(&students).Error; err != nil {
				return err
			}
			for part := 0; part < 4; part++ {
				start := time.Date(2026, 9, 1, 0, 0, 0, 0, from.Location()).AddDate(0, part*6, 0)
				end := start.AddDate(0, 6, -1)
				var term models.Semester
				if err := tx.Where("name = ? AND start_date = ? AND end_date = ?", fmt.Sprintf("HK%d - K26", part+1), start, end).First(&term).Error; err != nil {
					return err
				}
				grouped := map[int][]time.Time{}
				for week, dates := range k26Dates(start, end, from, index) {
					courseIndex := week / 3
					count := []int{8, 8, 7, 7}[part]
					if courseIndex >= count {
						courseIndex = count - 1
					}
					courseIndex += []int{0, 8, 16, 23}[part]
					grouped[courseIndex] = append(grouped[courseIndex], dates...)
				}
				for j := range grouped {
					sort.Slice(grouped[j], func(a, b int) bool { return grouped[j][a].Before(grouped[j][b]) })
				}
				for j := 0; j < len(courses); j++ {
					dates := grouped[j]
					if len(dates) == 0 {
						continue
					}
					course := courses[j]
					var offering models.CourseOffering
					result := tx.Where("class_id = ? AND course_id = ? AND semester_id = ?", class.ID, course.ID, term.ID).Limit(1).Find(&offering)
					if result.Error != nil {
						return result.Error
					}
					if offering.ID == 0 {
						b := builder{db: tx, now: from, admin: admin, start: "13:30", end: "16:30", session: "Chiều"}
						b.offering(class, course, term, teacher, rooms[index], students, dates, false, false)
						if b.err != nil {
							return b.err
						}
					} else {
						if err := tx.Model(&offering).Updates(map[string]interface{}{"teacher_id": teacher.ID, "room_id": rooms[index].ID, "status": "open"}).Error; err != nil {
							return err
						}
						for _, date := range dates {
							schedule := models.Schedule{ClassID: class.ID, CourseOfferingID: ptr(offering.ID), FullDate: ptr(date), DayOfWeek: date.Format("Mon"), StartTime: "13:30", EndTime: "16:30", Session: "Chiều"}
							if err := tx.Create(&schedule).Error; err != nil {
								return err
							}
						}
					}
					var accepted models.ClassOffer
					if err := tx.Where("class_id = ? AND course_id = ? AND semester_id = ? AND status = ?", class.ID, course.ID, term.ID, "accepted").Order("id DESC").First(&accepted).Error; err != nil {
						return err
					}
					if offering.ID == 0 {
						if err := tx.Where("class_id=? AND course_id=? AND semester_id=?", class.ID, course.ID, term.ID).First(&offering).Error; err != nil {
							return err
						}
					}
					var actual []models.Schedule
					if err := tx.Where("course_offering_id=?", offering.ID).Order("full_date").Find(&actual).Error; err != nil {
						return err
					}
					teachingDates := []string{}
					for _, lesson := range actual {
						teachingDates = append(teachingDates, day(*lesson.FullDate))
					}
					encoded, err := json.Marshal(map[string]interface{}{"assignment": map[string]interface{}{"classId": class.ID, "courseId": course.ID, "semesterId": term.ID, "teacherId": teacher.ID, "roomId": rooms[index].ID, "teachingDates": teachingDates, "startTime": "13:30", "endTime": "16:30", "session": "Chiều"}, "schedules": actual, "baseVersion": "new"})
					if err != nil {
						return err
					}
					accepted.PlanJSON = string(encoded)
					if err := tx.Model(&models.ClassOffer{}).Where("class_id=? AND course_id=? AND semester_id=? AND status=?", class.ID, course.ID, term.ID, "accepted").Updates(map[string]interface{}{"plan_json": accepted.PlanJSON, "teacher_id": teacher.ID, "room_id": rooms[index].ID}).Error; err != nil {
						return err
					}
					if err := tx.Model(&models.ClassOffer{}).Where("class_id = ? AND course_id = ? AND semester_id = ? AND status = ?", class.ID, course.ID, term.ID, "pending").Updates(map[string]interface{}{"status": "accepted", "plan_json": accepted.PlanJSON, "offered_at": accepted.OfferedAt, "responded_at": accepted.RespondedAt, "response_note": "Dữ liệu demo điểm danh K26 đã xác nhận"}).Error; err != nil {
						return err
					}
				}
			}
		}
		_, err := Validate(tx, now)
		return err
	})
}

func (b *builder) offer(class models.Class, course models.Course, term models.Semester, teacher models.Teacher, room models.Room, dates []time.Time, status string, urgent bool, note string) {
	if len(dates) == 0 {
		b.err = fmt.Errorf("no dates for offer %s", class.ClassCode)
		return
	}
	schedules := []models.Schedule{}
	dateStrings := []string{}
	start, end, session := b.start, b.end, b.session
	if urgent {
		start, end, session = "18:00", "21:00", "Tối"
	}
	for _, date := range dates {
		dateStrings = append(dateStrings, day(date))
		schedules = append(schedules, models.Schedule{ClassID: class.ID, FullDate: ptr(date), DayOfWeek: date.Format("Mon"), StartTime: start, EndTime: end, Session: session})
	}
	plan := map[string]interface{}{"assignment": map[string]interface{}{"courseId": course.ID, "semesterId": term.ID, "teacherId": teacher.ID, "roomId": room.ID, "teachingDates": dateStrings, "startTime": start, "endTime": end, "session": session}, "schedules": schedules, "baseVersion": "new"}
	if urgent {
		plan["urgentException"] = true
		plan["exceptionReason"] = note
	}
	data, err := json.Marshal(plan)
	if err != nil {
		b.err = err
		return
	}
	content := fmt.Sprintf("Kính gửi Thầy/Cô %s,\n\nBộ phận Đào tạo PP Academy trân trọng kính mời Thầy/Cô giảng dạy môn %s cho lớp %s, học kỳ %s. Kính đề nghị Thầy/Cô xem xét kế hoạch kèm theo và phản hồi trên hệ thống. Lịch chỉ được xác nhận khi Thầy/Cô chấp nhận lời mời.\n\nTrân trọng,\nBộ phận Đào tạo PP Academy", teacherName(b.db, teacher.UserID), course.Name, class.ClassCode, term.Name)
	row := models.ClassOffer{ClassID: class.ID, TeacherID: teacher.ID, CourseID: ptr(course.ID), SemesterID: ptr(term.ID), RoomID: ptr(room.ID), PlanJSON: string(data), OfferedByUserID: b.admin.ID, Status: status, Title: "Thư mời giảng dạy – " + course.Name, Content: content, Message: content, OfferedAt: b.now, ResponseNote: note}
	if status == "rejected" {
		row.OfferedAt = b.now.AddDate(0, 0, -2)
		row.RespondedAt = ptr(b.now.AddDate(0, 0, -1))
	}
	if !urgent && dates[0].Before(row.OfferedAt.AddDate(0, 0, 7)) {
		row.OfferedAt = dates[0].AddDate(0, 0, -14)
	}
	b.create(&row)
}
func teacherName(db *gorm.DB, userID uint) string {
	var u models.User
	db.First(&u, userID)
	return u.FullName
}

func (b *builder) offering(class models.Class, course models.Course, term models.Semester, teacher models.Teacher, room models.Room, students []models.Student, dates []time.Time, archived, registrationDemo bool) {
	if len(dates) == 0 {
		return
	}
	status := "open"
	if archived {
		status = "closed"
	}
	o := models.CourseOffering{ClassID: class.ID, CourseID: course.ID, SemesterID: term.ID, TeacherID: teacher.ID, RoomID: room.ID, Status: status}
	b.create(&o)
	schedules := []models.Schedule{}
	for _, date := range dates {
		row := models.Schedule{ClassID: class.ID, CourseOfferingID: ptr(o.ID), FullDate: ptr(date), DayOfWeek: date.Format("Mon"), Session: b.session, StartTime: b.start, EndTime: b.end}

		b.create(&row)
		schedules = append(schedules, row)
	}
	// Accepted plans reflect the original invitation, not a fabricated pending offer.
	originals := []models.Schedule{}
	dateStrings := []string{}
	for _, row := range schedules {
		original := row
		if original.OriginalDate != nil {
			original.FullDate = original.OriginalDate
			original.DayOfWeek = original.FullDate.Format("Mon")
		}
		originals = append(originals, original)
		dateStrings = append(dateStrings, day(*original.FullDate))
	}
	plan, _ := json.Marshal(map[string]interface{}{"assignment": map[string]interface{}{"courseId": course.ID, "semesterId": term.ID, "teacherId": teacher.ID, "roomId": room.ID, "teachingDates": dateStrings, "startTime": b.start, "endTime": b.end, "session": b.session}, "schedules": originals, "baseVersion": "new"})
	offered := dates[0].AddDate(0, 0, -14)
	accepted := offered.AddDate(0, 0, 1)
	text := fmt.Sprintf("Kính gửi Quý Thầy/Cô,\n\nPP Academy trân trọng kính mời Thầy/Cô phụ trách môn %s cho lớp %s theo kế hoạch kèm theo.\n\nTrân trọng,\nBộ phận Đào tạo PP Academy", course.Name, class.ClassCode)
	b.create(&models.ClassOffer{ClassID: class.ID, TeacherID: teacher.ID, CourseID: ptr(course.ID), SemesterID: ptr(term.ID), RoomID: ptr(room.ID), PlanJSON: string(plan), OfferedByUserID: b.admin.ID, Status: "accepted", Title: "Thư mời giảng dạy – " + course.Name, Content: text, Message: text, OfferedAt: offered, RespondedAt: &accepted, ResponseNote: "Đồng ý với kế hoạch giảng dạy."})
	enrollments := []models.Enrollment{}
	for i, student := range students {
		registered := dates[0].AddDate(0, 0, -7)
		if registrationDemo && i >= 10 {
			state := "pending_payment"
			r := models.CourseRegistration{CourseOfferingID: ptr(o.ID), StudentID: student.ID, CourseID: course.ID, ClassID: class.ID, Status: state, RegisteredAt: registered, Note: "Đăng ký học phần mẫu"}
			if i == 11 {
				r.Status = "paid"
				r.PaymentConfirmedAt = ptr(b.now.AddDate(0, 0, -1))
				r.PaymentConfirmedBy = ptr(b.admin.ID)
			}
			b.create(&r)
			continue
		}
		e := models.Enrollment{CourseOfferingID: ptr(o.ID), StudentID: student.ID, ClassID: class.ID, CourseID: course.ID, EnrollDate: registered, Status: "enrolled"}
		b.create(&e)
		enrollments = append(enrollments, e)
		b.create(&models.CourseRegistration{CourseOfferingID: ptr(o.ID), StudentID: student.ID, CourseID: course.ID, ClassID: class.ID, EnrollmentID: ptr(e.ID), Status: "scheduled", RegisteredAt: registered, PaymentConfirmedAt: ptr(registered), PaymentConfirmedBy: ptr(b.admin.ID), ScheduledAt: ptr(registered), Note: "Đã xác nhận học phí và xếp lịch"})
		if archived {
			score := float64(70 + i%5*4)
			if i == 0 {
				score = 35
			}
			letter := letterForScore(score)
			g := models.Grade{EnrollmentID: e.ID, AssignmentScore: score, MidtermScore: score, FinalScore: score, TotalScore: score, GradeLetter: letter, Status: "approved", Score: score, LetterGrade: letter, Remark: "Điểm học kỳ đã hoàn tất"}
			b.create(&g)
			b.create(&models.GradeApproval{GradeID: g.ID, TeacherID: teacher.ID, Status: "approved", ApprovedAt: term.EndDate})
			b.create(&models.Transcript{StudentID: student.ID, CourseID: course.ID, EnrollmentID: e.ID, GradeID: g.ID, SemesterID: term.ID, AssignmentScore: score, MidtermScore: score, FinalScore: score, TotalScore: score, GradeLetter: letter, Status: "approved"})
			if i == 0 {
				b.create(&models.AcademicWarning{StudentID: student.ID, SemesterID: term.ID, GPA: 0, FailedCourses: 1, FailedCredits: course.Credits, Reason: "Điểm trung bình học kỳ thấp; có môn chưa đạt", Status: "active"})
			}
		} else if !registrationDemo && dates[0].Before(b.now) {
			b.create(&models.Grade{EnrollmentID: e.ID, AssignmentScore: float64(65 + i%5*5), MidtermScore: 0, FinalScore: 0, Status: "draft", Remark: "Đang học; chưa có điểm cuối kỳ"})
		}
	}
	for _, schedule := range schedules {
		if !schedule.FullDate.Before(b.now) {
			continue
		}
		date := *schedule.FullDate
		session := models.AttendanceSession{CourseOfferingID: ptr(o.ID), ScheduleID: ptr(schedule.ID), ClassID: class.ID, CourseID: course.ID, TeacherID: teacher.ID, Code: fmt.Sprintf("seed-expired-%d", schedule.ID), ClassDate: date, ExpiresAt: clockOn(date, b.end).Add(time.Minute), Status: "confirmed", IsActive: false, Note: "Phiên điểm danh mẫu đã chốt"}
		b.create(&session)
		// GORM's default:true replaces false during Create; close historical sessions explicitly.
		if b.err == nil {
			b.err = b.db.Model(&session).Update("is_active", false).Error
		}
		for i, e := range enrollments {
			state, note := "present", "Có mặt đúng giờ"
			if i == 0 {
				state, note = "absent", "Vắng không phép"
			} else if i == 1 {
				state, note = "late", "Đi trễ"
			} else if i == 2 {
				state, note = "excused", "Vắng có phép"
			}
			a := models.Attendance{EnrollmentID: e.ID, AttendanceSessionID: ptr(session.ID), ClassDate: date, Status: state, Note: note}
			if state == "present" {
				a.CheckedInAt = ptr(clockOn(date, b.start))
			}
			if state == "late" {
				a.CheckedInAt = ptr(clockOn(date, b.start).Add(20 * time.Minute))
			}
			b.create(&a)
		}
	}
}

func letterForScore(score float64) string {
	for _, v := range []struct {
		min    float64
		letter string
	}{{85, "A"}, {80, "B+"}, {70, "B"}, {65, "C+"}, {55, "C"}, {50, "D+"}, {40, "D"}} {
		if score >= v.min {
			return v.letter
		}
	}
	return "F"
}
func (b *builder) publicWebsite() {
	b.create(&models.SiteSetting{ID: 1, SchoolName: "PP Academy", Slogan: "Học qua dự án, phát triển năng lực nghề nghiệp", Introduction: "PP Academy cung cấp môi trường học tập thực hành với các chương trình công nghệ, sáng tạo và kinh doanh. Dữ liệu trên hệ thống này được khởi tạo để minh họa các quy trình đào tạo.", HeroImageURL: "/pp-academy-campus.webp", LogoURL: "/pp-academy-logo.png", DownloadButtonLabel: "Tải ứng dụng", DownloadEnabled: false, ContactEmail: "daotao@gmail.com", Phone: "028 7300 6868", Address: "Thành phố Hồ Chí Minh"})
	for i, title := range []string{"Hướng dẫn sử dụng cổng học tập PP Academy", "Quy trình xác nhận lịch giảng dạy và học bù", "Đăng ký học phần và xác nhận học phí"} {
		b.create(&models.SchoolPost{Title: title, Slug: fmt.Sprintf("huong-dan-hoc-tap-%d", i+1), Category: []string{"news", "announcement", "announcement"}[i], Excerpt: "Thông tin hướng dẫn dành cho sinh viên và giảng viên.", Content: title + "\n\nSinh viên và giảng viên xem lịch học, bài tập và thông báo trên hệ thống. Khi cần hỗ trợ, vui lòng liên hệ Bộ phận Đào tạo.\n\nNội dung mẫu phục vụ kiểm thử dự án.", CoverImageURL: []string{"/pp-academy-tech-lab.webp", "/pp-academy-library.webp", "/pp-academy-campus.webp"}[i], AuthorName: "Bộ phận Đào tạo", IsPublished: true, IsFeatured: i == 0, PublishedAt: ptr(b.now.AddDate(0, 0, -i-1))})
	}
	b.create(&models.Consultation{FullName: "Nguyễn Hoàng Nam", Email: "tuvan.ppacademy@gmail.com", Phone: "0901234567", Program: "Lập trình web", Message: "Tôi muốn tìm hiểu chương trình học và thời gian tuyển sinh.", Status: "new", ConsentAt: b.now})
}

// Validate checks business invariants, not just row counts, before replacement.
func Validate(db *gorm.DB, now time.Time) (Summary, error) {
	result := Summary{Counts: map[string]int64{}, ReferenceDate: day(schoolDate(now))}
	for _, table := range []string{"roles", "users", "teachers", "students", "majors", "academic_years", "semesters", "classes", "rooms", "courses", "course_offerings", "schedules", "enrollments", "course_registrations", "class_offers", "attendance_sessions", "attendances", "grades", "transcripts", "exercises", "submissions", "auth_sessions", "exam_schedules", "academic_warnings", "school_posts"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			return result, err
		}
		result.Counts[table] = count
	}
	checks := []struct{ name, query string }{
		{"lịch không có ngày cụ thể", "SELECT COUNT(*) FROM schedules WHERE full_date IS NULL"},
		{"lịch ngoài học kỳ", "SELECT COUNT(*) FROM schedules s JOIN course_offerings o ON o.id=s.course_offering_id JOIN semesters t ON t.id=o.semester_id WHERE s.full_date<t.start_date OR s.full_date>t.end_date"},
		{"giảng viên hoặc môn sai ngành", "SELECT COUNT(*) FROM course_offerings o JOIN classes c ON c.id=o.class_id JOIN teachers t ON t.id=o.teacher_id JOIN courses k ON k.id=o.course_id WHERE t.major_id<>c.major_id OR k.major_id<>c.major_id"},
		{"đăng ký không khớp học phần", "SELECT COUNT(*) FROM enrollments e JOIN course_offerings o ON o.id=e.course_offering_id WHERE e.class_id<>o.class_id OR e.course_id<>o.course_id"},
		{"đăng ký đã xếp lịch thiếu quan hệ", "SELECT COUNT(*) FROM course_registrations r LEFT JOIN enrollments e ON e.id=r.enrollment_id WHERE r.status='scheduled' AND (e.id IS NULL OR r.course_offering_id<>e.course_offering_id OR r.payment_confirmed_at IS NULL)"},
		{"trùng lịch lớp, phòng hoặc giảng viên", "SELECT COUNT(*) FROM schedules a JOIN schedules b ON a.id<b.id AND a.full_date=b.full_date AND a.start_time<b.end_time AND b.start_time<a.end_time JOIN course_offerings x ON x.id=a.course_offering_id JOIN course_offerings y ON y.id=b.course_offering_id WHERE (x.class_id=y.class_id OR x.teacher_id=y.teacher_id OR x.room_id=y.room_id)"},
		{"vượt sĩ số lớp", "SELECT COUNT(*) FROM (SELECT c.id FROM classes c JOIN students s ON s.class_id=c.id GROUP BY c.id,c.max_students HAVING COUNT(*)>c.max_students) t"},
	}
	for _, check := range checks {
		var count int64
		if err := db.Raw(check.query).Scan(&count).Error; err != nil {
			return result, err
		}
		if count > 0 {
			return result, fmt.Errorf("seed không hợp lệ: %s (%d)", check.name, count)
		}
	}
	var invalidSessions int64
	if err := db.Model(&models.AttendanceSession{}).Where("class_date >= ? OR is_active = ?", schoolDate(now), true).Count(&invalidSessions).Error; err != nil {
		return result, err
	}
	if invalidSessions > 0 {
		return result, fmt.Errorf("seed contains active or future attendance sessions")
	}
	var offers []models.ClassOffer
	if err := db.Find(&offers).Error; err != nil {
		return result, err
	}
	for _, offer := range offers {
		var plan struct {
			Schedules []models.Schedule `json:"schedules"`
			Urgent    bool              `json:"urgentException"`
			Reason    string            `json:"exceptionReason"`
		}
		if json.Unmarshal([]byte(offer.PlanJSON), &plan) != nil || len(plan.Schedules) == 0 {
			return result, fmt.Errorf("offer %d lacks a valid plan", offer.ID)
		}
		for _, s := range plan.Schedules {
			if s.FullDate == nil {
				return result, fmt.Errorf("offer %d has recurring schedule", offer.ID)
			}
			if !plan.Urgent && day(*s.FullDate) < day(offer.OfferedAt.AddDate(0, 0, 7)) {
				return result, fmt.Errorf("offer %d violates seven-day notice", offer.ID)
			}
		}
		if plan.Urgent && strings.TrimSpace(plan.Reason) == "" {
			return result, fmt.Errorf("urgent offer lacks reason")
		}
	}
	if result.Counts["auth_sessions"] != 0 {
		return result, fmt.Errorf("seed must not contain login sessions")
	}
	return result, nil
}

// SeedData only fills an empty database; it never resets or overwrites accounts.
// Use cmd/dbreset for an explicit reset with a verified SQL backup.
func SeedData() error {
	return SeedDatabase(config.DB, Options{Now: time.Now(), Password: os.Getenv("SEED_PASSWORD")})
}

// Curriculum order follows four semesters: 8, 8, 7 and 7 courses.
var curricula = map[string][]string{
	"WEB": {
		"Nhập môn công nghệ thông tin", "Tư duy lập trình", "Toán ứng dụng", "HTML và CSS", "Thiết kế web căn bản", "Cơ sở dữ liệu căn bản", "Tiếng Anh chuyên ngành 1", "Kỹ năng học tập và làm việc nhóm",
		"JavaScript căn bản", "JavaScript nâng cao", "Lập trình hướng đối tượng", "Thiết kế cơ sở dữ liệu", "Git và quản lý mã nguồn", "Thiết kế giao diện người dùng", "Mạng máy tính căn bản", "Tiếng Anh chuyên ngành 2",
		"React và TypeScript", "Lập trình API với Go", "Lập trình PHP", "Kiểm thử phần mềm", "Bảo mật ứng dụng web", "Phân tích thiết kế hệ thống", "Dự án web thực tế",
		"Phát triển ứng dụng di động", "Điện toán đám mây", "DevOps căn bản", "Tối ưu hiệu năng web", "Quản lý dự án phần mềm", "Thực tập doanh nghiệp", "Đồ án tốt nghiệp web",
	},
	"GAME": {
		"Nhập môn phát triển game", "Tư duy lập trình game", "Toán cho lập trình game", "Lập trình C# căn bản", "Mỹ thuật game căn bản", "Thiết kế ý tưởng game", "Tiếng Anh chuyên ngành game 1", "Kỹ năng làm việc nhóm",
		"Lập trình C# nâng cao", "Unity căn bản", "Thiết kế gameplay", "Thiết kế màn chơi", "Đồ họa game 2D", "Vật lý trong game", "Git cho dự án game", "Tiếng Anh chuyên ngành game 2",
		"Phát triển game 2D", "Phát triển game 3D", "Trí tuệ nhân tạo trong game", "Hoạt hình nhân vật", "Âm thanh trong game", "Kiểm thử game", "Dự án game thực tế",
		"Game trên thiết bị di động", "Lập trình game trực tuyến", "Tối ưu hiệu năng game", "Thiết kế trải nghiệm người chơi", "Phát hành và kinh doanh game", "Thực tập doanh nghiệp game", "Đồ án tốt nghiệp game",
	},
	"MKT": {
		"Marketing căn bản", "Kinh tế học căn bản", "Hành vi khách hàng", "Nguyên lý truyền thông", "Tin học ứng dụng Marketing", "Kỹ năng viết nội dung", "Tiếng Anh Marketing 1", "Kỹ năng thuyết trình",
		"Digital Marketing", "Content Marketing", "Nghiên cứu thị trường", "Quản trị thương hiệu", "Thiết kế nội dung số", "Marketing mạng xã hội", "Thống kê kinh doanh", "Tiếng Anh Marketing 2",
		"Tối ưu công cụ tìm kiếm", "Quảng cáo Google Ads", "Quảng cáo mạng xã hội", "Email Marketing", "Phân tích dữ liệu Marketing", "Quan hệ công chúng", "Dự án chiến dịch Marketing",
		"Thương mại điện tử", "Marketing tự động hóa", "Quản trị quan hệ khách hàng", "Lập kế hoạch truyền thông", "Quản lý ngân sách Marketing", "Thực tập doanh nghiệp Marketing", "Đồ án tốt nghiệp Marketing",
	},
	"DESIGN": {
		"Nguyên lý thiết kế", "Hình họa căn bản", "Lý thuyết màu sắc", "Bố cục thị giác", "Lịch sử mỹ thuật", "Nhiếp ảnh căn bản", "Tiếng Anh thiết kế 1", "Tư duy sáng tạo",
		"Adobe Photoshop", "Adobe Illustrator", "Typography", "Vẽ minh họa số", "Thiết kế ấn phẩm", "Xử lý hình ảnh", "Kỹ thuật in ấn", "Tiếng Anh thiết kế 2",
		"Thiết kế Logo", "Thiết kế nhận diện thương hiệu", "Thiết kế bao bì", "Thiết kế giao diện Figma", "Thiết kế trải nghiệm người dùng", "Đồ họa chuyển động", "Dự án thiết kế thực tế",
		"Thiết kế đồ họa 3D", "Dựng phim căn bản", "Thiết kế truyền thông số", "Xây dựng Portfolio", "Quản lý dự án thiết kế", "Thực tập doanh nghiệp thiết kế", "Đồ án tốt nghiệp thiết kế",
	},
}

func curriculumSemester(index int) int {
	if index < 8 {
		return 0
	}
	if index < 16 {
		return 1
	}
	if index < 23 {
		return 2
	}
	return 3
}

func (b *builder) schoolDataset(roles map[string]uint) {
	terms := []models.Semester{}
	for year := 2024; year <= 2027; year++ {
		start := time.Date(year, 9, 1, 0, 0, 0, 0, b.now.Location())
		ay := models.AcademicYear{Name: fmt.Sprintf("%d–%d", year, year+1), StartDate: start, EndDate: start.AddDate(1, 0, -1)}
		b.create(&ay)
		for part := 0; part < 2; part++ {
			from := start.AddDate(0, part*6, 0)
			until := from.AddDate(0, 6, -1)
			status := "active"
			if until.Before(b.now) {
				status = "closed"
			} else if from.After(b.now) {
				status = "upcoming"
			}
			term := models.Semester{Name: fmt.Sprintf("HK%d", len(terms)+1), AcademicYearID: ptr(ay.ID), StartDate: from, EndDate: until, Status: status}
			b.create(&term)
			terms = append(terms, term)
		}
	}
	rooms := []models.Room{}
	for i := 0; i < 10; i++ {
		r := models.Room{Name: fmt.Sprintf("%c10%d", 'A'+rune(i/2), i%2+1), Building: fmt.Sprintf("Tòa %c", 'A'+rune(i/2)), Capacity: 30, Description: "Phòng học và thực hành", IsActive: true}
		b.create(&r)
		rooms = append(rooms, r)
	}
	groups := []struct{ code, name string }{{"WEB", "Lập trình web"}, {"GAME", "Lập trình game"}, {"MKT", "Marketing"}, {"DESIGN", "Thiết kế đồ họa"}}
	for m, group := range groups {
		major := models.Major{Code: group.code, Name: group.name, Description: "Chương trình 30 môn trong 4 học kỳ – " + group.name, IsActive: true}
		b.create(&major)
		teachers := []models.Teacher{}
		for j := 0; j < 5; j++ {
			n := m*5 + j + 1
			u := b.user(fmt.Sprintf("teacher%02d", n), DemoTeacherNames[n-1], roles["teacher"], n == 4)
			t := models.Teacher{UserID: u.ID, TeacherCode: fmt.Sprintf("GV%03d", n), MajorID: major.ID, Phone: fmt.Sprintf("090100%04d", n), Address: "Thành phố Hồ Chí Minh", Qualification: "Thạc sĩ", HireDate: terms[0].StartDate.AddDate(-2, 0, 0)}
			b.create(&t)
			teachers = append(teachers, t)
		}
		courses := []models.Course{}
		for j, name := range curricula[group.code] {
			c := models.Course{Code: fmt.Sprintf("%s%03d", group.code, j+1), Name: name, MajorID: major.ID, SemesterID: terms[curriculumSemester(j)].ID, Credits: 3, IsActive: true}
			b.create(&c)
			courses = append(courses, c)
		}
		for cohort := 0; cohort < 3; cohort++ {
			for section := 0; section < 2; section++ {
				index := cohort*8 + m*2 + section
				class := models.Class{ClassCode: fmt.Sprintf("K%d_%s_%02d", 24+cohort, group.code, section+1), MajorID: major.ID, CohortYear: 2024 + cohort, MaxStudents: 20, Status: "open"}
				b.create(&class)
				students := []models.Student{}
				for j := 0; j < 20; j++ {
					n := index*20 + j + 1
					name := []string{"Nguyễn Minh", "Trần Ngọc", "Lê Hoàng", "Phạm Gia", "Võ Khánh"}[j%5] + " " + []string{"An", "Bình", "Chi", "Dũng", "Hà", "Huy", "Khang", "Linh", "Minh", "Ngọc", "Phúc", "Vy", "Anh", "Bảo", "Châu", "Đạt", "Giang", "Hân", "Nam", "Thảo"}[j]
					identity, err := utils.FormatStudentIdentity(name, 2024+cohort, "CQ", group.code, uint64(n))
					if err != nil {
						b.err = err
						return
					}
					u := b.user(identity.Username, name, roles["student"], n == 12, identity.Email)
					s := models.Student{UserID: u.ID, StudentCode: identity.StudentCode, TrainingSystem: "CQ", MajorID: major.ID, ClassID: class.ID, DateOfBirth: time.Date(2006+cohort, time.Month(j%12+1), j%27+1, 0, 0, 0, 0, b.now.Location()), Gender: []string{"Nam", "Nữ"}[j%2], Phone: fmt.Sprintf("091200%04d", n), Address: "Thành phố Hồ Chí Minh", EnrollmentDate: terms[cohort*2].StartDate, Status: "active"}
					b.create(&s)
					students = append(students, s)
				}
				b.start, b.end, b.session = []string{"07:30", "13:30", "18:00"}[index%3], []string{"10:30", "16:30", "21:00"}[index%3], []string{"Sáng", "Chiều", "Tối"}[index%3]
				weekday := time.Weekday((index/3 + 1) % 7)
				for j, course := range courses {
					part := curriculumSemester(j)
					offset := j - []int{0, 8, 16, 23}[part]
					term := terms[cohort*2+part]
					dates := weeklyDates(term.StartDate.AddDate(0, 0, offset*21), term.EndDate, weekday, 3)
					archived := dates[len(dates)-1].Before(b.now)
					if !dates[0].Before(b.now) {
						b.offer(class, course, term, teachers[(cohort*2+section)%5], rooms[index%8], dates, "pending", false, "")
						continue
					}
					b.offering(class, course, term, teachers[(cohort*2+section)%5], rooms[index%8], students, dates, archived, false)
				}
			}
		}
	}
}
