package repository

import (
	"context"

	"github.com/khorihutama/hsi-go-api/internal/module/trip/model/domain"
	"github.com/khorihutama/hsi-go-api/internal/module/trip/model/entity"
	"github.com/khorihutama/hsi-go-api/internal/module/trip/model/query"
)

type Repository interface {
	List(ctx context.Context, filter query.Filter) ([]entity.Trip, int64, error)
	GetByIDOrSlug(ctx context.Context, identifier string) (*entity.Trip, error)
	Destinations(ctx context.Context) ([]domain.DestinationCount, error)
	Categories(ctx context.Context) ([]domain.CategoryCount, error)
}
