package entity

import (
	"time"

	"github.com/google/uuid"
)

type Document struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	ReservationID  uuid.UUID `gorm:"type:uuid;not null"`
	ParticipantID  uuid.UUID `gorm:"type:uuid;not null"`
	UserID         uuid.UUID `gorm:"type:uuid;not null"`
	Type           string    `gorm:"not null"`
	ObjectKey      string    `gorm:"not null;uniqueIndex"`
	MimeType       string    `gorm:"not null"`
	SizeBytes      int64     `gorm:"not null"`
	ChecksumSHA256 string    `gorm:"not null"`
	Status         string    `gorm:"not null;default:UPLOADED"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Document) TableName() string { return "documents" }
