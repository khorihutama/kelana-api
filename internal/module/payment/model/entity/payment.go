package entity

import (
	"time"

	"github.com/google/uuid"
)

type Payment struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	ReservationID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`
	Provider      string    `gorm:"not null"`
	ProviderRef   string    `gorm:"not null"`
	Method        string    `gorm:"not null"`
	AmountIDR     int64     `gorm:"column:amount_idr;not null"`
	Currency      string    `gorm:"type:char(3);not null;default:IDR"`
	Status        string    `gorm:"not null;default:PENDING"`
	VANumber      *string
	QRString      *string
	ExpiresAt     time.Time `gorm:"not null"`
	PaidAt        *time.Time
	RawResponse   *string `gorm:"type:jsonb"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (Payment) TableName() string { return "payments" }

type PaymentEvent struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	PaymentID       uuid.UUID `gorm:"type:uuid;not null"`
	Provider        string    `gorm:"not null"`
	ProviderEventID string    `gorm:"not null"`
	EventType       string    `gorm:"not null"`
	SignatureValid  bool      `gorm:"not null;default:false"`
	Payload         string    `gorm:"type:jsonb;not null"`
	ProcessedAt     time.Time `gorm:"not null;default:now()"`

	// Runtime-only fields used by the webhook transaction; not columns.
	ProviderRef string `gorm:"-"`
	AmountIDR   int64  `gorm:"-"`
}

func (PaymentEvent) TableName() string { return "payment_events" }
