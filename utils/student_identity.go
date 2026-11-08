package utils

import (
	"fmt"
	"golang.org/x/text/unicode/norm"
	"strings"
	"unicode"
)

const StudentEmailDomain = "ppacademy.edu.vn"

type StudentIdentity struct {
	StudentCode    string `json:"studentCode"`
	Username       string `json:"username"`
	Email          string `json:"email"`
	Sequence       uint64 `json:"sequence"`
	AdmissionYear  int    `json:"admissionYear"`
	TrainingSystem string `json:"trainingSystem"`
}

func NormalizeTrainingSystem(value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		value = "CQ"
	}
	switch value {
	case "CQ", "PLUS", "THS", "TS":
		return value, nil
	}
	return "", fmt.Errorf("Hệ đào tạo không hợp lệ")
}

func studentInitials(name string) string {
	var result strings.Builder
	for _, part := range strings.Fields(strings.ToLower(name)) {
		for _, letter := range norm.NFD.String(part) {
			if unicode.Is(unicode.Mn, letter) {
				continue
			}
			if letter == 'đ' {
				letter = 'd'
			}
			if letter >= 'a' && letter <= 'z' {
				result.WriteRune(letter)
				break
			}
		}
	}
	return result.String()
}

func FormatStudentIdentity(name string, year int, system, major string, sequence uint64) (StudentIdentity, error) {
	identity := StudentIdentity{}
	system, err := NormalizeTrainingSystem(system)
	if err != nil {
		return identity, err
	}
	major = strings.ToUpper(strings.TrimSpace(major))
	if year < 1900 || year > 9999 || sequence == 0 || major == "" {
		return identity, fmt.Errorf("Thiếu năm nhập học, chuyên ngành hoặc số thứ tự")
	}
	for _, letter := range major {
		if !(letter >= 'A' && letter <= 'Z') && !(letter >= '0' && letter <= '9') {
			return identity, fmt.Errorf("Mã chuyên ngành chỉ được gồm chữ và số")
		}
	}
	identity = StudentIdentity{StudentCode: fmt.Sprintf("%04d%s%s%05d", year, system, major, sequence), Username: fmt.Sprintf("student%03d", sequence), Sequence: sequence, AdmissionYear: year, TrainingSystem: system}
	if initials := studentInitials(name); initials != "" {
		identity.Email = fmt.Sprintf("%s.%s%05d@%s", initials, strings.ToLower(major), sequence, StudentEmailDomain)
	}
	if len(identity.StudentCode) > 32 || len(identity.Username) > 100 {
		return StudentIdentity{}, fmt.Errorf("Mã sinh viên vượt quá độ dài cho phép")
	}
	return identity, nil
}
