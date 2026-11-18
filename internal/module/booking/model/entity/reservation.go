package entity

import (
	"time"

	"github.com/google/uuid"
)

type Reservation struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code              string    `gorm:"not null;uniqueIndex"`
	UserID            uuid.UUID `gorm:"type:uuid;not null"`
	TripID            uuid.UUID `gorm:"type:uuid;not null"`
	ScheduleID        uuid.UUID `gorm:"type:uuid;not null"`
	Status            string    `gorm:"not null"`
	ParticipantsCount int       `gorm:"not null"`
	UnitPriceIDR      int64     `gorm:"column:unit_price_idr;not null"`
	TotalAmountIDR    int64     `gorm:"column:total_amount_idr;not null"`
	DiscountIDR       int64     `gorm:"column:discount_idr;not null;default:0"`
	Currency          string    `gorm:"type:char(3);not null;default:IDR"`
	PickupPoint       string    `gorm:"not null"`
	Notes             *string
	HotelID           *uuid.UUID `gorm:"type:uuid"`
	PromoCodeID       *uuid.UUID `gorm:"type:uuid"`
	LockExpiresAt     time.Time  `gorm:"not null"`
	PaymentExpiresAt  *time.Time
	PaidAt            *time.Time
	ExpiresAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time

	Participants []Participant `gorm:"foreignKey:ReservationID"`
}

func (Reservation) TableName() string { return "reservations" }

type Participant struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	ReservationID     uuid.UUID `gorm:"type:uuid;not null"`
	FullName          string    `gorm:"not null"`
	Gender            string    `gorm:"not null"`
	Email             string    `gorm:"not null"`
	Phone             string    `gorm:"not null"`
	IDNumberEncrypted []byte
	IsPrimary         bool `gorm:"not null;default:false"`
	CreatedAt         time.Time
}

func (Participant) TableName() string { return "participants" }

type Promo struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code              string    `gorm:"not null"`
	DiscountType      string    `gorm:"not null"`
	DiscountValue     int       `gorm:"not null"`
	MinTransactionIDR int64     `gorm:"column:min_transaction_idr;not null"`
	Quota             *int
	UsedCount         int       `gorm:"not null"`
	ValidFrom         time.Time `gorm:"not null"`
	ValidTo           time.Time `gorm:"not null"`
	Active            bool      `gorm:"not null"`
}

func (Promo) TableName() string { return "promo_codes" }
