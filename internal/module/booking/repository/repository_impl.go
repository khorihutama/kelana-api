package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/khorihutama/kelana-api/internal/module/booking/model/entity"
)

var ErrActiveReservation = errors.New("active reservation already exists")

type gormRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) GetSchedule(ctx context.Context, id uuid.UUID) (*ScheduleInfo, error) {
	var info ScheduleInfo
	err := r.db.WithContext(ctx).
		Table("trip_schedules").
		Select("id, trip_id, price_idr, quota_total, seats_sold, status").
		Where("id = ?", id).
		First(&info).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (r *gormRepository) ListHotels(ctx context.Context, tripID uuid.UUID) ([]HotelInfo, error) {
	var rows []HotelInfo
	err := r.db.WithContext(ctx).
		Table("trip_hotels").
		Select("id, extra_cost_idr").
		Where("trip_id = ?", tripID).
		Order("sort_order ASC").
		Scan(&rows).Error
	return rows, err
}

func (r *gormRepository) ListOpenSchedules(ctx context.Context) ([]ScheduleInfo, error) {
	var rows []ScheduleInfo
	err := r.db.WithContext(ctx).
		Table("trip_schedules").
		Select("id, trip_id, price_idr, quota_total, seats_sold, status").
		Where("status = ?", "OPEN").
		Scan(&rows).Error
	return rows, err
}

func (r *gormRepository) ListExpiredReservations(ctx context.Context, now time.Time, limit int) ([]ExpiredReservation, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []ExpiredReservation
	err := r.db.WithContext(ctx).
		Table("reservations").
		Select("id, schedule_id").
		Where("status = ? AND lock_expires_at <= ?", "PENDING_PAYMENT", now).
		Order("lock_expires_at ASC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

func (r *gormRepository) GetPromo(ctx context.Context, code string) (*entity.Promo, error) {
	var promo entity.Promo
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&promo).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &promo, nil
}

func (r *gormRepository) CreateReservation(ctx context.Context, reservation *entity.Reservation, participants []entity.Participant) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(reservation).Error; err != nil {
			return err
		}
		for i := range participants {
			participants[i].ReservationID = reservation.ID
			if err := tx.Create(&participants[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "reservations_active_unique" {
			return ErrActiveReservation
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "reservations_active_unique" {
			return ErrActiveReservation
		}
		if strings.Contains(err.Error(), "reservations_active_unique") {
			return ErrActiveReservation
		}
		return err
	}
	return nil
}

func (r *gormRepository) GetReservation(ctx context.Context, id uuid.UUID) (*entity.Reservation, error) {
	var reservation entity.Reservation
	err := r.db.WithContext(ctx).
		Preload("Participants").
		Where("id = ?", id).
		First(&reservation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &reservation, nil
}

func (r *gormRepository) CancelReservation(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	result := r.db.WithContext(ctx).Model(&entity.Reservation{}).
		Where("id = ? AND user_id = ? AND status = ?", id, userID, "PENDING_PAYMENT").
		Update("status", "CANCELLED")
	return result.RowsAffected == 1, result.Error
}

func (r *gormRepository) MarkExpired(ctx context.Context, id uuid.UUID) (bool, error) {
	result := r.db.WithContext(ctx).Model(&entity.Reservation{}).
		Where("id = ? AND status = ?", id, "PENDING_PAYMENT").
		Update("status", "EXPIRED")
	return result.RowsAffected == 1, result.Error
}

// MarkPaid converts a pending reservation to PAID under a row lock and enforces quota.
func (r *gormRepository) MarkPaid(ctx context.Context, id uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var reservation entity.Reservation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).First(&reservation).Error; err != nil {
			return err
		}
		if reservation.Status == "PAID" {
			return nil
		}
		if reservation.Status != "PENDING_PAYMENT" {
			return errors.New("reservation is not pending")
		}

		var schedule struct {
			QuotaTotal int
			SeatsSold  int
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Table("trip_schedules").
			Select("quota_total, seats_sold").
			Where("id = ?", reservation.ScheduleID).
			First(&schedule).Error; err != nil {
			return err
		}
		if schedule.SeatsSold+reservation.ParticipantsCount > schedule.QuotaTotal {
			return errors.New("quota exceeded")
		}

		if err := tx.Table("trip_schedules").
			Where("id = ?", reservation.ScheduleID).
			Update("seats_sold", gorm.Expr("seats_sold + ?", reservation.ParticipantsCount)).Error; err != nil {
			return err
		}
		return tx.Model(&entity.Reservation{}).
			Where("id = ?", id).
			Updates(map[string]any{"status": "PAID", "paid_at": now}).Error
	})
}
