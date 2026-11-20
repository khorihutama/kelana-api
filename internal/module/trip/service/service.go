package service

import (
	"context"

	"github.com/khorihutama/kelana-api/internal/common/dto"
	"github.com/khorihutama/kelana-api/internal/common/errs"
	"github.com/khorihutama/kelana-api/internal/common/util"
	"github.com/khorihutama/kelana-api/internal/module/trip/model/domain"
	tripdto "github.com/khorihutama/kelana-api/internal/module/trip/model/dto"
	"github.com/khorihutama/kelana-api/internal/module/trip/model/entity"
	"github.com/khorihutama/kelana-api/internal/module/trip/model/query"
	"github.com/khorihutama/kelana-api/internal/module/trip/repository"
)

type Service struct {
	repo repository.Repository
}

func New(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, filter query.Filter) ([]tripdto.TripCard, dto.Meta, error) {
	trips, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, dto.Meta{}, errs.Internal(err)
	}

	cards := make([]tripdto.TripCard, 0, len(trips))
	for _, trip := range trips {
		cards = append(cards, toCard(trip))
	}
	return cards, dto.Meta{
		Page:       filter.Page,
		Limit:      filter.Limit,
		TotalItems: total,
		TotalPages: util.TotalPages(total, filter.Limit),
	}, nil
}

func (s *Service) Detail(ctx context.Context, identifier string) (*tripdto.TripDetail, error) {
	trip, err := s.repo.GetByIDOrSlug(ctx, identifier)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if trip == nil || trip.Status != "PUBLISHED" {
		return nil, errs.NotFound("TRIP_NOT_FOUND", "Trip tidak ditemukan")
	}
	return toDetail(*trip), nil
}

func (s *Service) Schedules(ctx context.Context, identifier string) ([]tripdto.Schedule, error) {
	trip, err := s.repo.GetByIDOrSlug(ctx, identifier)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if trip == nil || trip.Status != "PUBLISHED" {
		return nil, errs.NotFound("TRIP_NOT_FOUND", "Trip tidak ditemukan")
	}
	schedules := make([]tripdto.Schedule, 0, len(trip.Schedules))
	for _, schedule := range trip.Schedules {
		if schedule.Status == "OPEN" {
			schedules = append(schedules, toSchedule(schedule))
		}
	}
	return schedules, nil
}

func (s *Service) Destinations(ctx context.Context) ([]domain.DestinationCount, error) {
	rows, err := s.repo.Destinations(ctx)
	if err != nil {
		return nil, errs.Internal(err)
	}
	return rows, nil
}

func (s *Service) Categories(ctx context.Context) ([]domain.CategoryCount, error) {
	rows, err := s.repo.Categories(ctx)
	if err != nil {
		return nil, errs.Internal(err)
	}
	return rows, nil
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
	if next := nextOpenSchedule(trip.Schedules); next != nil {
		date := next.StartDate.Format("2006-01-02")
		seats := next.QuotaTotal - next.SeatsSold
		card.NextDeparture = &date
		card.SeatsAvailable = &seats
	}
	return card
}

func toDetail(trip entity.Trip) *tripdto.TripDetail {
	detail := &tripdto.TripDetail{
		ID:                trip.ID.String(),
		Slug:              trip.Slug,
		Title:             trip.Title,
		Description:       trip.Description,
		Category:          trip.Category,
		DepartureCity:     trip.DepartureCity,
		DestinationCity:   trip.DestinationCity,
		DurationDays:      trip.DurationDays,
		BasePriceIDR:      trip.BasePriceIDR,
		Currency:          trip.Currency,
		HeroImageURL:      trip.HeroImageURL,
		RatingAvg:         trip.RatingAvg,
		RatingCount:       trip.RatingCount,
		RequiredDocuments: append([]string(nil), trip.RequiredDocuments...),
		Itineraries:       make([]tripdto.Itinerary, 0, len(trip.Itineraries)),
		Facilities:        make([]tripdto.Facility, 0, len(trip.Facilities)),
		HotelOptions:      make([]tripdto.HotelOption, 0, len(trip.HotelOptions)),
		Schedules:         make([]tripdto.Schedule, 0, len(trip.Schedules)),
	}
	for _, item := range trip.Itineraries {
		detail.Itineraries = append(detail.Itineraries, tripdto.Itinerary{
			DayNumber: item.DayNumber, Title: item.Title, Description: item.Description,
		})
	}
	for _, item := range trip.Facilities {
		detail.Facilities = append(detail.Facilities, tripdto.Facility{Name: item.Name, Type: item.Type})
	}
	for _, item := range trip.HotelOptions {
		facilities := append([]string(nil), item.Hotel.Facilities...)
		detail.HotelOptions = append(detail.HotelOptions, tripdto.HotelOption{
			ID: item.ID.String(), Name: item.Hotel.Name, Address: item.Hotel.Address,
			Facilities: facilities, ExtraCostIDR: item.ExtraCostIDR, Currency: trip.Currency,
		})
	}
	for _, schedule := range trip.Schedules {
		if schedule.Status == "OPEN" {
			detail.Schedules = append(detail.Schedules, toSchedule(schedule))
		}
	}
	return detail
}

func toSchedule(schedule entity.TripSchedule) tripdto.Schedule {
	return tripdto.Schedule{
		ID:             schedule.ID.String(),
		StartDate:      schedule.StartDate.Format("2006-01-02"),
		EndDate:        schedule.EndDate.Format("2006-01-02"),
		DeparturePoint: schedule.DeparturePoint,
		PriceIDR:       schedule.PriceIDR,
		Currency:       schedule.Currency,
		SeatsAvailable: schedule.QuotaTotal - schedule.SeatsSold,
		Status:         schedule.Status,
	}
}

func nextOpenSchedule(schedules []entity.TripSchedule) *entity.TripSchedule {
	for i := range schedules {
		if schedules[i].Status == "OPEN" {
			return &schedules[i]
		}
	}
	return nil
}
