package models

import (
	"fmt"
	"gorm.io/gorm"
)

// CourseOffering represents one course opened for one class in a semester.
// It is the central relation used by registration, teaching, grading and
// attendance flows.
type CourseOffering struct {
	ID uint `gorm:"primaryKey"`
	// Stable identity for this teaching section, separate from the subject and homeroom.
	OfferingCode string `gorm:"-"`

	CourseID uint   `gorm:"uniqueIndex:idx_course_offering"`
	Course   Course `gorm:"foreignKey:CourseID"`

	ClassID uint  `gorm:"uniqueIndex:idx_course_offering"`
	Class   Class `gorm:"foreignKey:ClassID"`

	TeacherID uint
	Teacher   Teacher `gorm:"foreignKey:TeacherID"`

	SemesterID uint     `gorm:"uniqueIndex:idx_course_offering"`
	Semester   Semester `gorm:"foreignKey:SemesterID"`

	RoomID uint
	Room   Room `gorm:"foreignKey:RoomID"`

	Status string `gorm:"default:open"`

	Enrollments        []Enrollment        `gorm:"foreignKey:CourseOfferingID"`
	Schedules          []Schedule          `gorm:"foreignKey:CourseOfferingID"`
	AttendanceSessions []AttendanceSession `gorm:"foreignKey:CourseOfferingID"`
}

func (offering *CourseOffering) AfterFind(_ *gorm.DB) error {
	offering.OfferingCode = fmt.Sprintf("LHP%06d", offering.ID)
	return nil
}
