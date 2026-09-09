package models

type User struct {
	ID         uint   `gorm:"primaryKey"`
	Username   string `gorm:"unique;not null"`
	Password   string `gorm:"not null" json:"-"`
	Email      string `gorm:"unique"`
	FullName   string
	RoleID     uint
	Role       Role `gorm:"foreignKey:RoleID"`
	IsActive   bool `gorm:"default:true"`
	FirstLogin bool `gorm:"default:true"`

	Teacher *Teacher
	Student *Student
}
