package seed

import "testing"

func TestDemoTeachersHaveDistinctNames(t *testing.T) {
	if len(DemoTeacherNames) != 20 {
		t.Fatal("expected 20 demo teachers")
	}
	seen := map[string]bool{}
	for _, name := range DemoTeacherNames {
		if name == "" || seen[name] {
			t.Fatalf("empty or duplicate teacher name: %q", name)
		}
		seen[name] = true
	}
}
