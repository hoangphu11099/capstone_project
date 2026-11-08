package seed

import "testing"

func TestCurriculumRequirements(t *testing.T) {
	if len(curricula) != 4 {
		t.Fatal("curriculum must contain four majors")
	}
	for code, courses := range curricula {
		if len(courses) != 30 {
			t.Fatalf("%s must have 30 courses", code)
		}
		seen := map[string]bool{}
		counts := [4]int{}
		for index, name := range courses {
			if name == "" || seen[name] {
				t.Fatalf("invalid or repeated course in %s: %s", code, name)
			}
			seen[name] = true
			counts[curriculumSemester(index)]++
		}
		if counts != [4]int{8, 8, 7, 7} {
			t.Fatalf("invalid semester distribution for %s: %v", code, counts)
		}
	}
}
