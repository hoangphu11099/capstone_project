package models

import "time"

type RoomRegister struct {
	ID uint `gorm:"primaryKey"`

	ClassID uint
	Class   Class `gorm:"foreignKey:ClassID"`

	RoomID uint
	Room   Room `gorm:"foreignKey:RoomID"`

	Date time.Time
	Time string
	Note string
}
