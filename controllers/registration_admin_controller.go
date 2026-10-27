package controllers

import (
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"student-management/config"
	"student-management/middleware"
	"student-management/models"
	"time"
)

const registrationSuccessMessage = "Đã đăng ký học phần thành công. Vui lòng liên hệ học vụ để đóng học phí. Lịch học sẽ được cập nhật sau khi học vụ xác nhận và xếp lịch."

func createPendingRegistration(tx *gorm.DB, studentID, offeringID uint, note string) (models.CourseRegistration, error) {
	var row models.CourseRegistration
	// Serialize competing requests from the same student and for the last seat.
	var student models.Student
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&student, studentID).Error; err != nil {
		return row, err
	}
	var offering models.CourseOffering
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Class").Preload("Course").Preload("Semester").First(&offering, offeringID).Error; err != nil {
		return row, err
	}
	major, err := registrationMajor(tx, student)
	if err != nil {
		return row, err
	}
	if offering.Course.MajorID != major.ID || offering.Class.MajorID != major.ID {
		return row, errors.New("Chỉ được đăng ký học phần thuộc chuyên ngành sinh viên đang học")
	}
	if offering.Status != "open" || offering.Class.Status != "open" || offering.Semester.Status != "active" {
		return row, errors.New("Học phần hoặc học kỳ đã đóng đăng ký")
	}
	var enrolled, pending int64
	if err := tx.Model(&models.Enrollment{}).Joins("JOIN course_offerings co ON co.id = enrollments.course_offering_id").Where("enrollments.student_id = ? AND enrollments.course_id = ? AND co.semester_id = ? AND enrollments.status <> 'cancelled'", studentID, offering.CourseID, offering.SemesterID).Count(&enrolled).Error; err != nil {
		return row, err
	}
	if err := tx.Model(&models.CourseRegistration{}).Joins("JOIN course_offerings co ON co.id = course_registrations.course_offering_id").Where("course_registrations.student_id = ? AND course_registrations.course_id = ? AND co.semester_id = ? AND course_registrations.status IN ?", studentID, offering.CourseID, offering.SemesterID, []string{"pending_payment", "paid", "scheduled"}).Count(&pending).Error; err != nil {
		return row, err
	}
	if enrolled+pending > 0 {
		return row, errors.New("Sinh viên đã đăng ký môn học này trong học kỳ")
	}
	if err := tx.Model(&models.Enrollment{}).Where("course_offering_id = ? AND status <> 'cancelled'", offering.ID).Count(&enrolled).Error; err != nil {
		return row, err
	}
	if err := tx.Model(&models.CourseRegistration{}).Where("course_offering_id = ? AND status IN ?", offering.ID, []string{"pending_payment", "paid"}).Count(&pending).Error; err != nil {
		return row, err
	}
	if offering.Class.MaxStudents > 0 && enrolled+pending >= int64(offering.Class.MaxStudents) {
		return row, errors.New("Lớp học phần đã đủ sĩ số")
	}
	row = models.CourseRegistration{CourseOfferingID: &offeringID, StudentID: studentID, CourseID: offering.CourseID, ClassID: offering.ClassID, Status: "pending_payment", RegisteredAt: time.Now(), Note: note}
	return row, tx.Create(&row).Error
}

func registrationID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"message": "ID đăng ký không hợp lệ"})
		return 0, false
	}
	return uint(id), true
}

func ListAdminCourseRegistrations(c *gin.Context) {
	var rows []models.CourseRegistration
	query := config.DB.Preload("Enrollment.CourseOffering.Semester").Preload("Enrollment.CourseOffering.Teacher.User").Preload("Enrollment.CourseOffering.Room").Preload("Enrollment.CourseOffering.Schedules").Preload("Student.User").Preload("Course").Preload("Class").Preload("CourseOffering.Semester").Preload("CourseOffering.Teacher.User").Preload("CourseOffering.Room").Preload("CourseOffering.Schedules")
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Order("registered_at desc, id desc").Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"message": "Không tải được danh sách đăng ký"})
		return
	}
	for i := range rows {
		if rows[i].CourseOffering == nil && rows[i].Enrollment != nil {
			rows[i].CourseOffering = rows[i].Enrollment.CourseOffering
		}
	}
	c.JSON(200, gin.H{"data": rows})
}

func ConfirmRegistrationPayment(c *gin.Context) {
	id, ok := registrationID(c)
	if !ok {
		return
	}
	userID, _ := c.Get(middleware.ContextUserIDKey)
	keptSchedule := false
	var saved models.CourseRegistration
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var row models.CourseRegistration
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error; err != nil {
			return err
		}
		if row.Status == "paid" || row.Status == "scheduled" {
			keptSchedule = row.Status == "scheduled"
			saved = row
			return nil
		}
		if row.Status != "pending_payment" && row.Status != "registered" {
			return errors.New("Chỉ xác nhận học phí cho đăng ký đang chờ thanh toán")
		}
		updates := map[string]interface{}{"status": "paid", "payment_confirmed_at": time.Now(), "payment_confirmed_by": userID}
		if row.Status == "registered" {
			// Resolve the exact historical offering; never infer payment or rebuild an enrollment.
			var enrollment models.Enrollment
			if row.EnrollmentID != nil {
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&enrollment, *row.EnrollmentID).Error; err != nil {
					return err
				}
				if enrollment.StudentID != row.StudentID || enrollment.CourseID != row.CourseID || enrollment.Status == "cancelled" {
					return errors.New("Hồ sơ học cũ không khớp hoặc đã hủy; vui lòng kiểm tra lại đăng ký")
				}
				if enrollment.CourseOfferingID != nil {
					row.CourseOfferingID = enrollment.CourseOfferingID
				}
			}
			if row.CourseOfferingID == nil {
				var candidates []models.CourseOffering
				if err := tx.Where("class_id = ? AND course_id = ?", row.ClassID, row.CourseID).Find(&candidates).Error; err != nil {
					return err
				}
				if len(candidates) != 1 {
					return errors.New("Chưa xác định được học phần của đăng ký cũ. Vui lòng kiểm tra lớp, môn và học kỳ trước khi xác nhận học phí")
				}
				row.CourseOfferingID = &candidates[0].ID
			}
			updates["course_offering_id"] = *row.CourseOfferingID
			var lessons int64
			if err := tx.Model(&models.Schedule{}).Where("course_offering_id = ?", *row.CourseOfferingID).Count(&lessons).Error; err != nil {
				return err
			}
			if enrollment.ID != 0 && lessons > 0 {
				updates["status"] = "scheduled"
				keptSchedule = true
			}
		}
		if err := tx.Model(&row).Updates(updates).Error; err != nil {
			return err
		}
		return tx.First(&saved, row.ID).Error
	})
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	message := "Đã xác nhận học phí. Đăng ký đang chờ xếp lịch."
	if keptSchedule {
		message = "Đã xác nhận học phí. Lịch học và dữ liệu học tập hiện có được giữ nguyên."
	}
	c.JSON(200, gin.H{"message": message, "data": gin.H{"ID": saved.ID, "Status": saved.Status, "PaymentConfirmedAt": saved.PaymentConfirmedAt, "CourseOfferingID": saved.CourseOfferingID}})
}

type ScheduleRegistrationRequest struct {
	CourseOfferingID uint `json:"courseOfferingId"`
	// Course and semester are derived from the registered offering below.
	Assignment *AssignCourseOfferingRequest `json:"assignment" binding:"-"`
}

func ScheduleCourseRegistration(c *gin.Context) {
	id, ok := registrationID(c)
	if !ok {
		return
	}
	var req ScheduleRegistrationRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"message": "Dữ liệu xếp lịch không hợp lệ"})
		return
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var row models.CourseRegistration
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error; err != nil {
			return err
		}
		if row.Status == "scheduled" {
			return nil
		}
		if row.Status != "paid" || row.PaymentConfirmedAt == nil {
			return errors.New("Phải xác nhận đã đóng học phí trước khi xếp lịch")
		}
		var student models.Student
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&student, row.StudentID).Error; err != nil {
			return err
		}
		var original, target models.CourseOffering
		if row.CourseOfferingID == nil {
			return errors.New("Đăng ký cũ chưa có thông tin học phần")
		}
		if err := tx.First(&original, *row.CourseOfferingID).Error; err != nil {
			return err
		}
		targetID := req.CourseOfferingID
		if targetID == 0 {
			targetID = original.ID
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Class").Preload("Semester").Preload("Schedules").First(&target, targetID).Error; err != nil {
			return err
		}
		if target.CourseID != row.CourseID || target.SemesterID != original.SemesterID {
			return errors.New("Học phần xếp lịch phải cùng môn và học kỳ đã đăng ký")
		}
		if target.Status != "open" || target.Class.Status != "open" || target.Semester.Status != "active" {
			return errors.New("Học phần, lớp hoặc học kỳ không còn mở")
		}
		if req.Assignment != nil {
			assignment := *req.Assignment
			if assignment.TeacherID == 0 || assignment.RoomID == 0 {
				return errors.New("Vui lòng chọn giảng viên và phòng học")
			}
			assignment.CourseID = target.CourseID
			assignment.SemesterID = target.SemesterID
			candidates, err := assignmentSchedules(assignment)
			if err != nil {
				return err
			}
			if err := validateFutureTeachingDates(candidates, attendanceNow()); err != nil {
				return err
			}
			if err := applyCourseAssignment(tx, target.ClassID, assignment, candidates, &target, false); err != nil {
				return err
			}
			if err := tx.Preload("Class").Preload("Semester").Preload("Schedules").First(&target, target.ID).Error; err != nil {
				return err
			}
		}
		if len(target.Schedules) == 0 || target.TeacherID == 0 || target.RoomID == 0 {
			return errors.New("Vui lòng bổ sung giảng viên, phòng và lịch học trước khi xếp sinh viên")
		}
		// Prevent clashes with lessons in the student's other classes.
		var others []models.Schedule
		if err := tx.Joins("JOIN enrollments e ON e.course_offering_id = schedules.course_offering_id").Preload("CourseOffering.Semester").Where("e.student_id = ? AND e.status <> 'cancelled' AND e.course_offering_id <> ?", row.StudentID, target.ID).Find(&others).Error; err != nil {
			return err
		}
		for _, a := range target.Schedules {
			for _, b := range others {
				if b.CourseOffering != nil && (b.CourseOffering.Semester.EndDate.Before(target.Semester.StartDate) || target.Semester.EndDate.Before(b.CourseOffering.Semester.StartDate)) {
					continue
				}
				if a.FullDate != nil && b.CourseOffering != nil && (a.FullDate.Format("2006-01-02") < b.CourseOffering.Semester.StartDate.Format("2006-01-02") || a.FullDate.Format("2006-01-02") > b.CourseOffering.Semester.EndDate.Format("2006-01-02")) {
					continue
				}
				if b.FullDate != nil && (b.FullDate.Format("2006-01-02") < target.Semester.StartDate.Format("2006-01-02") || b.FullDate.Format("2006-01-02") > target.Semester.EndDate.Format("2006-01-02")) {
					continue
				}
				if schedulesOverlap(a, b) {
					return errors.New("Lịch học bị trùng với một học phần khác của sinh viên")
				}
			}
		}
		var count int64
		if err := tx.Model(&models.Enrollment{}).Where("course_offering_id = ? AND student_id <> ? AND status <> 'cancelled'", target.ID, row.StudentID).Count(&count).Error; err != nil {
			return err
		}
		var reserved int64
		if err := tx.Model(&models.CourseRegistration{}).Where("course_offering_id = ? AND id <> ? AND status IN ?", target.ID, row.ID, []string{"pending_payment", "paid"}).Count(&reserved).Error; err != nil {
			return err
		}
		if target.Class.MaxStudents > 0 && count+reserved >= int64(target.Class.MaxStudents) {
			return errors.New("Lớp đã đủ sĩ số")
		}
		var enrollment models.Enrollment
		result := tx.Where("student_id = ? AND course_offering_id = ?", row.StudentID, target.ID).First(&enrollment)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return result.Error
		}
		enrollment.StudentID = row.StudentID
		enrollment.CourseID = row.CourseID
		enrollment.ClassID = target.ClassID
		enrollment.CourseOfferingID = &targetID
		enrollment.Status = "enrolled"
		if enrollment.EnrollDate.IsZero() {
			enrollment.EnrollDate = time.Now()
		}
		if err := tx.Save(&enrollment).Error; err != nil {
			return err
		}
		return tx.Model(&row).Updates(map[string]interface{}{"status": "scheduled", "enrollment_id": enrollment.ID, "course_offering_id": target.ID, "class_id": target.ClassID, "scheduled_at": time.Now()}).Error
	})
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Đã xếp lịch. Học phần đã được thêm vào thời khóa biểu sinh viên."})
}

func CancelPendingRegistration(c *gin.Context) {
	student, ok := getCurrentStudent(c)
	if !ok {
		return
	}
	id, ok := registrationID(c)
	if !ok {
		return
	}
	result := config.DB.Model(&models.CourseRegistration{}).Where("id = ? AND student_id = ? AND status = 'pending_payment'", id, student.ID).Updates(map[string]interface{}{"status": "cancelled", "canceled_at": time.Now()})
	if result.Error != nil || result.RowsAffected != 1 {
		c.JSON(400, gin.H{"message": "Chỉ được hủy đăng ký chưa xác nhận học phí. Vui lòng liên hệ học vụ nếu đã đóng học phí."})
		return
	}
	c.JSON(200, gin.H{"message": "Đã hủy đăng ký học phần"})
}
