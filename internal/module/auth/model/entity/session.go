package entity

import (
	"time"

	"github.com/google/uuid"
)

type RefreshSession struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID `gorm:"type:uuid;not null"`
	TokenHash  string    `gorm:"uniqueIndex;not null"`
	FamilyID   uuid.UUID `gorm:"type:uuid;not null"`
	UserAgent  *string
	IP         *string
	ExpiresAt  time.Time `gorm:"not null"`
	RevokedAt  *time.Time
	ReplacedBy *uuid.UUID `gorm:"type:uuid"`
	CreatedAt  time.Time
}

func (RefreshSession) TableName() string { return "refresh_sessions" }

type EmailVerification struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID `gorm:"type:uuid;not null"`
	CodeHash   string    `gorm:"not null"`
	ExpiresAt  time.Time `gorm:"not null"`
	ConsumedAt *time.Time
	Attempts   int `gorm:"not null;default:0"`
	CreatedAt  time.Time
}

func (EmailVerification) TableName() string { return "email_verifications" }
