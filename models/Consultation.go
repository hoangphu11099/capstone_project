package models

import "time"

type Consultation struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	FullName         string    `gorm:"size:120;not null" json:"fullName"`
	Phone            string    `gorm:"size:30;index;not null" json:"phone"`
	Email            string    `gorm:"size:254;not null" json:"email"`
	Program          string    `gorm:"size:160;not null" json:"program"`
	Location         string    `gorm:"size:250" json:"location"`
	PreferredSession string    `gorm:"size:30" json:"preferredSession"`
	Message          string    `gorm:"type:text" json:"message"`
	ConsentAt        time.Time `json:"consentAt"`
	Status           string    `gorm:"size:30;default:new" json:"status"`
	CreatedAt        time.Time `gorm:"index" json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}
