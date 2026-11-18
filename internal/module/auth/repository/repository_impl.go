package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/khorihutama/hsi-go-api/internal/module/auth/model/entity"
)

type gormRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) CreateUser(ctx context.Context, user *entity.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *gormRepository) FindUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) FindUserByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) SetEmailVerified(ctx context.Context, userID uuid.UUID, at time.Time) error {
	return r.db.WithContext(ctx).Model(&entity.User{}).
		Where("id = ?", userID).
		Update("email_verified_at", at).Error
}

func (r *gormRepository) CreateVerification(ctx context.Context, verification *entity.EmailVerification) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := tx.Model(&entity.EmailVerification{}).
			Where("user_id = ? AND consumed_at IS NULL", verification.UserID).
			Update("consumed_at", now).Error; err != nil {
			return err
		}
		return tx.Create(verification).Error
	})
}

func (r *gormRepository) FindActiveVerification(ctx context.Context, userID uuid.UUID) (*entity.EmailVerification, error) {
	var verification entity.EmailVerification
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND consumed_at IS NULL", userID).
		Order("created_at DESC").
		First(&verification).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &verification, nil
}

func (r *gormRepository) UpdateVerificationAttempts(ctx context.Context, id uuid.UUID, attempts int) error {
	return r.db.WithContext(ctx).Model(&entity.EmailVerification{}).
		Where("id = ?", id).
		Update("attempts", attempts).Error
}

func (r *gormRepository) ConsumeVerification(ctx context.Context, id uuid.UUID, at time.Time) error {
	return r.db.WithContext(ctx).Model(&entity.EmailVerification{}).
		Where("id = ?", id).
		Update("consumed_at", at).Error
}

func (r *gormRepository) CreateSession(ctx context.Context, session *entity.RefreshSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *gormRepository) FindSessionByHash(ctx context.Context, hash string) (*entity.RefreshSession, error) {
	var session entity.RefreshSession
	err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *gormRepository) RevokeSession(ctx context.Context, id uuid.UUID, at time.Time) error {
	return r.db.WithContext(ctx).Model(&entity.RefreshSession{}).
		Where("id = ?", id).
		Update("revoked_at", at).Error
}

func (r *gormRepository) RevokeSessionReplaced(ctx context.Context, id uuid.UUID, at time.Time, replacedBy uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&entity.RefreshSession{}).
		Where("id = ?", id).
		Updates(map[string]any{"revoked_at": at, "replaced_by": replacedBy}).Error
}

func (r *gormRepository) RevokeFamily(ctx context.Context, familyID uuid.UUID, at time.Time) error {
	return r.db.WithContext(ctx).Model(&entity.RefreshSession{}).
		Where("family_id = ? AND revoked_at IS NULL", familyID).
		Update("revoked_at", at).Error
}
