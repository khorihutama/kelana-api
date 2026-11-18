package repository

import (
	"context"

	"github.com/khorihutama/hsi-go-api/internal/module/trip/model/entity"
)

type Repository interface {
	ListPublished(ctx context.Context, limit, offset int) ([]entity.Trip, int64, error)
}
