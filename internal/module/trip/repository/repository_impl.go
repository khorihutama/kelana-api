package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/khorihutama/hsi-go-api/internal/module/trip/model/entity"
)

type gormRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) ListPublished(ctx context.Context, limit, offset int) ([]entity.Trip, int64, error) {
	query := r.db.WithContext(ctx).Model(&entity.Trip{}).Where("status = ?", "PUBLISHED")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var trips []entity.Trip
	err := query.
		Preload("Schedules", func(db *gorm.DB) *gorm.DB {
			return db.Where("status = ?", "OPEN").Order("start_date ASC")
		}).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&trips).Error
	if err != nil {
		return nil, 0, err
	}
	return trips, total, nil
}
