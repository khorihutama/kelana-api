package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/khorihutama/kelana-api/internal/module/trip/model/domain"
	"github.com/khorihutama/kelana-api/internal/module/trip/model/entity"
	"github.com/khorihutama/kelana-api/internal/module/trip/model/query"
)

type gormRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) List(ctx context.Context, f query.Filter) ([]entity.Trip, int64, error) {
	q := r.baseQuery(ctx)

	if f.Query != "" {
		like := "%" + f.Query + "%"
		q = q.Where("(trips.title ILIKE ? OR trips.destination_city ILIKE ? OR trips.departure_city ILIKE ?)", like, like, like)
	}
	if f.Category != "" {
		q = q.Where("trips.category = ?", f.Category)
	}
	if f.Destination != "" {
		q = q.Where("lower(trips.destination_city) = lower(?)", f.Destination)
	}
	if f.Departure != "" {
		q = q.Where("lower(trips.departure_city) = lower(?)", f.Departure)
	}
	if f.PriceMinIDR != nil {
		q = q.Where("trips.base_price_idr >= ?", *f.PriceMinIDR)
	}
	if f.PriceMaxIDR != nil {
		q = q.Where("trips.base_price_idr <= ?", *f.PriceMaxIDR)
	}
	if f.DateFrom != nil || f.DateTo != nil {
		sub := r.db.WithContext(ctx).Table("trip_schedules AS ts").
			Select("1").
			Where("ts.trip_id = trips.id AND ts.status = 'OPEN'")
		if f.DateFrom != nil {
			sub = sub.Where("ts.start_date >= ?", *f.DateFrom)
		}
		if f.DateTo != nil {
			sub = sub.Where("ts.start_date <= ?", *f.DateTo)
		}
		q = q.Where("EXISTS (?)", sub)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	q = applySort(q, f.Sort)

	var trips []entity.Trip
	err := q.
		Preload("Schedules", func(db *gorm.DB) *gorm.DB {
			return db.Where("status = ?", "OPEN").Order("start_date ASC")
		}).
		Limit(f.Limit).
		Offset(f.Offset()).
		Find(&trips).Error
	if err != nil {
		return nil, 0, err
	}
	return trips, total, nil
}

func (r *gormRepository) GetByIDOrSlug(ctx context.Context, identifier string) (*entity.Trip, error) {
	var trip entity.Trip
	err := r.db.WithContext(ctx).
		Where("slug = ? OR id::text = ?", identifier, identifier).
		Preload("Itineraries", func(db *gorm.DB) *gorm.DB { return db.Order("day_number ASC") }).
		Preload("Facilities", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC") }).
		Preload("HotelOptions", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC") }).
		Preload("HotelOptions.Hotel").
		Preload("Schedules", func(db *gorm.DB) *gorm.DB { return db.Order("start_date ASC") }).
		First(&trip).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &trip, nil
}

func (r *gormRepository) Destinations(ctx context.Context) ([]domain.DestinationCount, error) {
	var rows []domain.DestinationCount
	err := r.db.WithContext(ctx).
		Model(&entity.Trip{}).
		Select("destination_city AS city, count(*) AS count").
		Where("status = ?", "PUBLISHED").
		Group("destination_city").
		Order("count DESC, city ASC").
		Scan(&rows).Error
	return rows, err
}

func (r *gormRepository) Categories(ctx context.Context) ([]domain.CategoryCount, error) {
	var rows []domain.CategoryCount
	err := r.db.WithContext(ctx).
		Model(&entity.Trip{}).
		Select("category, count(*) AS count").
		Where("status = ?", "PUBLISHED").
		Group("category").
		Order("category ASC").
		Scan(&rows).Error
	return rows, err
}

func (r *gormRepository) baseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Model(&entity.Trip{}).Where("trips.status = ?", "PUBLISHED")
}

func applySort(q *gorm.DB, sort string) *gorm.DB {
	switch sort {
	case "price_asc":
		return q.Order("trips.base_price_idr ASC, trips.created_at DESC")
	case "price_desc":
		return q.Order("trips.base_price_idr DESC, trips.created_at DESC")
	case "rating_desc":
		return q.Order("trips.rating_avg DESC, trips.rating_count DESC, trips.created_at DESC")
	case "departure_asc":
		return q.Order("(SELECT MIN(ts.start_date) FROM trip_schedules ts WHERE ts.trip_id = trips.id AND ts.status = 'OPEN') ASC NULLS LAST")
	default:
		return q.Order("trips.created_at DESC")
	}
}
