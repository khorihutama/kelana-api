package entity

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	Email           string    `gorm:"type:citext;uniqueIndex;not null"`
	PasswordHash    string    `gorm:"not null"`
	Name            string    `gorm:"not null"`
	Phone           *string
	Role            string `gorm:"not null;default:CUSTOMER"`
	Status          string `gorm:"not null;default:ACTIVE"`
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (User) TableName() string { return "users" }
