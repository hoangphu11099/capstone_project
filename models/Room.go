package models

type Room struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"unique;not null"`
	Building    string
	Capacity    int
	Description string
	IsActive    bool `gorm:"default:true"`

	Classes       []Class
	RoomRegisters []RoomRegister
}

type Class struct {
	ID        uint   `gorm:"primaryKey"`
	ClassCode string `gorm:"unique;not null"`

	MajorID uint
	Major   Major `gorm:"foreignKey:MajorID"`

	TeacherID uint
	Teacher   Teacher `gorm:"foreignKey:TeacherID"`

	SemesterID uint
	Semester   Semester `gorm:"foreignKey:SemesterID"`

	RoomID uint
	Room   Room `gorm:"foreignKey:RoomID"`

	MaxStudents int
	Status      string `gorm:"default:open"`

	Students      []Student
	Schedules     []Schedule
	Enrollments   []Enrollment
	RoomRegisters []RoomRegister
}
