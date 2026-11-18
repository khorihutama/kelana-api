package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/khorihutama/hsi-go-api/internal/module/document/model/entity"
)

type gormRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) GetParticipantContext(ctx context.Context, participantID uuid.UUID) (*ParticipantContext, error) {
	var out ParticipantContext
	err := r.db.WithContext(ctx).
		Table("participants AS p").
		Select("p.id AS participant_id, r.id AS reservation_id, r.user_id, r.status AS reservation_status, r.lock_expires_at, r.paid_at").
		Joins("JOIN reservations AS r ON r.id = p.reservation_id").
		Where("p.id = ?", participantID).
		Take(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *gormRepository) GetReservationContext(ctx context.Context, reservationID uuid.UUID) (*ReservationContext, error) {
	var out ReservationContext
	err := r.db.WithContext(ctx).
		Table("reservations AS r").
		Select("r.id, r.user_id, r.status, r.lock_expires_at, r.trip_id, t.required_documents").
		Joins("JOIN trips AS t ON t.id = r.trip_id").
		Where("r.id = ?", reservationID).
		Take(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *gormRepository) ListParticipants(ctx context.Context, reservationID uuid.UUID) ([]ParticipantInfo, error) {
	var rows []ParticipantInfo
	err := r.db.WithContext(ctx).
		Table("participants").
		Select("id, full_name").
		Where("reservation_id = ?", reservationID).
		Order("is_primary DESC, created_at ASC").
		Scan(&rows).Error
	return rows, err
}

func (r *gormRepository) ListDocuments(ctx context.Context, reservationID uuid.UUID) ([]DocumentInfo, error) {
	var rows []DocumentInfo
	err := r.db.WithContext(ctx).
		Model(&entity.Document{}).
		Select("participant_id, type").
		Where("reservation_id = ? AND status <> ?", reservationID, "REJECTED").
		Scan(&rows).Error
	return rows, err
}

func (r *gormRepository) GetDocument(ctx context.Context, id uuid.UUID) (*entity.Document, error) {
	var doc entity.Document
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&doc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *gormRepository) CreateDocument(ctx context.Context, doc *entity.Document) error {
	return r.db.WithContext(ctx).Create(doc).Error
}

func (r *gormRepository) DeleteDocument(ctx context.Context, id uuid.UUID) (bool, error) {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&entity.Document{})
	return result.RowsAffected == 1, result.Error
}
