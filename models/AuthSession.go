package models

import "time"

type AuthSession struct {
	ID                    string    `gorm:"primaryKey;size:64" json:"-"`
	UserID                uint      `gorm:"index" json:"-"`
	CredentialFingerprint string    `gorm:"size:64;not null" json:"-"`
	ExpiresAt             time.Time `gorm:"index" json:"-"`
	CreatedAt             time.Time
}
