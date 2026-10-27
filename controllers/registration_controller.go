package controllers

import (
	"errors"
	"net/http"
	"strconv"

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

	var requests []models.CourseRegistration
	if err := config.DB.Preload("Enrollment.CourseOffering.Semester").Preload("Enrollment.CourseOffering.Teacher.User").Preload("Enrollment.CourseOffering.Room").Preload("Enrollment.CourseOffering.Schedules").Preload("Course").Preload("Class").Preload("CourseOffering.Semester").Preload("CourseOffering.Teacher.User").Preload("CourseOffering.Room").Preload("CourseOffering.Schedules").Where("student_id = ?", student.ID).Order("registered_at desc").Find(&requests).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}
	for i := range requests {
		if requests[i].CourseOffering == nil && requests[i].Enrollment != nil {
			requests[i].CourseOffering = requests[i].Enrollment.CourseOffering
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách học phần đã đăng ký thành công", "data": enrollments, "requests": requests})
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

	var paid int64
	if err := config.DB.Model(&models.CourseRegistration{}).Where("enrollment_id = ? AND payment_confirmed_at IS NOT NULL", enrollment.ID).Count(&paid).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}
	if paid > 0 {
		c.JSON(400, gin.H{"message": "Vui lòng liên hệ học vụ để hủy học phần đã xác nhận học phí"})
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

func registrationMajor(db *gorm.DB, student models.Student) (models.Major, error) {
	var class models.Class
	if student.ClassID == 0 {
		return models.Major{}, errors.New("Sinh viên chưa được xếp lớp. Vui lòng liên hệ học vụ cập nhật chuyên ngành")
	}
	if err := db.Preload("Major").First(&class, student.ClassID).Error; err != nil {
		return models.Major{}, err
	}
	if class.MajorID == 0 || class.Major.ID == 0 {
		return models.Major{}, errors.New("Lớp sinh viên chưa có chuyên ngành. Vui lòng liên hệ học vụ cập nhật")
	}
	return class.Major, nil
}

func ListOpenCourseClasses(c *gin.Context) {
	student, ok := getCurrentStudent(c)
	if !ok {
		return
	}
	major, err := registrationMajor(config.DB, student)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	var offerings []models.CourseOffering

	query := config.DB.
		Preload("Class").
		Preload("Course").
		Preload("Teacher.User").
		Preload("Semester").
		Preload("Room").
		Preload("Schedules").
		Where("status = ?", "open").
		Where("class_id IN (?)", config.DB.Model(&models.Class{}).Select("id").Where("major_id = ?", major.ID)).
		Where("course_id IN (?)", config.DB.Model(&models.Course{}).Select("id").Where("major_id = ?", major.ID))

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

		var pending int64
		if err := config.DB.Model(&models.CourseRegistration{}).Where("course_offering_id = ? AND status IN ?", offering.ID, []string{"pending_payment", "paid"}).Count(&pending).Error; err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		currentStudents += pending
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
		"message":   "Lấy danh sách lớp học phần đang mở thành công",
		"data":      responses,
		"majorId":   major.ID,
		"majorName": major.Name,
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
	var registration models.CourseRegistration
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		registration, err = createPendingRegistration(tx, student.ID, offerings[0].ID, req.Note)
		return err
	})
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": registrationSuccessMessage, "data": gin.H{"registration": registration}})
}
