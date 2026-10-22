package controllers

import "testing"

func TestNormalizeExamSession(t *testing.T) {
	for _, tc := range []struct{ session, normalized, start, end string }{
		{"AM", "Sáng", "07:30", "11:30"}, {"Sáng", "Sáng", "07:30", "11:30"},
		{"Trưa", "Chiều", "13:30", "16:30"}, {"Chiều", "Chiều", "13:30", "16:30"},
		{"Tối", "Tối", "18:00", "21:00"},
	} {
		req := ExamScheduleRequest{Session: tc.session, StartTime: "00:00", EndTime: "00:01", ExamType: "Online"}
		normalizeExamSession(&req)
		if req.Session != tc.normalized || req.StartTime != tc.start || req.EndTime != tc.end || req.ExamType != "Online" {
			t.Fatalf("unexpected fixed exam session: %+v", req)
		}
	}
	req := ExamScheduleRequest{Session: "Sáng"}
	normalizeExamSession(&req)
	if req.ExamType != "Trực tiếp" {
		t.Fatal("missing default exam format")
	}
}
