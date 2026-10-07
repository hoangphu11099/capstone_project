package controllers

import (
	"student-management/models"
	"testing"
	"time"
)

func TestTeachingDateCannotBePast(t *testing.T) {
	// UTC is still October 6, but the school is already on October 7.
	now := time.Date(2026, 10, 6, 18, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		date   string
		reject bool
	}{
		{"2026-10-06", true}, {"2026-10-07", false}, {"2026-10-08", false},
	} {
		t.Run(tc.date, func(t *testing.T) {
			day, _ := time.ParseInLocation("2006-01-02", tc.date, attendanceLocation())
			err := validateFutureTeachingDates([]models.Schedule{{FullDate: &day}}, now)
			if (err != nil) != tc.reject {
				t.Fatalf("date %s: unexpected error %v", tc.date, err)
			}
		})
	}
	yesterday := time.Date(2026, 10, 6, 0, 0, 0, 0, attendanceLocation())
	tomorrow := yesterday.AddDate(0, 0, 2)
	if validateFutureTeachingDates([]models.Schedule{{FullDate: &tomorrow}, {FullDate: &yesterday}}, now) == nil {
		t.Fatal("batch containing a past date must be rejected")
	}
}
