package controllers

import (
	"strings"
	"testing"
)

func TestImportTemplateParsing(t *testing.T) {
	header := "username,password,email,fullName,studentCode,classId,classCode,gender,phone,address,dateOfBirth"
	for _, separator := range []string{",", ";", "\t"} {
		data := "\ufeff" + strings.ReplaceAll(header, ",", separator) + "\n" + strings.Join([]string{"sv01", "", "a@example.com", "Student", "SV001", "", "CLASS01", "Nam", "0901234567", "", "2005-08-18"}, separator)
		items, err := parseStudentImportFile(strings.NewReader(data), "students.csv")
		if err != nil || len(items) != 1 {
			t.Fatalf("separator %q: items=%v err=%v", separator, items, err)
		}
		if items[0].StudentCode != "SV001" || items[0].ClassCode != "CLASS01" || items[0].Phone != "0901234567" {
			t.Fatalf("incorrect field mapping: %+v", items[0])
		}
	}
	if _, err := parseStudentImportFile(strings.NewReader("name,class\na,b"), "bad.csv"); err == nil {
		t.Fatal("expected invalid header error")
	}
	if _, err := parseStudentImportFile(strings.NewReader(header+"\nsv01,,,,,-1,CLASS01,,,,"), "bad.csv"); err == nil {
		t.Fatal("expected invalid class ID error")
	}
}

func TestTeacherImportTemplateParsing(t *testing.T) {
	data := "username,password,email,fullName,teacherCode,phone,address,qualification,majorCode\ngv01,,a@example.com,Teacher,GV001,0901234567,,Master,IT"
	items, err := parseTeacherImportFile(strings.NewReader(data), "teachers.csv")
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	if items[0].TeacherCode != "GV001" || items[0].MajorCode != "IT" {
		t.Fatalf("incorrect field mapping: %+v", items[0])
	}
}
