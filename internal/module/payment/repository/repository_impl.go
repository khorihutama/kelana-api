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

	"github.com/khorihutama/kelana-api/internal/module/payment/model/entity"
)

type gormRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) GetReservationForPayment(ctx context.Context, id uuid.UUID) (*ReservationInfo, error) {
	var out ReservationInfo
	err := r.db.WithContext(ctx).
		Table("reservations").
		Select("id, code, user_id, schedule_id, status, participants_count, total_amount_idr, lock_expires_at, payment_expires_at").
		Where("id = ?", id).
		Take(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *gormRepository) GetPaymentByID(ctx context.Context, id uuid.UUID) (*entity.Payment, error) {
	var payment entity.Payment
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&payment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

func (r *gormRepository) GetPaymentByReservation(ctx context.Context, reservationID uuid.UUID) (*entity.Payment, error) {
	var payment entity.Payment
	err := r.db.WithContext(ctx).Where("reservation_id = ?", reservationID).Take(&payment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

func (r *gormRepository) CreatePayment(ctx context.Context, payment *entity.Payment, paymentExpiresAt time.Time) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(payment).Error; err != nil {
			return err
		}
		result := tx.Table("reservations").
			Where("id = ? AND status = ?", payment.ReservationID, "PENDING_PAYMENT").
			Update("payment_expires_at", paymentExpiresAt)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrReservationState
		}
		return nil
	})
	if err == nil {
		return nil
	}
	if isUniqueViolation(err, "payments_reservation_id_key") {
		return ErrPaymentExists
	}
	return err
}

func (r *gormRepository) ConfirmWebhook(ctx context.Context, provider string, event *entity.PaymentEvent, now time.Time) (ConfirmOutcome, error) {
	if event.PaymentID == uuid.Nil {
		return ConfirmOutcome{}, ErrPaymentNotFound
	}
	var outcome ConfirmOutcome
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(event)
		if insert.Error != nil {
			return insert.Error
		}
		if insert.RowsAffected == 0 {
			outcome.Kind = ConfirmDuplicate
			return nil
		}

		var payment entity.Payment
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("provider = ? AND id = ?", strings.ToUpper(provider), event.PaymentID)
		if err := query.Take(&payment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPaymentNotFound
			}
			return err
		}
		if event.AmountIDR > 0 && event.AmountIDR != payment.AmountIDR {
			return ErrAmountMismatch
		}
		if payment.Status == "PAID" {
			outcome.Kind = ConfirmAlreadyPaid
			return nil
		}
		if payment.Status != "PENDING" {
			return ErrPaymentNotPending
		}

		var reservation struct {
			ID                uuid.UUID `gorm:"column:id"`
			ScheduleID        uuid.UUID `gorm:"column:schedule_id"`
			Status            string    `gorm:"column:status"`
			ParticipantsCount int       `gorm:"column:participants_count"`
			LockExpiresAt     time.Time `gorm:"column:lock_expires_at"`
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Table("reservations").
			Select("id, schedule_id, status, participants_count, lock_expires_at").
			Where("id = ?", payment.ReservationID).
			Take(&reservation).Error; err != nil {
			return err
		}
		if reservation.Status == "PAID" {
			if err := tx.Model(&entity.Payment{}).
				Where("id = ?", payment.ID).
				Updates(map[string]any{"status": "PAID", "paid_at": now}).Error; err != nil {
				return err
			}
			outcome.Kind = ConfirmAlreadyPaid
			return nil
		}
		if reservation.Status != "PENDING_PAYMENT" || now.After(reservation.LockExpiresAt) {
			return ErrLockExpired
		}

		var schedule struct {
			QuotaTotal int `gorm:"column:quota_total"`
			SeatsSold  int `gorm:"column:seats_sold"`
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Table("trip_schedules").
			Select("quota_total, seats_sold").
			Where("id = ?", reservation.ScheduleID).
			Take(&schedule).Error; err != nil {
			return err
		}
		if schedule.SeatsSold+reservation.ParticipantsCount > schedule.QuotaTotal {
			return ErrQuotaExceeded
		}
		if err := tx.Table("trip_schedules").
			Where("id = ?", reservation.ScheduleID).
			Update("seats_sold", gorm.Expr("seats_sold + ?", reservation.ParticipantsCount)).Error; err != nil {
			return err
		}
		if err := tx.Table("reservations").
			Where("id = ?", reservation.ID).
			Updates(map[string]any{"status": "PAID", "paid_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&entity.Payment{}).
			Where("id = ?", payment.ID).
			Updates(map[string]any{"status": "PAID", "paid_at": now}).Error; err != nil {
			return err
		}
		outcome = ConfirmOutcome{
			Kind:          ConfirmPaid,
			ReservationID: reservation.ID,
			ScheduleID:    reservation.ScheduleID,
		}
		return nil
	})
	return outcome, err
}

// ExpireDuePayments marks at most limit pending payments as EXPIRED. When the
// reservation is still locked, it expires the reservation too and reports that
// the caller must release the Redis seat hold.
func (r *gormRepository) ExpireDuePayments(ctx context.Context, now time.Time, limit int) ([]ExpiredPayment, error) {
	if limit <= 0 {
		limit = 100
	}
	var out []ExpiredPayment
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var due []struct {
			ID            uuid.UUID `gorm:"column:id"`
			ReservationID uuid.UUID `gorm:"column:reservation_id"`
		}
		if err := tx.Raw(`
			SELECT id, reservation_id
			FROM payments
			WHERE status = 'PENDING' AND expires_at <= ?
			ORDER BY expires_at ASC
			LIMIT ?
			FOR UPDATE SKIP LOCKED
		`, now, limit).Scan(&due).Error; err != nil {
			return err
		}
		for _, payment := range due {
			update := tx.Model(&entity.Payment{}).
				Where("id = ? AND status = 'PENDING'", payment.ID).
				Updates(map[string]any{"status": "EXPIRED", "updated_at": now})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				continue
			}
			item := ExpiredPayment{PaymentID: payment.ID, ReservationID: payment.ReservationID}
			var reservation struct {
				Status     string    `gorm:"column:status"`
				ScheduleID uuid.UUID `gorm:"column:schedule_id"`
			}
			if err := tx.Table("reservations").
				Select("status, schedule_id").
				Where("id = ?", payment.ReservationID).
				Take(&reservation).Error; err != nil {
				return err
			}
			if reservation.Status == "PENDING_PAYMENT" {
				result := tx.Table("reservations").
					Where("id = ? AND status = 'PENDING_PAYMENT'", payment.ReservationID).
					Update("status", "EXPIRED")
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected == 1 {
					item.ReleaseSeat = true
					item.ScheduleID = reservation.ScheduleID
				}
			}
			out = append(out, item)
		}
		return nil
	})
	return out, err
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && (constraint == "" || pgErr.ConstraintName == constraint) {
		return true
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" && (constraint == "" || pqErr.Constraint == constraint) {
		return true
	}
	return constraint != "" && strings.Contains(err.Error(), constraint)
}
