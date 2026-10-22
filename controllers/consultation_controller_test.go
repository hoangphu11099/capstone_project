package controllers

import "testing"

func TestValidateConsultation(t *testing.T) {
	valid := consultationRequest{FullName: "Nguyễn An", Phone: "090 123 4567", Email: "an@example.com", Program: "Thiết kế đồ họa", Location: "Online", PreferredSession: "Tối", Consent: true}
	normalized := valid
	if err := validateConsultation(&normalized); err != nil || normalized.Phone != "0901234567" {
		t.Fatalf("valid request: %v, phone=%s", err, normalized.Phone)
	}
	for _, tc := range []struct {
		name   string
		change func(*consultationRequest)
	}{
		{"missing consent", func(r *consultationRequest) { r.Consent = false }},
		{"invalid phone", func(r *consultationRequest) { r.Phone = "abc" }},
		{"invalid email", func(r *consultationRequest) { r.Email = "not-an-email" }},
		{"missing name", func(r *consultationRequest) { r.FullName = " " }},
		{"invalid session", func(r *consultationRequest) { r.PreferredSession = "invalid" }},
		{"missing program", func(r *consultationRequest) { r.Program = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := valid
			tc.change(&req)
			if validateConsultation(&req) == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
}
