package controllers

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type TeacherImportItem struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	Email         string `json:"email"`
	FullName      string `json:"fullName"`
	TeacherCode   string `json:"teacherCode"`
	Phone         string `json:"phone"`
	Address       string `json:"address"`
	Qualification string `json:"qualification"`
}

type StudentImportItem struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Email       string `json:"email"`
	FullName    string `json:"fullName"`
	StudentCode string `json:"studentCode"`
	ClassID     uint   `json:"classId"`
	ClassCode   string `json:"classCode"`
	Gender      string `json:"gender"`
	Phone       string `json:"phone"`
	Address     string `json:"address"`
	DateOfBirth string `json:"dateOfBirth"`
}

type teacherImportBody struct {
	Teachers []TeacherImportItem `json:"teachers"`
}

type studentImportBody struct {
	Students []StudentImportItem `json:"students"`
}

func ImportListTeacher(c *gin.Context) {
	items, err := readTeacherImportItems(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu import giảng viên không hợp lệ", "error": err.Error()})
		return
	}

	roleID, err := getRoleIDByName("teacher")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tìm thấy role teacher", "error": err.Error()})
		return
	}

	result := importSummary{Total: len(items)}
	for index, item := range items {
		if err := importOneTeacher(item, roleID); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Dòng %d: %s", index+1, err.Error()))
			continue
		}
		result.Success++
	}

	c.JSON(http.StatusOK, gin.H{"message": "Import danh sách giảng viên hoàn tất", "result": result})
}

func ImportListStudent(c *gin.Context) {
	items, err := readStudentImportItems(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu import sinh viên không hợp lệ", "error": err.Error()})
		return
	}

	roleID, err := getRoleIDByName("student")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tìm thấy role student", "error": err.Error()})
		return
	}

	result := importSummary{Total: len(items)}
	for index, item := range items {
		if err := importOneStudent(item, roleID); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Dòng %d: %s", index+1, err.Error()))
			continue
		}
		result.Success++
	}

	c.JSON(http.StatusOK, gin.H{"message": "Import danh sách sinh viên hoàn tất", "result": result})
}

type importSummary struct {
	Total   int      `json:"total"`
	Success int      `json:"success"`
	Errors  []string `json:"errors"`
}

func readTeacherImportItems(c *gin.Context) ([]TeacherImportItem, error) {
	fileHeader, err := c.FormFile("file")
	if err == nil {
		file, err := fileHeader.Open()
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return parseTeacherImportFile(file, fileHeader.Filename)
	}

	var body teacherImportBody
	if err := c.ShouldBindJSON(&body); err != nil {
		return nil, err
	}
	return body.Teachers, nil
}

func readStudentImportItems(c *gin.Context) ([]StudentImportItem, error) {
	fileHeader, err := c.FormFile("file")
	if err == nil {
		file, err := fileHeader.Open()
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return parseStudentImportFile(file, fileHeader.Filename)
	}

	var body studentImportBody
	if err := c.ShouldBindJSON(&body); err != nil {
		return nil, err
	}
	return body.Students, nil
}

func parseTeacherImportFile(r io.Reader, filename string) ([]TeacherImportItem, error) {
	rows, err := readImportRows(r, filename)
	if err != nil {
		return nil, err
	}
	if len(rows) <= 1 {
		return nil, errors.New("file import không có dữ liệu")
	}

	var items []TeacherImportItem
	for _, row := range rows[1:] {
		row = normalizeCSVRow(row, 8)
		if isEmptyRow(row) {
			continue
		}
		items = append(items, TeacherImportItem{
			Username:      row[0],
			Password:      row[1],
			Email:         row[2],
			FullName:      row[3],
			TeacherCode:   row[4],
			Phone:         row[5],
			Address:       row[6],
			Qualification: row[7],
		})
	}
	return items, nil
}

func parseStudentImportFile(r io.Reader, filename string) ([]StudentImportItem, error) {
	rows, err := readImportRows(r, filename)
	if err != nil {
		return nil, err
	}
	if len(rows) <= 1 {
		return nil, errors.New("file import không có dữ liệu")
	}

	var items []StudentImportItem
	for _, row := range rows[1:] {
		row = normalizeCSVRow(row, 11)
		if isEmptyRow(row) {
			continue
		}
		classID, _ := strconv.Atoi(row[5])
		items = append(items, StudentImportItem{
			Username:    row[0],
			Password:    row[1],
			Email:       row[2],
			FullName:    row[3],
			StudentCode: row[4],
			ClassID:     uint(classID),
			ClassCode:   row[6],
			Gender:      row[7],
			Phone:       row[8],
			Address:     row[9],
			DateOfBirth: row[10],
		})
	}
	return items, nil
}

func readImportRows(r io.Reader, filename string) ([][]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".xlsx", ".xlsm", ".xltx", ".xltm":
		return readExcelRows(data)
	case ".xls":
		return nil, errors.New("file .xls chưa được hỗ trợ, vui lòng lưu lại thành .xlsx hoặc .csv")
	default:
		return readDelimitedRows(data)
	}
}

func readExcelRows(data []byte) ([][]string, error) {
	workbook, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer workbook.Close()

	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("file Excel không có sheet")
	}
	return workbook.GetRows(sheets[0])
}

func readDelimitedRows(data []byte) ([][]string, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	if bytes.Contains(data, []byte("\t")) && !bytes.Contains(data, []byte(",")) {
		reader.Comma = '\t'
	}
	return reader.ReadAll()
}

func isEmptyRow(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func importOneTeacher(item TeacherImportItem, roleID uint) error {
	item.Username = strings.TrimSpace(item.Username)
	item.TeacherCode = strings.TrimSpace(item.TeacherCode)
	if item.Username == "" || item.TeacherCode == "" {
		return errors.New("username và teacherCode không được để trống")
	}
	if item.Password == "" {
		item.Password = "123456"
	}

	return config.DB.Transaction(func(tx *gorm.DB) error {
		user := models.User{}
		err := tx.Where("username = ?", item.Username).First(&user).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(item.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		user.Username = item.Username
		user.Password = string(passwordHash)
		user.Email = strings.TrimSpace(item.Email)
		user.FullName = strings.TrimSpace(item.FullName)
		user.RoleID = roleID
		user.IsActive = true
		user.FirstLogin = true

		if user.ID == 0 {
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		} else if err := tx.Save(&user).Error; err != nil {
			return err
		}

		teacher := models.Teacher{}
		err = tx.Where("teacher_code = ?", item.TeacherCode).First(&teacher).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		teacher.TeacherCode = item.TeacherCode
		teacher.UserID = user.ID
		teacher.Phone = strings.TrimSpace(item.Phone)
		teacher.Address = strings.TrimSpace(item.Address)
		teacher.Qualification = strings.TrimSpace(item.Qualification)
		if teacher.HireDate.IsZero() {
			teacher.HireDate = time.Now()
		}

		if teacher.ID == 0 {
			return tx.Create(&teacher).Error
		}
		return tx.Save(&teacher).Error
	})
}

func importOneStudent(item StudentImportItem, roleID uint) error {
	item.Username = strings.TrimSpace(item.Username)
	item.StudentCode = strings.TrimSpace(item.StudentCode)
	if item.Username == "" || item.StudentCode == "" {
		return errors.New("username và studentCode không được để trống")
	}
	if item.Password == "" {
		item.Password = "123456"
	}

	classID := item.ClassID
	if classID == 0 && strings.TrimSpace(item.ClassCode) != "" {
		var class models.Class
		if err := config.DB.Where("class_code = ?", strings.TrimSpace(item.ClassCode)).First(&class).Error; err != nil {
			return errors.New("không tìm thấy classCode " + item.ClassCode)
		}
		classID = class.ID
	}
	if classID == 0 {
		return errors.New("classId hoặc classCode không được để trống")
	}

	dob := time.Time{}
	if strings.TrimSpace(item.DateOfBirth) != "" {
		parsed, err := parseDateTime(item.DateOfBirth)
		if err != nil {
			return errors.New("dateOfBirth không hợp lệ")
		}
		dob = parsed
	}

	return config.DB.Transaction(func(tx *gorm.DB) error {
		user := models.User{}
		err := tx.Where("username = ?", item.Username).First(&user).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(item.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		user.Username = item.Username
		user.Password = string(passwordHash)
		user.Email = strings.TrimSpace(item.Email)
		user.FullName = strings.TrimSpace(item.FullName)
		user.RoleID = roleID
		user.IsActive = true
		user.FirstLogin = true

		if user.ID == 0 {
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		} else if err := tx.Save(&user).Error; err != nil {
			return err
		}

		student := models.Student{}
		err = tx.Where("student_code = ?", item.StudentCode).First(&student).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		student.StudentCode = item.StudentCode
		student.UserID = user.ID
		student.ClassID = classID
		student.DateOfBirth = dob
		student.Gender = strings.TrimSpace(item.Gender)
		student.Phone = strings.TrimSpace(item.Phone)
		student.Address = strings.TrimSpace(item.Address)
		student.Status = "active"
		if student.EnrollmentDate.IsZero() {
			student.EnrollmentDate = time.Now()
		}

		if student.ID == 0 {
			return tx.Create(&student).Error
		}
		return tx.Save(&student).Error
	})
}

func getRoleIDByName(name string) (uint, error) {
	var role models.Role
	if err := config.DB.Where("name = ?", name).First(&role).Error; err != nil {
		return 0, err
	}
	return role.ID, nil
}

func normalizeCSVRow(row []string, size int) []string {
	out := make([]string, size)
	for i := 0; i < size && i < len(row); i++ {
		out[i] = strings.TrimSpace(row[i])
	}
	return out
}
