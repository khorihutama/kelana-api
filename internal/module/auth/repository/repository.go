package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/khorihutama/kelana-api/internal/module/auth/model/entity"
)

type Repository interface {
	CreateUser(ctx context.Context, user *entity.User) error
	FindUserByEmail(ctx context.Context, email string) (*entity.User, error)
	FindUserByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	SetEmailVerified(ctx context.Context, userID uuid.UUID, at time.Time) error

	CreateVerification(ctx context.Context, verification *entity.EmailVerification) error
	FindActiveVerification(ctx context.Context, userID uuid.UUID) (*entity.EmailVerification, error)
	UpdateVerificationAttempts(ctx context.Context, id uuid.UUID, attempts int) error
	ConsumeVerification(ctx context.Context, id uuid.UUID, at time.Time) error

	CreateSession(ctx context.Context, session *entity.RefreshSession) error
	FindSessionByHash(ctx context.Context, hash string) (*entity.RefreshSession, error)
	RevokeSession(ctx context.Context, id uuid.UUID, at time.Time) error
	RevokeSessionReplaced(ctx context.Context, id uuid.UUID, at time.Time, replacedBy uuid.UUID) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID, at time.Time) error
}
