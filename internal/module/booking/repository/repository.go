package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/khorihutama/hsi-go-api/internal/module/booking/model/entity"
)

type ScheduleInfo struct {
	ID         uuid.UUID `gorm:"column:id"`
	TripID     uuid.UUID `gorm:"column:trip_id"`
	PriceIDR   int64     `gorm:"column:price_idr"`
	QuotaTotal int       `gorm:"column:quota_total"`
	SeatsSold  int       `gorm:"column:seats_sold"`
	Status     string    `gorm:"column:status"`
}

type HotelInfo struct {
	ID           uuid.UUID `gorm:"column:id"`
	ExtraCostIDR int64     `gorm:"column:extra_cost_idr"`
}

type Repository interface {
	GetSchedule(ctx context.Context, id uuid.UUID) (*ScheduleInfo, error)
	ListHotels(ctx context.Context, tripID uuid.UUID) ([]HotelInfo, error)
	ListOpenSchedules(ctx context.Context) ([]ScheduleInfo, error)
	GetPromo(ctx context.Context, code string) (*entity.Promo, error)
	CreateReservation(ctx context.Context, reservation *entity.Reservation, participants []entity.Participant) error
	GetReservation(ctx context.Context, id uuid.UUID) (*entity.Reservation, error)
	CancelReservation(ctx context.Context, id, userID uuid.UUID) (bool, error)
	MarkExpired(ctx context.Context, id uuid.UUID) (bool, error)
	MarkPaid(ctx context.Context, id uuid.UUID, now time.Time) error
}
