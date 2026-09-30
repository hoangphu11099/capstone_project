package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ListMyCourseRegistrations(c *gin.Context) {
	student, ok := getCurrentStudent(c)
	if !ok {
		return
	}

	var enrollments []models.Enrollment
	if err := config.DB.Preload("Course").Preload("Class").Preload("CourseOffering.Semester").Preload("Class.Semester").
		Where("student_id = ?", student.ID).
		Order("enroll_date desc").
		Find(&enrollments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy danh sách học phần", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách học phần đã đăng ký thành công", "data": enrollments})
}

func CancelCourseRegistration(c *gin.Context) {
	student, ok := getCurrentStudent(c)
	if !ok {
		return
	}

	enrollmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil || enrollmentID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID đăng ký học phần không hợp lệ"})
		return
	}

	var enrollment models.Enrollment
	if err := config.DB.Preload("CourseOffering.Semester").Preload("Class.Semester").Preload("Grade").
		Where("id = ? AND student_id = ?", enrollmentID, student.ID).
		First(&enrollment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy học phần đã đăng ký"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi lấy học phần", "error": err.Error()})
		return
	}

	semester := enrollment.Class.Semester
	if enrollment.CourseOffering != nil {
		semester = enrollment.CourseOffering.Semester
	}
	if semester.Status != "active" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Chỉ được hủy học phần trong học kỳ đang mở"})
		return
	}

	if enrollment.Grade != nil && enrollment.Grade.Status == "Approved" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Học phần đã có điểm được duyệt nên không thể hủy"})
		return
	}

	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.CourseRegistration{}).Where("enrollment_id = ?", enrollment.ID).Update("status", "cancelled").Error; err != nil {
			return err
		}
		return tx.Model(&enrollment).Update("status", "cancelled").Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Hủy học phần thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Hủy học phần thành công"})
}

type RegisterCourseRequest struct {
	CourseOfferingID uint   `json:"courseOfferingId"`
	SemesterID       uint   `json:"semesterId"`
	ClassID          uint   `json:"classId"`
	CourseID         uint   `json:"courseId"`
	Note             string `json:"note"`
}
type OpenCourseClassResponse struct {
	CourseOfferingID uint              `json:"courseOfferingId"`
	ClassID          uint              `json:"classId"`
	ClassCode        string            `json:"classCode"`
	CourseID         uint              `json:"courseId"`
	CourseCode       string            `json:"courseCode"`
	CourseName       string            `json:"courseName"`
	Status           string            `json:"status"`
	MajorID          uint              `json:"majorId"`
	TeacherID        uint              `json:"teacherId"`
	TeacherCode      string            `json:"teacherCode"`
	TeacherName      string            `json:"teacherName"`
	SemesterID       uint              `json:"semesterId"`
	SemesterName     string            `json:"semesterName"`
	RoomID           uint              `json:"roomId"`
	RoomName         string            `json:"roomName"`
	MaxStudents      int               `json:"maxStudents"`
	CurrentStudents  int64             `json:"currentStudents"`
	Schedules        []models.Schedule `json:"schedules"`
}

func ListOpenCourseClasses(c *gin.Context) {
	var offerings []models.CourseOffering

	query := config.DB.
		Preload("Class").
		Preload("Course").
		Preload("Teacher.User").
		Preload("Semester").
		Preload("Room").
		Preload("Schedules").
		Where("status = ?", "open")

	if semesterID := c.Query("semesterId"); semesterID != "" {
		query = query.Where("semester_id = ?", semesterID)
	}

	if err := query.Order("class_id, course_id").Find(&offerings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Lỗi lấy danh sách lớp học phần đang mở",
			"error":   err.Error(),
		})
		return
	}

	responses := make([]OpenCourseClassResponse, 0, len(offerings))

	for _, offering := range offerings {
		if offering.Class.Status != "open" || offering.Semester.Status != "active" {
			continue
		}
		var currentStudents int64

		if err := config.DB.Model(&models.Enrollment{}).
			Where("course_offering_id = ? AND status <> ?", offering.ID, "cancelled").
			Count(&currentStudents).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Lỗi kiểm tra sĩ số lớp học phần",
				"error":   err.Error(),
			})
			return
		}

		item := OpenCourseClassResponse{
			CourseOfferingID: offering.ID,
			ClassID:          offering.ClassID,
			ClassCode:        offering.Class.ClassCode,
			CourseID:         offering.CourseID,
			CourseCode:       offering.Course.Code,
			CourseName:       offering.Course.Name,
			Status:           offering.Status,
			MajorID:          offering.Class.MajorID,
			TeacherID:        offering.TeacherID,
			SemesterID:       offering.SemesterID,
			RoomID:           offering.RoomID,
			RoomName:         offering.Room.Name,
			MaxStudents:      offering.Class.MaxStudents,
			CurrentStudents:  currentStudents,
			Schedules:        offering.Schedules,
		}

		if offering.Teacher.ID != 0 {
			item.TeacherCode = offering.Teacher.TeacherCode
			item.TeacherName = offering.Teacher.User.FullName
		}

		if offering.Semester.ID != 0 {
			item.SemesterName = offering.Semester.Name
		}

		responses = append(responses, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Lấy danh sách lớp học phần đang mở thành công",
		"data":    responses,
	})
}
func RegisterCourse(c *gin.Context) {
	student, ok := getCurrentStudent(c)
	if !ok {
		return
	}

	var req RegisterCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu đăng ký học phần không hợp lệ", "error": err.Error()})
		return
	}

	var offerings []models.CourseOffering
	query := config.DB.Preload("Class").Preload("Course").Preload("Semester").Where("status = ?", "open")
	if req.CourseOfferingID != 0 {
		query = query.Where("id = ?", req.CourseOfferingID)
	} else {
		if req.ClassID == 0 || req.CourseID == 0 {
			c.JSON(400, gin.H{"message": "Vui lòng chọn học phần"})
			return
		}
		query = query.Where("class_id = ? AND course_id = ?", req.ClassID, req.CourseID)
		if req.SemesterID != 0 {
			query = query.Where("semester_id = ?", req.SemesterID)
		}
	}
	if err := query.Find(&offerings).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}
	if len(offerings) != 1 {
		c.JSON(400, gin.H{"message": "Học phần không tồn tại hoặc có nhiều học kỳ; hãy chọn courseOfferingId"})
		return
	}
	offering := offerings[0]
	class, course := offering.Class, offering.Course
	courseID := course.ID
	if class.Status != "open" || offering.Semester.Status != "active" {
		c.JSON(400, gin.H{"message": "Lớp hoặc học kỳ chưa mở đăng ký"})
		return
	}

	var scheduleCount int64
	if err := config.DB.Model(&models.Schedule{}).Where("course_offering_id = ?", offering.ID).Count(&scheduleCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi kiểm tra lịch học phần", "error": err.Error()})
		return
	}
	if scheduleCount == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Học phần chưa có lịch học"})
		return
	}

	var duplicated int64
	if err := config.DB.Model(&models.Enrollment{}).
		Joins("JOIN course_offerings co ON co.id = enrollments.course_offering_id").Where("enrollments.student_id = ? AND enrollments.course_id = ? AND co.semester_id = ? AND enrollments.status <> ?", student.ID, courseID, offering.SemesterID, "cancelled").
		Count(&duplicated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi kiểm tra đăng ký học phần", "error": err.Error()})
		return
	}
	if duplicated > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Sinh viên đã đăng ký môn học này"})
		return
	}

	var currentStudents int64
	if err := config.DB.Model(&models.Enrollment{}).
		Where("course_offering_id = ? AND status <> ?", offering.ID, "cancelled").
		Count(&currentStudents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Lỗi kiểm tra sĩ số", "error": err.Error()})
		return
	}
	if class.MaxStudents > 0 && int(currentStudents) >= class.MaxStudents {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Lớp học phần đã đủ số lượng sinh viên"})
		return
	}

	now := time.Now()
	offeringID := offering.ID
	enrollment := models.Enrollment{
		CourseOfferingID: &offeringID,
		StudentID:        student.ID,
		ClassID:          class.ID,
		CourseID:         course.ID,
		EnrollDate:       now,
		Status:           "enrolled",
	}

	var registration models.CourseRegistration
	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&enrollment).Error; err != nil {
			return err
		}

		registration = models.CourseRegistration{
			StudentID:    student.ID,
			CourseID:     course.ID,
			ClassID:      class.ID,
			EnrollmentID: &enrollment.ID,
			Status:       "registered",
			RegisteredAt: now,
			Note:         req.Note,
		}
		return tx.Create(&registration).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Đăng ký học phần thất bại", "error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Đăng ký học phần thành công", "data": gin.H{"enrollment": enrollment, "registration": registration}})
}
