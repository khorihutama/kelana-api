package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/khorihutama/kelana-api/internal/module/payment/model/entity"
)

var (
	ErrPaymentExists     = errors.New("payment already exists")
	ErrPaymentNotFound   = errors.New("payment not found")
	ErrReservationState  = errors.New("reservation is not payable")
	ErrLockExpired       = errors.New("reservation lock expired")
	ErrQuotaExceeded     = errors.New("schedule quota exceeded")
	ErrAmountMismatch    = errors.New("webhook amount mismatch")
	ErrProviderMismatch  = errors.New("webhook provider mismatch")
	ErrPaymentNotPending = errors.New("payment is not pending")
)

type ReservationInfo struct {
	ID               uuid.UUID  `gorm:"column:id"`
	Code             string     `gorm:"column:code"`
	UserID           uuid.UUID  `gorm:"column:user_id"`
	ScheduleID       uuid.UUID  `gorm:"column:schedule_id"`
	Status           string     `gorm:"column:status"`
	Participants     int        `gorm:"column:participants_count"`
	TotalAmountIDR   int64      `gorm:"column:total_amount_idr"`
	LockExpiresAt    time.Time  `gorm:"column:lock_expires_at"`
	PaymentExpiresAt *time.Time `gorm:"column:payment_expires_at"`
}

type ConfirmKind string

const (
	ConfirmPaid        ConfirmKind = "PAID"
	ConfirmAlreadyPaid ConfirmKind = "ALREADY_PAID"
	ConfirmDuplicate   ConfirmKind = "DUPLICATE"
)

type ConfirmOutcome struct {
	Kind          ConfirmKind
	ReservationID uuid.UUID
	ScheduleID    uuid.UUID
}

type ExpiredPayment struct {
	PaymentID     uuid.UUID
	ReservationID uuid.UUID
	ScheduleID    uuid.UUID
	ReleaseSeat   bool
}

type Repository interface {
	GetReservationForPayment(ctx context.Context, id uuid.UUID) (*ReservationInfo, error)
	GetPaymentByID(ctx context.Context, id uuid.UUID) (*entity.Payment, error)
	GetPaymentByReservation(ctx context.Context, reservationID uuid.UUID) (*entity.Payment, error)
	CreatePayment(ctx context.Context, payment *entity.Payment, paymentExpiresAt time.Time) error
	ConfirmWebhook(ctx context.Context, provider string, event *entity.PaymentEvent, now time.Time) (ConfirmOutcome, error)
	ExpireDuePayments(ctx context.Context, now time.Time, limit int) ([]ExpiredPayment, error)
}
