package models

import "time"

type ProgramImage struct {
	Slug      string    `gorm:"primaryKey;size:100" json:"slug"`
	ImageURL  string    `gorm:"size:2048" json:"imageUrl"`
	UpdatedAt time.Time `json:"updatedAt"`
}
