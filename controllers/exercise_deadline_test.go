package controllers

import (
	"student-management/models"
	"testing"
	"time"
)

func TestExerciseDeadlineBoundary(t *testing.T) {
	due := time.Date(2026, 10, 7, 13, 30, 0, 0, attendanceLocation())
	exercise := models.Exercise{Status: "open", DueDate: due}
	if !exerciseAcceptsSubmission(exercise, due.Add(-time.Nanosecond)) {
		t.Fatal("before deadline should be open")
	}
	if exerciseAcceptsSubmission(exercise, due) || exerciseAcceptsSubmission(exercise, due.Add(time.Second)) {
		t.Fatal("deadline must lock submissions")
	}
	exercise.Status = "closed"
	if exerciseAcceptsSubmission(exercise, due.Add(-time.Hour)) {
		t.Fatal("manual lock must block before due date")
	}
	parsed, err := parseExerciseDeadline("2026-10-07 13:30")
	if err != nil || !parsed.Equal(due) {
		t.Fatal("school timezone changed deadline")
	}
	parsed, err = parseExerciseDeadline("2026-10-07T06:30:00Z")
	if err != nil || !parsed.Equal(due) {
		t.Fatal("RFC3339 instant changed")
	}
}
