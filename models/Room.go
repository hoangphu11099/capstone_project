package models

type Room struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"unique;not null"`
	Building    string
	Capacity    int
	Description string
	IsActive    bool `gorm:"default:true"`

	Classes         []Class
	RoomRegisters   []RoomRegister
	CourseOfferings []CourseOffering
}

type Class struct {
	ID        uint   `gorm:"primaryKey"`
	ClassCode string `gorm:"unique;not null"`

	MajorID uint
	Major   Major `gorm:"foreignKey:MajorID"`

	TeacherID uint    `gorm:"default:null"`
	Teacher   Teacher `gorm:"foreignKey:TeacherID"`

	SemesterID uint     `gorm:"default:null"`
	Semester   Semester `gorm:"foreignKey:SemesterID"`

	RoomID uint `gorm:"default:null"`
	Room   Room `gorm:"foreignKey:RoomID"`

	CohortYear  int
	MaxStudents int
	Status      string `gorm:"default:open"`

	Students         []Student
	Schedules        []Schedule
	Enrollments      []Enrollment
	RoomRegisters    []RoomRegister
	CourseOfferings  []CourseOffering
	CurrentStudents  int64 `gorm:"->;-:migration" json:"currentStudents"`
	CurrentOfferings int64 `gorm:"->;-:migration" json:"currentOfferings"`
}
