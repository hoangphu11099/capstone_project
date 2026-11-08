package controllers

import (
	"student-management/models"
	"testing"
	"time"
)

func TestOfferPreparationPeriod(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, attendanceLocation())
	for _, tc := range []struct {
		name, date, clock, reason string
		urgent, wantError         bool
	}{
		{"seven days", "2026-11-09", "07:30", "", false, false},
		{"six days", "2026-11-08", "13:30", "", false, true},
		{"urgent with reason", "2026-11-03", "07:30", "Thay giảng viên đột xuất", true, false},
		{"urgent without reason", "2026-11-03", "07:30", "  ", true, true},
		{"reason alone cannot bypass", "2026-11-03", "07:30", "Khẩn cấp", false, true},
		{"past date with exception", "2026-11-01", "13:30", "Khẩn cấp", true, true},
		{"past time today", "2026-11-02", "09:59", "Khẩn cấp", true, true},
		{"already starting", "2026-11-02", "10:00", "Khẩn cấp", true, true},
		{"future today with exception", "2026-11-02", "10:01", "Khẩn cấp", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			date, err := time.ParseInLocation("2006-01-02", tc.date, attendanceLocation())
			if err != nil {
				t.Fatal(err)
			}
			err = validateOfferPreparation([]models.Schedule{{FullDate: &date, StartTime: tc.clock}}, now, tc.urgent, tc.reason)
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected validation result: %v", err)
			}
		})
	}
	// A later lesson does not hide a first lesson with insufficient notice.
	first := now.AddDate(0, 0, 6)
	last := now.AddDate(0, 0, 13)
	if validateOfferPreparation([]models.Schedule{{FullDate: &last, StartTime: "13:30"}, {FullDate: &first, StartTime: "13:30"}}, now, false, "") == nil {
		t.Fatal("accepted a plan with an early lesson")
	}
	// Use the school calendar even when the caller's clock is UTC.
	date := time.Date(2026, 11, 9, 0, 0, 0, 0, attendanceLocation())
	utcNow := time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)
	if err := validateOfferPreparation([]models.Schedule{{FullDate: &date, StartTime: "07:30"}}, utcNow, false, ""); err != nil {
		t.Fatal(err)
	}
	date = date.AddDate(0, 0, -1)
	if validateOfferPreparation([]models.Schedule{{FullDate: &date, StartTime: "07:30"}}, utcNow, false, "") == nil {
		t.Fatal("used UTC date instead of school date")
	}
}
