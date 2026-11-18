package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/khorihutama/hsi-go-api/internal/common/errs"
	"github.com/khorihutama/hsi-go-api/internal/infra/redis"
	"github.com/khorihutama/hsi-go-api/internal/module/booking/model/dto"
	"github.com/khorihutama/hsi-go-api/internal/module/booking/model/entity"
	"github.com/khorihutama/hsi-go-api/internal/module/booking/repository"
)

type Config struct {
	SeatLockTTL       time.Duration
	SeatHoldSafetyTTL time.Duration
	SeatSweepInterval time.Duration
}

type Service struct {
	repo   repository.Repository
	seats  *redis.SeatHoldStore
	cfg    Config
	logger *slog.Logger
	now    func() time.Time
}

func New(repo repository.Repository, seats *redis.SeatHoldStore, cfg Config, logger *slog.Logger) *Service {
	if cfg.SeatHoldSafetyTTL <= cfg.SeatLockTTL {
		cfg.SeatHoldSafetyTTL = cfg.SeatLockTTL * 2
	}
	return &Service{
		repo:   repo,
		seats:  seats,
		cfg:    cfg,
		logger: logger,
		now:    func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) ValidatePromo(ctx context.Context, code string, subtotal int64) (*dto.PromoValidateResponse, error) {
	promo, discount, err := s.resolvePromo(ctx, code, subtotal)
	if err != nil {
		return nil, err
	}
	return &dto.PromoValidateResponse{
		Valid:         true,
		Code:          promo.Code,
		DiscountIDR:   discount,
		FinalTotalIDR: subtotal - discount,
	}, nil
}

func (s *Service) Lock(ctx context.Context, userID string, req dto.LockRequest) (*dto.LockResponse, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, errs.Unauthorized("Sesi tidak valid")
	}
	scheduleID, err := uuid.Parse(req.ScheduleID)
	if err != nil {
		return nil, errs.Validation([]errs.FieldError{{Field: "schedule_id", Message: "schedule_id tidak valid"}})
	}

	schedule, err := s.repo.GetSchedule(ctx, scheduleID)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if schedule == nil {
		return nil, errs.NotFound(errs.CodeScheduleNotFound, "Jadwal tidak ditemukan")
	}
	if schedule.Status != "OPEN" {
		return nil, errs.Conflict(errs.CodeScheduleClosed, "Jadwal sudah ditutup")
	}

	count := len(req.Participants)
	subtotal := schedule.PriceIDR * int64(count)

	hotelID, hotelExtra, appErr := s.resolveHotel(ctx, schedule.TripID, req.HotelID)
	if appErr != nil {
		return nil, appErr
	}
	subtotal += hotelExtra

	var promo *entity.Promo
	var discount int64
	if req.PromoCode != nil && strings.TrimSpace(*req.PromoCode) != "" {
		promo, discount, err = s.resolvePromo(ctx, *req.PromoCode, subtotal)
		if err != nil {
			return nil, err
		}
	}
	total := subtotal - discount
	if total < 0 {
		total = 0
	}

	reservationID := uuid.Must(uuid.NewV7())
	reservationCode := "HSI-" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:10])
	lockExpiresAt := s.now().Add(s.cfg.SeatLockTTL)

	remaining, err := s.seats.Lock(ctx, scheduleID, reservationID, count, s.cfg.SeatLockTTL, s.cfg.SeatHoldSafetyTTL)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if remaining == -2 {
		if err := s.seats.InitIfMissing(ctx, scheduleID, int64(schedule.QuotaTotal-schedule.SeatsSold)); err != nil {
			return nil, errs.Internal(err)
		}
		remaining, err = s.seats.Lock(ctx, scheduleID, reservationID, count, s.cfg.SeatLockTTL, s.cfg.SeatHoldSafetyTTL)
		if err != nil {
			return nil, errs.Internal(err)
		}
	}
	if remaining < 0 {
		return nil, errs.Conflict(errs.CodeSeatUnavailable, "Kuota kursi tidak tersedia")
	}

	reservation := &entity.Reservation{
		ID:                reservationID,
		Code:              reservationCode,
		UserID:            userUUID,
		TripID:            schedule.TripID,
		ScheduleID:        scheduleID,
		Status:            "PENDING_PAYMENT",
		ParticipantsCount: count,
		UnitPriceIDR:      schedule.PriceIDR,
		TotalAmountIDR:    total,
		DiscountIDR:       discount,
		Currency:          "IDR",
		PickupPoint:       req.PickupPoint,
		Notes:             req.Notes,
		HotelID:           hotelID,
		LockExpiresAt:     lockExpiresAt,
	}
	if promo != nil {
		reservation.PromoCodeID = &promo.ID
	}

	participants := make([]entity.Participant, 0, count)
	for i, p := range req.Participants {
		participants = append(participants, entity.Participant{
			ID:        uuid.Must(uuid.NewV7()),
			FullName:  p.FullName,
			Gender:    p.Gender,
			Email:     p.Email,
			Phone:     p.Phone,
			IsPrimary: i == 0,
		})
	}

	if err := s.repo.CreateReservation(ctx, reservation, participants); err != nil {
		_, _ = s.seats.Release(ctx, scheduleID, reservationID)
		if errors.Is(err, repository.ErrActiveReservation) {
			return nil, errs.Conflict(errs.CodeActiveReservation, "Anda sudah punya reservasi aktif untuk jadwal ini")
		}
		return nil, errs.Internal(err)
	}

	return &dto.LockResponse{
		ID:             reservation.ID.String(),
		Code:           reservation.Code,
		Status:         reservation.Status,
		Currency:       reservation.Currency,
		LockExpiresAt:  reservation.LockExpiresAt.Format(time.RFC3339),
		TotalAmountIDR: reservation.TotalAmountIDR,
		DiscountIDR:    reservation.DiscountIDR,
		SeatsRemaining: remaining,
	}, nil
}

func (s *Service) Get(ctx context.Context, userID, id string) (*dto.ReservationResponse, error) {
	reservation, appErr := s.getOwned(ctx, userID, id)
	if appErr != nil {
		return nil, appErr
	}
	return toResponse(reservation), nil
}

func (s *Service) Cancel(ctx context.Context, userID, id string) (*dto.ReservationResponse, error) {
	reservation, appErr := s.getOwned(ctx, userID, id)
	if appErr != nil {
		return nil, appErr
	}
	if reservation.Status != "PENDING_PAYMENT" {
		return nil, errs.Conflict(errs.CodeInvalidState, "Reservasi tidak bisa dibatalkan pada status ini")
	}
	changed, err := s.repo.CancelReservation(ctx, reservation.ID, reservation.UserID)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if !changed {
		return nil, errs.Conflict(errs.CodeInvalidState, "Reservasi tidak bisa dibatalkan pada status ini")
	}
	_, _ = s.seats.Release(ctx, reservation.ScheduleID, reservation.ID)
	reservation.Status = "CANCELLED"
	return toResponse(reservation), nil
}

func (s *Service) SweepOnce(ctx context.Context) (int, error) {
	schedules, err := s.repo.ListOpenSchedules(ctx)
	if err != nil {
		return 0, err
	}
	expiredCount := 0
	for _, schedule := range schedules {
		ids, err := s.seats.ExpiredHolds(ctx, schedule.ID, 100)
		if err != nil {
			return expiredCount, err
		}
		for _, id := range ids {
			changed, err := s.repo.MarkExpired(ctx, id)
			if err != nil {
				s.logger.Error("mark expired failed", "reservation_id", id, "error", err)
				continue
			}
			if changed {
				expiredCount++
				if _, err := s.seats.Release(ctx, schedule.ID, id); err != nil {
					s.logger.Error("release seat failed", "reservation_id", id, "error", err)
				}
			} else {
				// Reservation is not pending anymore; drop the stale hold without returning seats.
				_ = s.seats.ConfirmPaid(ctx, schedule.ID, id)
			}
		}
	}
	return expiredCount, nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	schedules, err := s.repo.ListOpenSchedules(ctx)
	if err != nil {
		return err
	}
	for _, schedule := range schedules {
		if err := s.seats.Reconcile(ctx, schedule.ID, schedule.QuotaTotal, schedule.SeatsSold); err != nil {
			return fmt.Errorf("reconcile schedule %s: %w", schedule.ID, err)
		}
	}
	return nil
}

func (s *Service) StartSweeper(ctx context.Context) {
	if err := s.Reconcile(ctx); err != nil {
		s.logger.Error("seat reconcile failed", "error", err)
	}
	ticker := time.NewTicker(s.cfg.SeatSweepInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweepCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				count, err := s.SweepOnce(sweepCtx)
				cancel()
				if err != nil {
					s.logger.Error("seat sweeper failed", "error", err)
				} else if count > 0 {
					s.logger.Info("seat sweeper expired reservations", "count", count)
				}
			}
		}
	}()
}

func (s *Service) resolvePromo(ctx context.Context, rawCode string, subtotal int64) (*entity.Promo, int64, error) {
	code := strings.TrimSpace(rawCode)
	if code == "" {
		return nil, 0, errs.New(errs.CodePromoNotFound, 422, "Kode promo tidak ditemukan")
	}
	promo, err := s.repo.GetPromo(ctx, code)
	if err != nil {
		return nil, 0, errs.Internal(err)
	}
	if promo == nil {
		return nil, 0, errs.New(errs.CodePromoNotFound, 422, "Kode promo tidak ditemukan")
	}
	now := s.now()
	if !promo.Active {
		return nil, 0, errs.New(errs.CodePromoInactive, 422, "Kode promo tidak aktif")
	}
	if now.Before(promo.ValidFrom) || now.After(promo.ValidTo) {
		return nil, 0, errs.New(errs.CodePromoExpired, 422, "Kode promo sudah tidak berlaku")
	}
	if promo.Quota != nil && promo.UsedCount >= *promo.Quota {
		return nil, 0, errs.Conflict(errs.CodePromoQuota, "Kuota promo sudah habis")
	}
	if subtotal < promo.MinTransactionIDR {
		return nil, 0, errs.New(errs.CodePromoMin, 422, "Minimum transaksi untuk promo ini belum terpenuhi")
	}

	var discount int64
	if promo.DiscountType == "PERCENT" {
		discount = (subtotal*int64(promo.DiscountValue) + 5000) / 10000
	} else {
		discount = int64(promo.DiscountValue)
	}
	if discount > subtotal {
		discount = subtotal
	}
	return promo, discount, nil
}

func (s *Service) resolveHotel(ctx context.Context, tripID uuid.UUID, rawHotelID *string) (*uuid.UUID, int64, *errs.AppError) {
	hotels, err := s.repo.ListHotels(ctx, tripID)
	if err != nil {
		return nil, 0, errs.Internal(err)
	}
	if rawHotelID != nil && strings.TrimSpace(*rawHotelID) != "" {
		hotelID, err := uuid.Parse(*rawHotelID)
		if err != nil {
			return nil, 0, errs.Validation([]errs.FieldError{{Field: "hotel_id", Message: "hotel_id tidak valid"}})
		}
		for _, hotel := range hotels {
			if hotel.ID == hotelID {
				return &hotel.ID, hotel.ExtraCostIDR, nil
			}
		}
		return nil, 0, errs.Validation([]errs.FieldError{{Field: "hotel_id", Message: "hotel_id bukan pilihan trip ini"}})
	}
	if len(hotels) == 0 {
		return nil, 0, nil
	}
	if len(hotels) > 1 {
		return nil, 0, errs.Validation([]errs.FieldError{{Field: "hotel_id", Message: "hotel_id wajib dipilih karena trip punya lebih dari satu hotel"}})
	}
	return &hotels[0].ID, hotels[0].ExtraCostIDR, nil
}

func (s *Service) getOwned(ctx context.Context, userID, id string) (*entity.Reservation, *errs.AppError) {
	reservationID, err := uuid.Parse(id)
	if err != nil {
		return nil, errs.NotFound("RESERVATION_NOT_FOUND", "Reservasi tidak ditemukan")
	}
	ownerID, err := uuid.Parse(userID)
	if err != nil {
		return nil, errs.Unauthorized("Sesi tidak valid")
	}
	reservation, err := s.repo.GetReservation(ctx, reservationID)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if reservation == nil {
		return nil, errs.NotFound("RESERVATION_NOT_FOUND", "Reservasi tidak ditemukan")
	}
	if reservation.UserID != ownerID {
		return nil, errs.Forbidden(errs.CodeForbidden, "Reservasi ini bukan milik Anda")
	}
	return reservation, nil
}

func toResponse(reservation *entity.Reservation) *dto.ReservationResponse {
	out := &dto.ReservationResponse{
		ID:             reservation.ID.String(),
		Code:           reservation.Code,
		Status:         reservation.Status,
		TripID:         reservation.TripID.String(),
		ScheduleID:     reservation.ScheduleID.String(),
		UnitPriceIDR:   reservation.UnitPriceIDR,
		TotalAmountIDR: reservation.TotalAmountIDR,
		DiscountIDR:    reservation.DiscountIDR,
		Currency:       reservation.Currency,
		PickupPoint:    reservation.PickupPoint,
		Notes:          reservation.Notes,
		LockExpiresAt:  reservation.LockExpiresAt.Format(time.RFC3339),
		Participants:   make([]dto.ParticipantResponse, 0, len(reservation.Participants)),
	}
	if reservation.HotelID != nil {
		hotel := reservation.HotelID.String()
		out.HotelID = &hotel
	}
	if reservation.PromoCodeID != nil {
		promo := reservation.PromoCodeID.String()
		out.PromoCodeID = &promo
	}
	for _, participant := range reservation.Participants {
		out.Participants = append(out.Participants, dto.ParticipantResponse{
			ID: participant.ID.String(), FullName: participant.FullName,
			Gender: participant.Gender, Phone: participant.Phone, Email: participant.Email,
		})
	}
	return out
}
