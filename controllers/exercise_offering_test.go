package controllers

import (
	"student-management/models"
	"testing"
	"time"
)

func TestExerciseOfferingUsesFinalLesson(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, attendanceLocation())
	future := now.AddDate(0, 1, 0)
	past := now.AddDate(0, -1, 0)
	offering := models.CourseOffering{Status: "open", Class: models.Class{Status: "open"}, Semester: models.Semester{Status: "closed", EndDate: past}, Schedules: []models.Schedule{{FullDate: &future, EndTime: "16:30"}}}
	if exerciseOfferingEnded(offering, now) {
		t.Fatal("future lesson must allow preparation even when semester metadata is stale")
	}
	offering.Schedules[0].FullDate = &past
	offering.Semester.Status = "active"
	offering.Semester.EndDate = future
	if !exerciseOfferingEnded(offering, now) {
		t.Fatal("final lesson already ended must block new exercises")
	}
	offering.Schedules[0].FullDate = &future
	offering.Class.Status = "closed"
	if !exerciseOfferingEnded(offering, now) {
		t.Fatal("explicitly closed class must remain blocked")
	}
}
