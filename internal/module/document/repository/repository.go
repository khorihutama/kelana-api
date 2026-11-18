package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/khorihutama/hsi-go-api/internal/module/document/model/entity"
)

type ParticipantContext struct {
	ParticipantID    uuid.UUID  `gorm:"column:participant_id"`
	ReservationID    uuid.UUID  `gorm:"column:reservation_id"`
	UserID           uuid.UUID  `gorm:"column:user_id"`
	ReservationState string     `gorm:"column:reservation_status"`
	LockExpiresAt    time.Time  `gorm:"column:lock_expires_at"`
	PaidAt           *time.Time `gorm:"column:paid_at"`
}

type ParticipantInfo struct {
	ID       uuid.UUID `gorm:"column:id"`
	FullName string    `gorm:"column:full_name"`
}

type DocumentInfo struct {
	ParticipantID uuid.UUID `gorm:"column:participant_id"`
	Type          string    `gorm:"column:type"`
}

type ReservationContext struct {
	ReservationID     uuid.UUID      `gorm:"column:id"`
	UserID            uuid.UUID      `gorm:"column:user_id"`
	Status            string         `gorm:"column:status"`
	LockExpiresAt     time.Time      `gorm:"column:lock_expires_at"`
	TripID            uuid.UUID      `gorm:"column:trip_id"`
	RequiredDocuments pq.StringArray `gorm:"column:required_documents;type:text[]"`
}

type Repository interface {
	GetParticipantContext(ctx context.Context, participantID uuid.UUID) (*ParticipantContext, error)
	GetReservationContext(ctx context.Context, reservationID uuid.UUID) (*ReservationContext, error)
	ListParticipants(ctx context.Context, reservationID uuid.UUID) ([]ParticipantInfo, error)
	ListDocuments(ctx context.Context, reservationID uuid.UUID) ([]DocumentInfo, error)
	GetDocument(ctx context.Context, id uuid.UUID) (*entity.Document, error)
	CreateDocument(ctx context.Context, doc *entity.Document) error
	DeleteDocument(ctx context.Context, id uuid.UUID) (bool, error)
}
