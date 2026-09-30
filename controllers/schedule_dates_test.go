package controllers

import (
	"student-management/models"
	"testing"
	"time"
)

func TestAssignmentCalendarDates(t *testing.T) {
	today := attendanceNow().Format("2006-01-02")
	req := AssignCourseOfferingRequest{TeachingDates: []string{today, attendanceNow().AddDate(0, 0, 7).Format("2006-01-02"), today}, StartTime: "08:00", EndTime: "09:00"}
	rows, err := assignmentSchedules(req)
	if err != nil || len(rows) != 2 {
		t.Fatalf("dates: %v %v", rows, err)
	}
	if rows[0].FullDate.Format("2006-01-02") != today {
		t.Fatal("today was lost")
	}
	req.TeachingDates = []string{"2026-02-30"}
	if _, err := assignmentSchedules(req); err == nil {
		t.Fatal("accepted invalid date")
	}
	req.TeachingDates = nil
	req.DayOfWeek = "Mon"
	rows, err = assignmentSchedules(req)
	if err != nil || rows[0].FullDate != nil {
		t.Fatal("legacy weekday failed")
	}
	req.EndTime = "07:00"
	if _, err := assignmentSchedules(req); err == nil {
		t.Fatal("accepted reversed times")
	}
}
func TestCalendarConflictAndAttendance(t *testing.T) {
	date, _ := time.Parse("2006-01-02", "2026-09-30")
	next := date.AddDate(0, 0, 7)
	a := models.Schedule{FullDate: &date, DayOfWeek: "Wed", StartTime: "08:00", EndTime: "09:00"}
	b := a
	b.FullDate = &next
	if schedulesOverlap(a, b) {
		t.Fatal("different dates conflict")
	}
	if scheduleMatchesDate(a, next) {
		t.Fatal("dated lesson repeats next week")
	}
	if !scheduleMatchesDate(a, date) {
		t.Fatal("lesson missing on exact date")
	}
	b.FullDate = nil
	if !schedulesOverlap(a, b) || !scheduleMatchesDate(b, next) {
		t.Fatal("legacy recurrence lost")
	}
	b.StartTime = "09:00"
	b.EndTime = "10:00"
	if schedulesOverlap(a, b) {
		t.Fatal("adjacent lessons conflict")
	}
}
