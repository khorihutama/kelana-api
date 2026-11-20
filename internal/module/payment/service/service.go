package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/khorihutama/kelana-api/internal/common/errs"
	"github.com/khorihutama/kelana-api/internal/infra/redis"
	"github.com/khorihutama/kelana-api/internal/module/payment/model/dto"
	"github.com/khorihutama/kelana-api/internal/module/payment/model/entity"
	"github.com/khorihutama/kelana-api/internal/module/payment/port"
	"github.com/khorihutama/kelana-api/internal/module/payment/repository"
)

type DocumentGate interface {
	CompleteForReservation(ctx context.Context, reservationID uuid.UUID) (bool, error)
}

type Config struct {
	Provider       string
	PaymentTTL     time.Duration
	ExpiryInterval time.Duration
	BrandName      string
}

type Service struct {
	repo      repository.Repository
	seats     *redis.SeatHoldStore
	docs      DocumentGate
	providers map[string]port.Provider
	cfg       Config
	logger    *slog.Logger
	now       func() time.Time
}

func New(repo repository.Repository, seats *redis.SeatHoldStore, docs DocumentGate, providers []port.Provider, cfg Config, logger *slog.Logger) *Service {
	if cfg.PaymentTTL <= 0 {
		cfg.PaymentTTL = 24 * time.Hour
	}
	if cfg.Provider == "" {
		cfg.Provider = "mock"
	}
	if cfg.ExpiryInterval <= 0 {
		cfg.ExpiryInterval = 30 * time.Second
	}
	if cfg.BrandName == "" {
		cfg.BrandName = "Kelana"
	}
	registry := make(map[string]port.Provider, len(providers))
	for _, provider := range providers {
		registry[strings.ToLower(provider.Name())] = provider
	}
	return &Service{
		repo:      repo,
		seats:     seats,
		docs:      docs,
		providers: registry,
		cfg:       cfg,
		logger:    logger,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Create(ctx context.Context, userID, reservationID, method string) (*dto.Response, bool, error) {
	ownerID, err := uuid.Parse(userID)
	if err != nil {
		return nil, false, errs.Unauthorized("Sesi tidak valid")
	}
	reservationUUID, err := uuid.Parse(reservationID)
	if err != nil {
		return nil, false, errs.NotFound("RESERVATION_NOT_FOUND", "Reservasi tidak ditemukan")
	}
	reservation, err := s.repo.GetReservationForPayment(ctx, reservationUUID)
	if err != nil {
		return nil, false, errs.Internal(err)
	}
	if reservation == nil {
		return nil, false, errs.NotFound("RESERVATION_NOT_FOUND", "Reservasi tidak ditemukan")
	}
	if reservation.UserID != ownerID {
		return nil, false, errs.Forbidden(errs.CodeForbidden, "Reservasi ini bukan milik Anda")
	}

	now := s.now()
	switch reservation.Status {
	case "PAID":
		return nil, false, errs.Conflict(errs.CodeAlreadyPaid, "Reservasi sudah dibayar")
	case "PENDING_PAYMENT":
		if now.After(reservation.LockExpiresAt) {
			return nil, false, errs.Conflict(errs.CodeLockExpired, "Masa lock reservasi sudah berakhir")
		}
	default:
		return nil, false, errs.Conflict(errs.CodeLockExpired, "Reservasi tidak aktif")
	}

	complete, err := s.docs.CompleteForReservation(ctx, reservationUUID)
	if err != nil {
		if appErr, ok := errs.As(err); ok {
			return nil, false, appErr
		}
		return nil, false, errs.Internal(err)
	}
	if !complete {
		return nil, false, errs.WithDetails(
			errs.New(errs.CodeDocumentsIncomplete, http.StatusUnprocessableEntity, "Dokumen wajib belum lengkap"),
			[]errs.FieldError{{Field: "documents", Message: "Lengkapi dokumen seluruh peserta sebelum membayar"}},
		)
	}

	method = strings.ToUpper(strings.TrimSpace(method))
	existing, err := s.repo.GetPaymentByReservation(ctx, reservationUUID)
	if err != nil {
		return nil, false, errs.Internal(err)
	}
	if existing != nil {
		if existing.Status == "PAID" {
			return nil, false, errs.Conflict(errs.CodeAlreadyPaid, "Reservasi sudah dibayar")
		}
		if existing.Status == "PENDING" && existing.Method == method && now.Before(existing.ExpiresAt) {
			return toResponse(existing), false, nil
		}
		return nil, false, errs.Conflict(errs.CodePaymentExists, "Pembayaran untuk reservasi ini sudah dibuat")
	}

	provider, ok := s.providers[strings.ToLower(s.cfg.Provider)]
	if !ok {
		return nil, false, errs.Internal(fmt.Errorf("payment provider %q is not registered", s.cfg.Provider))
	}
	paymentID := uuid.Must(uuid.NewV7())
	charge, err := provider.CreateCharge(ctx, port.ChargeRequest{
		PaymentID:       paymentID,
		ReservationID:   reservation.ID,
		ReservationCode: reservation.Code,
		AmountIDR:       reservation.TotalAmountIDR,
		Method:          port.Method(method),
		Description:     s.cfg.BrandName + " " + reservation.Code,
	})
	if err != nil {
		if errors.Is(err, port.ErrNotSupported) {
			return nil, false, errs.New(errs.CodePaymentMethodUnavailable, http.StatusConflict, "Metode pembayaran belum tersedia")
		}
		return nil, false, errs.Internal(err)
	}

	expiresAt := now.Add(s.cfg.PaymentTTL)
	payment := &entity.Payment{
		ID:            paymentID,
		ReservationID: reservation.ID,
		Provider:      strings.ToUpper(provider.Name()),
		ProviderRef:   charge.ProviderRef,
		Method:        method,
		AmountIDR:     reservation.TotalAmountIDR,
		Currency:      "IDR",
		Status:        string(port.StatusPending),
		ExpiresAt:     expiresAt,
	}
	if charge.Status != "" {
		payment.Status = string(charge.Status)
	}
	if charge.VANumber != "" {
		va := charge.VANumber
		payment.VANumber = &va
	}
	if charge.QRString != "" {
		qr := charge.QRString
		payment.QRString = &qr
	}
	if len(charge.Raw) > 0 {
		raw := string(charge.Raw)
		payment.RawResponse = &raw
	}

	if err := s.repo.CreatePayment(ctx, payment, expiresAt); err != nil {
		if errors.Is(err, repository.ErrPaymentExists) {
			return nil, false, errs.Conflict(errs.CodePaymentExists, "Pembayaran untuk reservasi ini sudah dibuat")
		}
		if errors.Is(err, repository.ErrReservationState) {
			return nil, false, errs.Conflict(errs.CodeLockExpired, "Reservasi tidak aktif")
		}
		return nil, false, errs.Internal(err)
	}
	return toResponse(payment), true, nil
}

func (s *Service) Get(ctx context.Context, userID, paymentID string) (*dto.Response, error) {
	ownerID, err := uuid.Parse(userID)
	if err != nil {
		return nil, errs.Unauthorized("Sesi tidak valid")
	}
	id, err := uuid.Parse(paymentID)
	if err != nil {
		return nil, errs.NotFound(errs.CodePaymentNotFound, "Pembayaran tidak ditemukan")
	}
	payment, err := s.repo.GetPaymentByID(ctx, id)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if payment == nil {
		return nil, errs.NotFound(errs.CodePaymentNotFound, "Pembayaran tidak ditemukan")
	}
	reservation, err := s.repo.GetReservationForPayment(ctx, payment.ReservationID)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if reservation == nil || reservation.UserID != ownerID {
		return nil, errs.NotFound(errs.CodePaymentNotFound, "Pembayaran tidak ditemukan")
	}
	return toResponse(payment), nil
}

func (s *Service) HandleWebhook(ctx context.Context, providerName string, headers http.Header, raw []byte) error {
	provider, ok := s.providers[strings.ToLower(providerName)]
	if !ok {
		return errs.NotFound(errs.CodeUnknownProvider, "Provider pembayaran tidak dikenal")
	}
	event, err := provider.ParseWebhook(ctx, headers, raw)
	if err != nil {
		switch {
		case errors.Is(err, port.ErrInvalidSignature):
			return errs.New(errs.CodeInvalidSignature, http.StatusBadRequest, "Tanda tangan webhook tidak valid")
		case errors.Is(err, port.ErrInvalidEvent):
			return errs.New(errs.CodeInvalidEvent, http.StatusBadRequest, "Payload webhook tidak valid")
		default:
			return errs.Internal(err)
		}
	}
	if event.PaymentID == nil {
		return errs.New(errs.CodeInvalidEvent, http.StatusBadRequest, "payment_id wajib ada pada webhook mock")
	}

	now := s.now()
	paymentEvent := &entity.PaymentEvent{
		ID:              uuid.Must(uuid.NewV7()),
		PaymentID:       *event.PaymentID,
		Provider:        strings.ToUpper(provider.Name()),
		ProviderEventID: event.ProviderEventID,
		EventType:       event.Type,
		SignatureValid:  true,
		Payload:         string(event.Raw),
		ProcessedAt:     now,
		ProviderRef:     event.ProviderRef,
		AmountIDR:       event.AmountIDR,
	}
	outcome, err := s.repo.ConfirmWebhook(ctx, provider.Name(), paymentEvent, now)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrPaymentNotFound):
			return errs.NotFound(errs.CodeUnknownPayment, "Pembayaran tidak ditemukan")
		case errors.Is(err, repository.ErrAmountMismatch):
			return errs.Conflict(errs.CodeAmountMismatch, "Nominal webhook tidak sesuai tagihan")
		case errors.Is(err, repository.ErrLockExpired):
			return errs.Conflict(errs.CodeLockExpired, "Masa lock reservasi sudah berakhir")
		case errors.Is(err, repository.ErrQuotaExceeded):
			return errs.Conflict(errs.CodeSeatUnavailable, "Kuota kursi tidak tersedia")
		case errors.Is(err, repository.ErrPaymentNotPending):
			return errs.Conflict(errs.CodeInvalidState, "Pembayaran tidak bisa diproses pada status ini")
		case errors.Is(err, repository.ErrProviderMismatch):
			return errs.New(errs.CodeInvalidEvent, http.StatusBadRequest, "Provider webhook tidak cocok")
		default:
			return errs.Internal(err)
		}
	}

	if outcome.Kind == repository.ConfirmPaid {
		if err := s.seats.ConfirmPaid(ctx, outcome.ScheduleID, outcome.ReservationID); err != nil {
			// DB is the source of truth; the sweeper drops the stale hold later.
			s.logger.Error("payment hold cleanup failed",
				"reservation_id", outcome.ReservationID,
				"schedule_id", outcome.ScheduleID,
				"error", err,
			)
		}
	}
	return nil
}

// ExpireDue marks due PENDING payments EXPIRED and expires still-locked
// reservations. Paid reservations are never selected and never lose seats.
func (s *Service) ExpireDue(ctx context.Context) (int, error) {
	expired, err := s.repo.ExpireDuePayments(ctx, s.now(), 100)
	if err != nil {
		return 0, err
	}
	for _, item := range expired {
		if !item.ReleaseSeat {
			continue
		}
		if _, err := s.seats.Release(ctx, item.ScheduleID, item.ReservationID); err != nil {
			s.logger.Error("expire payment release seat failed",
				"reservation_id", item.ReservationID,
				"schedule_id", item.ScheduleID,
				"error", err,
			)
		}
	}
	return len(expired), nil
}

func (s *Service) StartExpiryJob(ctx context.Context) {
	if count, err := s.ExpireDue(ctx); err != nil {
		s.logger.Error("payment expiry initial run failed", "error", err)
	} else if count > 0 {
		s.logger.Info("payment expiry job expired payments", "count", count)
	}
	ticker := time.NewTicker(s.cfg.ExpiryInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				jobCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				count, err := s.ExpireDue(jobCtx)
				cancel()
				if err != nil {
					s.logger.Error("payment expiry job failed", "error", err)
				} else if count > 0 {
					s.logger.Info("payment expiry job expired payments", "count", count)
				}
			}
		}
	}()
}

func (s *Service) Refund(ctx context.Context, providerName, providerRef string, amountIDR int64, reason string) error {
	provider, ok := s.providers[strings.ToLower(providerName)]
	if !ok {
		return errs.NotFound(errs.CodeUnknownProvider, "Provider pembayaran tidak dikenal")
	}
	return provider.Refund(ctx, port.RefundRequest{ProviderRef: providerRef, AmountIDR: amountIDR, Reason: reason})
}

func toResponse(payment *entity.Payment) *dto.Response {
	out := &dto.Response{
		ID:            payment.ID.String(),
		ReservationID: payment.ReservationID.String(),
		Provider:      payment.Provider,
		Method:        payment.Method,
		Status:        payment.Status,
		Currency:      payment.Currency,
		AmountIDR:     payment.AmountIDR,
		VANumber:      payment.VANumber,
		QRString:      payment.QRString,
		ExpiresAt:     payment.ExpiresAt.UTC().Format(time.RFC3339),
		CreatedAt:     payment.CreatedAt.UTC().Format(time.RFC3339),
	}
	if payment.PaidAt != nil {
		paidAt := payment.PaidAt.UTC().Format(time.RFC3339)
		out.PaidAt = &paidAt
	}
	return out
}
