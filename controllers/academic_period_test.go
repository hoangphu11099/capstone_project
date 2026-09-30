package controllers

import "testing"

func TestPeriodDateValidation(t *testing.T) {
	for _, tc := range []struct {
		name, start, end string
		ok               bool
	}{
		{"Term", "2026-09-01", "2027-01-31", true},
		{"One day", "2026-09-30", "2026-09-30", true},
		{"Bad", "2026-10-01", "2026-09-30", false},
		{"Bad", "2026-02-30", "2026-03-01", false},
		{" ", "2026-09-01", "2027-01-31", false},
	} {
		_, _, err := periodDates(PeriodRequest{Name: tc.name, StartDate: tc.start, EndDate: tc.end})
		if (err == nil) != tc.ok {
			t.Errorf("%+v: %v", tc, err)
		}
	}
}
