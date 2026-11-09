package controllers

import (
	"bytes"
	"encoding/json"
	"github.com/xuri/excelize/v2"
	"student-management/models"
	"testing"
)

func quizWorkbook(t *testing.T, rows [][]interface{}) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow("Sheet1", cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	buffer, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
func TestQuizExcelAndServerScoring(t *testing.T) {
	rows := [][]interface{}{{"STT", "CauHoi", "A", "B", "C", "D"}, {1, "Question 1", "a", "b", "c", "d"}, {2, "Question 2", "a", "b", "c", "d"}}
	questions, err := readQuizExcel(bytes.NewReader(quizWorkbook(t, rows)), false)
	if err != nil || len(questions) != 2 {
		t.Fatalf("parse: %v", err)
	}
	questions[0].Answer = "C"
	questions[1].Answer = "A"
	for _, tc := range []struct {
		answers map[int]string
		score   float64
	}{{map[int]string{1: "C", 2: "A"}, 100}, {map[int]string{1: "C"}, 50}, {map[int]string{}, 0}} {
		score, err := scoreQuiz(questions, tc.answers)
		if err != nil || score != tc.score {
			t.Fatalf("grade=%v err=%v", score, err)
		}
	}
	if _, err := scoreQuiz(questions, map[int]string{999: "A"}); err == nil {
		t.Fatal("unknown question accepted")
	}
	encoded, _ := json.Marshal(models.QuizQuestion{Number: 1, Answer: "C"})
	var publicQuestion map[string]interface{}
	json.Unmarshal(encoded, &publicQuestion)
	if _, leaked := publicQuestion["Answer"]; leaked {
		t.Fatal("answer leaked")
	}
	rows[2][0] = 1
	if _, err := readQuizExcel(bytes.NewReader(quizWorkbook(t, rows)), false); err == nil {
		t.Fatal("duplicate question accepted")
	}
	if _, err := readQuizExcel(bytes.NewReader(quizWorkbook(t, [][]interface{}{{"STT", "DapAn"}, {1, "E"}})), true); err == nil {
		t.Fatal("invalid answer accepted")
	}
	if _, err := readQuizExcel(bytes.NewReader(quizWorkbook(t, [][]interface{}{{"STT", "CauHoi", "A", "B", "C", "D", "DapAn"}, {1, "Question", "a", "b", "c", "d", "A"}})), false); err == nil {
		t.Fatal("answer column in student question file accepted")
	}
}
