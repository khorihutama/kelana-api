package service

import (
	"context"

	"github.com/khorihutama/hsi-go-api/internal/common/dto"
	"github.com/khorihutama/hsi-go-api/internal/common/errs"
	"github.com/khorihutama/hsi-go-api/internal/common/util"
	tripdto "github.com/khorihutama/hsi-go-api/internal/module/trip/model/dto"
	"github.com/khorihutama/hsi-go-api/internal/module/trip/model/entity"
	"github.com/khorihutama/hsi-go-api/internal/module/trip/repository"
)

type Service struct {
	repo repository.Repository
}

func New(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, page, limit int) ([]tripdto.TripCard, dto.Meta, error) {
	page, limit = util.NormalizePagination(page, limit)
	trips, total, err := s.repo.ListPublished(ctx, limit, util.Offset(page, limit))
	if err != nil {
		return nil, dto.Meta{}, errs.Internal(err)
	}

	cards := make([]tripdto.TripCard, 0, len(trips))
	for _, trip := range trips {
		cards = append(cards, toCard(trip))
	}

	return cards, dto.Meta{
		Page:       page,
		Limit:      limit,
		TotalItems: total,
		TotalPages: util.TotalPages(total, limit),
	}, nil
}

func toCard(trip entity.Trip) tripdto.TripCard {
	card := tripdto.TripCard{
		ID:              trip.ID.String(),
		Slug:            trip.Slug,
		Title:           trip.Title,
		Category:        trip.Category,
		DepartureCity:   trip.DepartureCity,
		DestinationCity: trip.DestinationCity,
		DurationDays:    trip.DurationDays,
		BasePriceIDR:    trip.BasePriceIDR,
		Currency:        trip.Currency,
		HeroImageURL:    trip.HeroImageURL,
		RatingAvg:       trip.RatingAvg,
		RatingCount:     trip.RatingCount,
	}

	if len(trip.Schedules) > 0 {
		next := trip.Schedules[0]
		date := next.StartDate.Format("2006-01-02")
		seats := next.QuotaTotal - next.SeatsSold
		card.NextDeparture = &date
		card.SeatsAvailable = &seats
	}
	return card
}
