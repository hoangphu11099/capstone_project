package utils

import (
	"strings"
	"unicode"
)

// MajorCodeBase derives a short ASCII abbreviation from the school's major name.
func MajorCodeBase(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsMark(r) {
			return r
		}
		return ' '
	}, name)
	code := strings.ToUpper(studentInitials(name))
	if len(code) > 12 {
		code = code[:12]
	}
	if code == "" {
		code = "CN"
	}
	return code
}
