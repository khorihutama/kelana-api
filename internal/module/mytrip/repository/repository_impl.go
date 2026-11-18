package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const listSelect = `
	r.id, r.code, r.status, r.participants_count, r.total_amount_idr, r.discount_idr,
	r.currency, r.pickup_point, r.notes, r.lock_expires_at, r.payment_expires_at,
	r.paid_at, r.created_at,
	t.id AS trip_id, t.slug AS trip_slug, t.title AS trip_title, t.category AS trip_category,
	t.departure_city, t.destination_city, t.wa_group_url AS trip_wa_group_url,
	s.id AS schedule_id, s.start_date, s.end_date,
	p.id AS payment_id, p.status AS payment_status, p.method AS payment_method,
	p.amount_idr AS payment_amount_idr, p.currency AS payment_currency,
	p.expires_at AS payment_expires_at_db, p.paid_at AS payment_paid_at,
	p.va_number AS payment_va_number, p.qr_string AS payment_qr_string`

const listFrom = `
	FROM reservations r
	JOIN trips t ON t.id = r.trip_id
	JOIN trip_schedules s ON s.id = r.schedule_id
	LEFT JOIN payments p ON p.reservation_id = r.id`

type gormRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) List(ctx context.Context, userID uuid.UUID, tab, today string, page, limit int) ([]ListRow, int64, error) {
	where, args := tabWhere(userID, tab, today)
	var total int64
	if err := r.db.WithContext(ctx).
		Table("reservations AS r").
		Joins("JOIN trip_schedules AS s ON s.id = r.schedule_id").
		Where(where, args...).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []ListRow{}, 0, nil
	}

	var rows []ListRow
	err := r.db.WithContext(ctx).
		Raw(`SELECT `+listSelect+listFrom+` WHERE `+where+` ORDER BY r.created_at DESC, r.id DESC LIMIT ? OFFSET ?`,
			append(args, limit, (page-1)*limit)...).
		Scan(&rows).Error
	return rows, total, err
}

func (r *gormRepository) Get(ctx context.Context, userID, reservationID uuid.UUID) (*ListRow, error) {
	var row ListRow
	err := r.db.WithContext(ctx).
		Raw(`SELECT `+listSelect+listFrom+` WHERE r.user_id = ? AND r.id = ? LIMIT 1`, userID, reservationID).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == uuid.Nil {
		return nil, nil
	}
	return &row, nil
}

func (r *gormRepository) ListParticipants(ctx context.Context, reservationID uuid.UUID) ([]ParticipantRow, error) {
	var rows []ParticipantRow
	err := r.db.WithContext(ctx).
		Table("participants").
		Select("id, full_name, gender, email, phone").
		Where("reservation_id = ?", reservationID).
		Order("is_primary DESC, created_at ASC").
		Scan(&rows).Error
	return rows, err
}

func (r *gormRepository) ListDocuments(ctx context.Context, reservationID uuid.UUID) ([]DocumentRow, error) {
	var rows []DocumentRow
	err := r.db.WithContext(ctx).
		Table("documents").
		Select("id, participant_id, type, mime_type, size_bytes, status, created_at").
		Where("reservation_id = ?", reservationID).
		Order("created_at ASC").
		Scan(&rows).Error
	return rows, err
}

func tabWhere(userID uuid.UUID, tab, today string) (string, []any) {
	where := "r.user_id = ?"
	args := []any{userID}
	switch tab {
	case "BELUM_BAYAR":
		where += " AND r.status = 'PENDING_PAYMENT'"
	case "SIAP_BERANGKAT":
		where += " AND r.status = 'PAID' AND s.end_date >= ?::date"
		args = append(args, today)
	case "SELESAI":
		where += " AND (r.status = 'COMPLETED' OR (r.status = 'PAID' AND s.end_date < ?::date))"
		args = append(args, today)
	case "RIWAYAT":
		where += " AND r.status IN ('CANCELLED', 'EXPIRED')"
	}
	return where, args
}
