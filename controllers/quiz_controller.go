package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"io"
	"net/http"
	"strconv"
	"strings"
	"student-management/config"
	"student-management/middleware"
	"student-management/models"
)

func quizError(c *gin.Context, err error) { c.JSON(400, gin.H{"message": err.Error()}) }
func quizManager(c *gin.Context) bool {
	role := c.GetString(middleware.ContextRoleKey)
	if role != "teacher" {
		c.AbortWithStatusJSON(403, gin.H{"message": "Chỉ giảng viên được quản lý đề trắc nghiệm"})
		return false
	}
	return true
}
func quizAccess(c *gin.Context, db *gorm.DB, quiz *models.Quiz) bool {
	if c.GetString(middleware.ContextRoleKey) == "admin" {
		return false
	}
	if c.GetString(middleware.ContextRoleKey) == "teacher" {
		t, ok := getCurrentTeacher(c)
		return ok && quiz.CourseOffering.TeacherID == t.ID
	}
	s, ok := getCurrentStudent(c)
	if !ok {
		return false
	}
	var count int64
	db.Model(&models.Enrollment{}).Where("student_id=? AND course_offering_id=? AND status<>?", s.ID, quiz.CourseOfferingID, "cancelled").Count(&count)
	return count > 0 && quiz.Status == "published"
}
func loadQuiz(c *gin.Context, db *gorm.DB, lock bool) (models.Quiz, bool) {
	var q models.Quiz
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"message": "ID bài không hợp lệ"})
		return q, false
	}
	query := db.Preload("CourseOffering.Course").Preload("CourseOffering.Class").Preload("CourseOffering.Teacher.User")
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&q, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "Không tìm thấy bài trắc nghiệm"})
		return q, false
	}
	if !quizAccess(c, db, &q) {
		if !c.IsAborted() {
			c.AbortWithStatusJSON(403, gin.H{"message": "Bạn không được truy cập bài này"})
		}
		return q, false
	}
	return q, true
}

func CreateQuiz(c *gin.Context) {
	if !quizManager(c) {
		return
	}
	var req struct {
		Title            string `json:"title"`
		Kind             string `json:"kind"`
		CourseOfferingID uint   `json:"courseOfferingId"`
		DueDate          string `json:"dueDate"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Title) == "" {
		quizError(c, errors.New("Nhập tên bài trắc nghiệm"))
		return
	}
	var o models.CourseOffering
	if config.DB.Preload("Class").Preload("Semester").Preload("Schedules").First(&o, req.CourseOfferingID).Error != nil {
		quizError(c, errors.New("Học phần không tồn tại"))
		return
	}
	if c.GetString(middleware.ContextRoleKey) == "teacher" {
		t, ok := getCurrentTeacher(c)
		if !ok {
			return
		}
		if t.ID != o.TeacherID {
			c.JSON(403, gin.H{"message": "Không được phân công học phần"})
			return
		}
	}
	due, err := parseExerciseDeadline(req.DueDate)
	if err != nil || !due.After(attendanceNow()) {
		quizError(c, errors.New("Chọn hạn nộp ở tương lai"))
		return
	}
	if exerciseOfferingEnded(o, attendanceNow()) {
		quizError(c, errors.New("Học phần đã kết thúc"))
		return
	}
	if req.Kind != "test" {
		req.Kind = "exercise"
	}
	q := models.Quiz{Title: strings.TrimSpace(req.Title), Kind: req.Kind, CourseOfferingID: o.ID, DueDate: due, Status: "draft"}
	if err = config.DB.Create(&q).Error; err != nil {
		quizError(c, err)
		return
	}
	c.JSON(201, gin.H{"data": q})
}
func ListQuizzes(c *gin.Context) {
	role := c.GetString(middleware.ContextRoleKey)
	query := config.DB.Model(&models.Quiz{})
	if role == "teacher" {
		t, ok := getCurrentTeacher(c)
		if !ok {
			return
		}
		query = query.Where("course_offering_id IN (SELECT id FROM course_offerings WHERE teacher_id=?)", t.ID)
	} else if role == "student" {
		s, ok := getCurrentStudent(c)
		if !ok {
			return
		}
		query = query.Where("status=? AND course_offering_id IN (SELECT course_offering_id FROM enrollments WHERE student_id=? AND status<>?)", "published", s.ID, "cancelled")
	} else {
		c.JSON(403, gin.H{"message": "Không có quyền"})
		return
	}
	var quizzes []models.Quiz
	if err := query.Preload("CourseOffering.Class").Preload("CourseOffering.Course").Preload("CourseOffering.Teacher.User").Order("created_at desc").Find(&quizzes).Error; err != nil {
		quizError(c, err)
		return
	}
	c.JSON(200, gin.H{"data": quizzes})
}
func QuizDetail(c *gin.Context) {
	q, ok := loadQuiz(c, config.DB, false)
	if !ok {
		return
	}
	var questions []models.QuizQuestion
	config.DB.Where("quiz_id=?", q.ID).Order("number").Find(&questions)
	result := gin.H{"data": q, "questions": questions}
	if c.GetString(middleware.ContextRoleKey) == "student" {
		s, ok := getCurrentStudent(c)
		if !ok {
			return
		}
		var attempt models.QuizAttempt
		if config.DB.Where("quiz_id=? AND student_id=?", q.ID, s.ID).First(&attempt).Error == nil {
			result["attempt"] = gin.H{"score": attempt.Score, "submittedAt": attempt.SubmittedAt}
		}
	} else {
		var keyed int64
		config.DB.Model(&models.QuizQuestion{}).Where("quiz_id=? AND answer<>?", q.ID, "").Count(&keyed)
		result["keyedCount"] = keyed
	}
	c.JSON(200, result)
}

// The student DTO and model JSON never include answer keys or per-question correctness.
func QuizAnswerKey(c *gin.Context) {
	if c.GetString(middleware.ContextRoleKey) != "teacher" {
		c.JSON(403, gin.H{"message": "Chỉ giảng viên phụ trách được xem đáp án"})
		return
	}
	q, ok := loadQuiz(c, config.DB, false)
	if !ok {
		return
	}
	var rows []models.QuizQuestion
	config.DB.Where("quiz_id=?", q.ID).Order("number").Find(&rows)
	keys := []gin.H{}
	for _, r := range rows {
		keys = append(keys, gin.H{"number": r.Number, "answer": r.Answer})
	}
	c.JSON(200, gin.H{"data": keys})
}

func readQuizExcel(reader io.Reader, answers bool) ([]models.QuizQuestion, error) {
	book, err := excelize.OpenReader(reader, excelize.Options{UnzipSizeLimit: 20 << 20, UnzipXMLSizeLimit: 10 << 20})
	if err != nil {
		return nil, errors.New("Không đọc được Excel .xlsx")
	}
	defer book.Close()
	sheets := book.GetSheetList()
	if len(sheets) != 1 {
		return nil, errors.New("File phải có đúng 1 sheet")
	}
	rows, err := book.GetRows(sheets[0])
	if err != nil || len(rows) < 2 {
		return nil, errors.New("File không có dữ liệu")
	}
	headers := []string{"STT", "CauHoi", "A", "B", "C", "D"}
	if answers {
		headers = []string{"STT", "DapAn"}
	}
	if len(rows[0]) != len(headers) {
		return nil, fmt.Errorf("Cột bắt buộc: %s", strings.Join(headers, ", "))
	}
	for i, h := range headers {
		if strings.TrimSpace(rows[0][i]) != h {
			return nil, fmt.Errorf("Cột %d phải là %s", i+1, h)
		}
	}
	parsed := []models.QuizQuestion{}
	seen := map[int]bool{}
	for index, row := range rows[1:] {
		if len(row) == 0 {
			continue
		}
		if len(row) > len(headers) {
			return nil, fmt.Errorf("Dòng %d có cột thừa", index+2)
		}
		for len(row) < len(headers) {
			row = append(row, "")
		}
		for i := range row {
			row[i] = strings.TrimSpace(row[i])
		}
		n, e := strconv.Atoi(row[0])
		if e != nil || n < 1 || seen[n] {
			return nil, fmt.Errorf("STT trùng hoặc không hợp lệ ở dòng %d", index+2)
		}
		seen[n] = true
		q := models.QuizQuestion{Number: n}
		if answers {
			q.Answer = strings.ToUpper(row[1])
			if !strings.Contains("ABCD", q.Answer) || len(q.Answer) != 1 {
				return nil, fmt.Errorf("Đáp án dòng %d phải là A, B, C hoặc D", index+2)
			}
		} else {
			for _, v := range row[1:] {
				if v == "" {
					return nil, fmt.Errorf("Dòng %d thiếu câu hỏi hoặc lựa chọn", index+2)
				}
			}
			q.Content = row[1]
			q.A = row[2]
			q.B = row[3]
			q.C = row[4]
			q.D = row[5]
		}
		parsed = append(parsed, q)
		if len(parsed) > 200 {
			return nil, errors.New("Tối đa 200 câu mỗi đề")
		}
	}
	if len(parsed) == 0 {
		return nil, errors.New("File không có câu hỏi")
	}
	return parsed, nil
}
func ImportQuizExcel(c *gin.Context) {
	if !quizManager(c) {
		return
	}
	isKey := c.Param("kind") == "answers"
	if c.Param("kind") != "questions" && !isKey {
		quizError(c, errors.New("Loại file không hợp lệ"))
		return
	}
	if isKey && c.GetString(middleware.ContextRoleKey) != "teacher" {
		c.JSON(403, gin.H{"message": "Giảng viên phụ trách nhập file đáp án"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 6<<20)
	file, err := c.FormFile("file")
	if err != nil || file.Size > 5<<20 {
		quizError(c, errors.New("Chọn file .xlsx tối đa 5MB"))
		return
	}
	stream, err := file.Open()
	if err != nil {
		quizError(c, err)
		return
	}
	defer stream.Close()
	parsed, err := readQuizExcel(stream, isKey)
	if err != nil {
		quizError(c, err)
		return
	}
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		q, ok := loadQuiz(c, tx, true)
		if !ok {
			return errors.New("Không thể sửa đề")
		}
		if q.Status != "draft" {
			return errors.New("Đề đã giao, không thể thay câu hỏi hoặc đáp án")
		}
		if isKey {
			var questions []models.QuizQuestion
			tx.Where("quiz_id=?", q.ID).Find(&questions)
			if len(questions) != len(parsed) {
				return errors.New("Số đáp án phải bằng số câu hỏi")
			}
			numbers := map[int]bool{}
			for _, r := range questions {
				numbers[r.Number] = true
			}
			for _, r := range parsed {
				if !numbers[r.Number] {
					return fmt.Errorf("Không có câu số %d trong đề", r.Number)
				}
				if err := tx.Model(&models.QuizQuestion{}).Where("quiz_id=? AND number=?", q.ID, r.Number).Update("answer", r.Answer).Error; err != nil {
					return err
				}
			}
			return nil
		}
		if err := tx.Where("quiz_id=?", q.ID).Delete(&models.QuizQuestion{}).Error; err != nil {
			return err
		}
		for i := range parsed {
			parsed[i].QuizID = q.ID
		}
		return tx.Create(&parsed).Error
	})
	if c.Writer.Written() {
		return
	}
	if err != nil {
		quizError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Đã nhập Excel. Kiểm tra đề trước khi giao.", "count": len(parsed)})
}
func PublishQuiz(c *gin.Context) {
	if !quizManager(c) {
		return
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		q, ok := loadQuiz(c, tx, true)
		if !ok {
			return errors.New("Không có quyền")
		}
		if !q.DueDate.After(attendanceNow()) {
			return errors.New("Đã quá hạn nộp")
		}
		var count, missing int64
		tx.Model(&models.QuizQuestion{}).Where("quiz_id=?", q.ID).Count(&count)
		tx.Model(&models.QuizQuestion{}).Where("quiz_id=? AND answer=?", q.ID, "").Count(&missing)
		if count == 0 || missing > 0 {
			return errors.New("Cần nhập đủ đề và đáp án trước khi giao bài")
		}
		return tx.Model(&q).Update("status", "published").Error
	})
	if c.Writer.Written() {
		return
	}
	if err != nil {
		quizError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Đã giao bài cho sinh viên đăng ký học phần"})
}
func SubmitQuiz(c *gin.Context) {
	s, ok := getCurrentStudent(c)
	if !ok {
		return
	}
	var req struct {
		Answers map[int]string `json:"answers"`
	}
	if c.ShouldBindJSON(&req) != nil {
		quizError(c, errors.New("Bài làm không hợp lệ"))
		return
	}
	var result models.QuizAttempt
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		q, ok := loadQuiz(c, tx, true)
		if !ok {
			return errors.New("Không có quyền")
		}
		if q.Status != "published" || !q.DueDate.After(attendanceNow()) {
			return errors.New("Bài đã đóng hoặc hết hạn")
		}
		var count int64
		tx.Model(&models.QuizAttempt{}).Where("quiz_id=? AND student_id=?", q.ID, s.ID).Count(&count)
		if count > 0 {
			return errors.New("Bạn đã nộp bài này; mỗi bài được nộp một lần")
		}
		var questions []models.QuizQuestion
		if err := tx.Where("quiz_id=?", q.ID).Find(&questions).Error; err != nil {
			return err
		}
		score, err := scoreQuiz(questions, req.Answers)
		if err != nil {
			return err
		}
		encoded, _ := json.Marshal(req.Answers)
		result = models.QuizAttempt{QuizID: q.ID, StudentID: s.ID, Answers: string(encoded), Score: score, SubmittedAt: attendanceNow()}
		return tx.Create(&result).Error
	})
	if c.Writer.Written() {
		return
	}
	if err != nil {
		quizError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Đã nộp và chấm điểm tự động", "score": result.Score})
}
func scoreQuiz(questions []models.QuizQuestion, answers map[int]string) (float64, error) {
	if len(questions) == 0 {
		return 0, errors.New("Đề chưa có câu hỏi")
	}
	valid := map[int]bool{}
	correct := 0
	for _, q := range questions {
		valid[q.Number] = true
		if q.Answer == "" {
			return 0, errors.New("Đề chưa có đủ đáp án")
		}
		if answers[q.Number] == q.Answer {
			correct++
		}
	}
	for n, a := range answers {
		if !valid[n] || len(a) != 1 || !strings.Contains("ABCD", a) {
			return 0, errors.New("Lựa chọn không hợp lệ")
		}
	}
	return float64(correct) * 100 / float64(len(questions)), nil
}
func QuizResults(c *gin.Context) {
	if !quizManager(c) {
		return
	}
	q, ok := loadQuiz(c, config.DB, false)
	if !ok {
		return
	}
	var rows []models.QuizAttempt
	config.DB.Preload("Student.User").Where("quiz_id=?", q.ID).Order("submitted_at desc").Find(&rows)
	c.JSON(200, gin.H{"data": rows})
}
func QuizTemplate(c *gin.Context) {
	if !quizManager(c) {
		return
	}
	key := c.Query("kind") == "answers"
	if key && c.GetString(middleware.ContextRoleKey) != "teacher" {
		c.JSON(403, gin.H{"message": "Chỉ giảng viên tải mẫu đáp án"})
		return
	}
	f := excelize.NewFile()
	defer f.Close()
	rows := [][]interface{}{{"STT", "CauHoi", "A", "B", "C", "D"}, {1, "2 + 2 bằng bao nhiêu?", "2", "3", "4", "5"}, {2, "Thủ đô Việt Nam là gì?", "Hà Nội", "Huế", "Đà Nẵng", "TP.HCM"}}
	if key {
		rows = [][]interface{}{{"STT", "DapAn"}, {1, "C"}, {2, "A"}}
	}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		f.SetSheetRow("Sheet1", cell, &row)
	}
	f.SetColWidth("Sheet1", "A", "F", 25)
	buf, err := f.WriteToBuffer()
	if err != nil {
		quizError(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=quiz-template.xlsx")
	c.Data(200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}
