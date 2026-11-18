package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ListRow struct {
	ID                uuid.UUID  `gorm:"column:id"`
	Code              string     `gorm:"column:code"`
	Status            string     `gorm:"column:status"`
	ParticipantsCount int        `gorm:"column:participants_count"`
	TotalAmountIDR    int64      `gorm:"column:total_amount_idr"`
	DiscountIDR       int64      `gorm:"column:discount_idr"`
	Currency          string     `gorm:"column:currency"`
	PickupPoint       string     `gorm:"column:pickup_point"`
	Notes             *string    `gorm:"column:notes"`
	LockExpiresAt     time.Time  `gorm:"column:lock_expires_at"`
	PaymentExpiresAt  *time.Time `gorm:"column:payment_expires_at"`
	PaidAt            *time.Time `gorm:"column:paid_at"`
	CreatedAt         time.Time  `gorm:"column:created_at"`

	TripID          uuid.UUID `gorm:"column:trip_id"`
	TripSlug        string    `gorm:"column:trip_slug"`
	TripTitle       string    `gorm:"column:trip_title"`
	TripCategory    string    `gorm:"column:trip_category"`
	DepartureCity   string    `gorm:"column:departure_city"`
	DestinationCity string    `gorm:"column:destination_city"`
	TripWAGroupURL  *string   `gorm:"column:trip_wa_group_url"`

	ScheduleID uuid.UUID `gorm:"column:schedule_id"`
	StartDate  time.Time `gorm:"column:start_date"`
	EndDate    time.Time `gorm:"column:end_date"`

	PaymentID          *uuid.UUID `gorm:"column:payment_id"`
	PaymentStatus      *string    `gorm:"column:payment_status"`
	PaymentMethod      *string    `gorm:"column:payment_method"`
	PaymentAmountIDR   *int64     `gorm:"column:payment_amount_idr"`
	PaymentCurrency    *string    `gorm:"column:payment_currency"`
	PaymentExpiresAtDB *time.Time `gorm:"column:payment_expires_at_db"`
	PaymentPaidAt      *time.Time `gorm:"column:payment_paid_at"`
	PaymentVANumber    *string    `gorm:"column:payment_va_number"`
	PaymentQRString    *string    `gorm:"column:payment_qr_string"`
}

type ParticipantRow struct {
	ID       uuid.UUID `gorm:"column:id"`
	FullName string    `gorm:"column:full_name"`
	Gender   string    `gorm:"column:gender"`
	Email    string    `gorm:"column:email"`
	Phone    string    `gorm:"column:phone"`
}

type DocumentRow struct {
	ID            uuid.UUID `gorm:"column:id"`
	ParticipantID uuid.UUID `gorm:"column:participant_id"`
	Type          string    `gorm:"column:type"`
	MimeType      string    `gorm:"column:mime_type"`
	SizeBytes     int64     `gorm:"column:size_bytes"`
	Status        string    `gorm:"column:status"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

type Repository interface {
	List(ctx context.Context, userID uuid.UUID, tab, today string, page, limit int) ([]ListRow, int64, error)
	Get(ctx context.Context, userID, reservationID uuid.UUID) (*ListRow, error)
	ListParticipants(ctx context.Context, reservationID uuid.UUID) ([]ParticipantRow, error)
	ListDocuments(ctx context.Context, reservationID uuid.UUID) ([]DocumentRow, error)
}
