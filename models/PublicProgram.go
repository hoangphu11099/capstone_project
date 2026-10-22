package models

import "time"

type PublicProgram struct {
	Slug      string `gorm:"primaryKey;size:100"`
	Content   string `gorm:"type:longtext;not null"`
	UpdatedAt time.Time
}
