package controllers

import (
	"reflect"
	"testing"
	"time"

	"student-management/models"
)

func TestWarningAttendanceAndGradeBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 23, 15, 0, 0, 0, attendanceLocation())
	offering := &models.CourseOffering{}
	for i := 0; i < 8; i++ {
		date := now.AddDate(0, 0, i-4)
		offering.Schedules = append(offering.Schedules, models.Schedule{FullDate: &date, StartTime: "08:00", EndTime: "10:00"})
	}
	base := models.Enrollment{CourseOffering: offering, Grade: &models.Grade{Status: "Approved", TotalScore: 40}}
	base.Attendances = []models.Attendance{{ClassDate: now.AddDate(0, 0, -2), Status: "absent"}, {ClassDate: now.AddDate(0, 0, -1), Status: "excused"}}
	r := enrollmentWarning(base, now)
	if r.TotalLessons != 8 || r.Missed != 2 || r.Unexcused != 1 || !reflect.DeepEqual(r.Reasons, []string{"unexcused"}) {
		t.Fatalf("25%% and passing score should not trigger excess absence or failure: %+v", r)
	}
	base.Attendances = append(base.Attendances, models.Attendance{ClassDate: now.AddDate(0, 0, -3), Status: "excused"})
	base.Grade.TotalScore = 39.99
	r = enrollmentWarning(base, now)
	if !reflect.DeepEqual(r.Reasons, []string{"unexcused", "absence_limit", "failed_course"}) {
		t.Fatalf("expected three reasons: %+v", r)
	}
	base.Grade.Status = "Draft"
	if candidateHasReason(enrollmentWarning(base, now), "failed_course") {
		t.Fatal("draft score must not trigger failure")
	}
}

func TestWarningIgnoresUnfinishedAndCancelledAttendance(t *testing.T) {
	now := time.Date(2026, 10, 23, 9, 0, 0, 0, attendanceLocation())
	schedule := &models.Schedule{StartTime: "08:00", EndTime: "10:00"}
	e := models.Enrollment{Attendances: []models.Attendance{
		{ClassDate: now, Status: "absent", AttendanceSession: &models.AttendanceSession{Status: "pending", ClassDate: now}},
		{ClassDate: now, Status: "absent", AttendanceSession: &models.AttendanceSession{Status: "cancelled", ClassDate: now}},
		{ClassDate: now, Status: "absent", AttendanceSession: &models.AttendanceSession{Status: "confirmed", IsActive: true, ClassDate: now, Schedule: schedule}},
		{ClassDate: now.AddDate(0, 0, 1), Status: "absent"},
	}}
	if r := enrollmentWarning(e, now); len(r.Reasons) != 0 {
		t.Fatalf("unfinished attendance counted: %+v", r)
	}
	if r := enrollmentWarning(e, now.Add(2*time.Hour)); r.Unexcused != 1 {
		t.Fatalf("ended confirmed lesson should count: %+v", r)
	}
}

func TestWarningWeeklyPlanAndMissingSchedule(t *testing.T) {
	start := time.Date(2026, 10, 5, 0, 0, 0, 0, attendanceLocation())
	offering := &models.CourseOffering{Semester: models.Semester{StartDate: start, EndDate: start.AddDate(0, 0, 13)}, Schedules: []models.Schedule{{DayOfWeek: "Mon", StartTime: "08:00", EndTime: "10:00"}, {DayOfWeek: "Mon", StartTime: "08:00", EndTime: "10:00"}, {DayOfWeek: "Wed", StartTime: "08:00", EndTime: "10:00"}}}
	if count := warningLessonCount(offering); count != 4 {
		t.Fatalf("want 4 unique weekly lessons, got %d", count)
	}
	e := models.Enrollment{Attendances: []models.Attendance{{ClassDate: start, Status: "absent"}}}
	r := enrollmentWarning(e, start.AddDate(0, 0, 1))
	if r.TotalLessons != 0 || candidateHasReason(r, "absence_limit") || !candidateHasReason(r, "unexcused") {
		t.Fatalf("missing schedule must not invent an absence limit: %+v", r)
	}
}
