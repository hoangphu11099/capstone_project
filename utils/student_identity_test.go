package utils

import "testing"

func TestStudentIdentityConvention(t *testing.T) {
	identity, err := FormatStudentIdentity(" Nguyễn   Minh  Anh ", 2026, "cq", "web", 481)
	if err != nil {
		t.Fatal(err)
	}
	if identity.StudentCode != "2026CQWEB00481" || identity.Username != "student481" || identity.Email != "nma.web00481@ppacademy.edu.vn" {
		t.Fatalf("wrong identity: %+v", identity)
	}
	identity, err = FormatStudentIdentity("Đặng Thị Ánh", 2027, "PLUS", "DESIGN", 482)
	if err != nil || identity.StudentCode != "2027PLUSDESIGN00482" || identity.Email != "dta.design00482@ppacademy.edu.vn" {
		t.Fatalf("Vietnamese initials/system wrong: %+v %v", identity, err)
	}
	for _, system := range []string{"CQ", "PLUS", "THS", "TS"} {
		if _, err := FormatStudentIdentity("Test Name", 2026, system, "WEB", 1000); err != nil {
			t.Fatal(err)
		}
	}
	for _, input := range []struct {
		year          int
		system, major string
		sequence      uint64
	}{{0, "CQ", "WEB", 1}, {2026, "invalid", "WEB", 1}, {2026, "CQ", "", 1}, {2026, "CQ", "WEB<script>", 1}, {2026, "CQ", "WEB", 0}} {
		if _, err := FormatStudentIdentity("Name", input.year, input.system, input.major, input.sequence); err == nil {
			t.Fatalf("invalid identity accepted: %+v", input)
		}
	}
}
