package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/khorihutama/kelana-api/internal/common/errs"
	"github.com/khorihutama/kelana-api/internal/common/util"
	"github.com/khorihutama/kelana-api/internal/module/mytrip/model/dto"
	"github.com/khorihutama/kelana-api/internal/module/mytrip/repository"
)

var wib = time.FixedZone("WIB", 7*3600)

var validTabs = map[string]struct{}{
	"BELUM_BAYAR":    {},
	"SIAP_BERANGKAT": {},
	"SELESAI":        {},
	"RIWAYAT":        {},
}

type Canceller interface {
	CancelOnly(ctx context.Context, userID, reservationID string) error
}

type Config struct {
	WAGroupURL string
}

type Service struct {
	repo      repository.Repository
	canceller Canceller
	cfg       Config
	logger    *slog.Logger
	now       func() time.Time
}

func New(repo repository.Repository, canceller Canceller, cfg Config, logger *slog.Logger) *Service {
	return &Service{
		repo:      repo,
		canceller: canceller,
		cfg:       cfg,
		logger:    logger,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) List(ctx context.Context, userID, tab string, page, limit int) ([]dto.ListItem, int64, int, int, error) {
	ownerID, err := uuid.Parse(userID)
	if err != nil {
		return nil, 0, 0, 0, errs.Unauthorized("Sesi tidak valid")
	}
	tab = strings.ToUpper(strings.TrimSpace(tab))
	if tab != "" {
		if _, ok := validTabs[tab]; !ok {
			return nil, 0, 0, 0, errs.WithDetails(
				errs.New(errs.CodeInvalidStatus, 422, "Tab tidak valid"),
				[]errs.FieldError{{Field: "tab", Message: "tab harus BELUM_BAYAR, SIAP_BERANGKAT, SELESAI, atau RIWAYAT"}},
			)
		}
	}
	page, limit = util.NormalizePagination(page, limit)
	rows, total, err := s.repo.List(ctx, ownerID, tab, s.today(), page, limit)
	if err != nil {
		return nil, 0, 0, 0, errs.Internal(err)
	}
	items := make([]dto.ListItem, 0, len(rows))
	for i := range rows {
		items = append(items, s.toListItem(&rows[i]))
	}
	return items, total, page, limit, nil
}

func (s *Service) Get(ctx context.Context, userID, reservationID string) (*dto.Detail, error) {
	ownerID, err := uuid.Parse(userID)
	if err != nil {
		return nil, errs.Unauthorized("Sesi tidak valid")
	}
	id, err := uuid.Parse(reservationID)
	if err != nil {
		return nil, errs.NotFound("RESERVATION_NOT_FOUND", "Reservasi tidak ditemukan")
	}
	row, err := s.repo.Get(ctx, ownerID, id)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if row == nil {
		return nil, errs.NotFound("RESERVATION_NOT_FOUND", "Reservasi tidak ditemukan")
	}
	participants, err := s.repo.ListParticipants(ctx, id)
	if err != nil {
		return nil, errs.Internal(err)
	}
	documents, err := s.repo.ListDocuments(ctx, id)
	if err != nil {
		return nil, errs.Internal(err)
	}

	out := &dto.Detail{
		ID:                row.ID.String(),
		Code:              row.Code,
		Status:            row.Status,
		Tab:               tabFor(row.Status, row.EndDate, s.today()),
		Trip:              s.tripSummary(row),
		Schedule:          scheduleSummary(row),
		Participants:      make([]dto.Participant, 0, len(participants)),
		Documents:         make([]dto.Document, 0, len(documents)),
		Payment:           paymentSummary(row),
		ParticipantsCount: row.ParticipantsCount,
		TotalAmountIDR:    row.TotalAmountIDR,
		DiscountIDR:       row.DiscountIDR,
		Currency:          row.Currency,
		PickupPoint:       row.PickupPoint,
		Notes:             row.Notes,
		LockExpiresAt:     row.LockExpiresAt.UTC().Format(time.RFC3339),
		JoinWAURL:         s.joinWA(row),
		Timer:             s.timer(row),
		CreatedAt:         row.CreatedAt.UTC().Format(time.RFC3339),
	}
	if row.PaymentExpiresAt != nil {
		value := row.PaymentExpiresAt.UTC().Format(time.RFC3339)
		out.PaymentExpiresAt = &value
	}
	if row.PaidAt != nil {
		value := row.PaidAt.UTC().Format(time.RFC3339)
		out.PaidAt = &value
	}
	for _, participant := range participants {
		out.Participants = append(out.Participants, dto.Participant{
			ID: participant.ID.String(), FullName: participant.FullName,
			Gender: participant.Gender, Email: participant.Email, Phone: participant.Phone,
		})
	}
	for _, document := range documents {
		out.Documents = append(out.Documents, dto.Document{
			ID: document.ID.String(), ParticipantID: document.ParticipantID.String(),
			Type: document.Type, MimeType: document.MimeType, SizeBytes: document.SizeBytes,
			Status: document.Status, CreatedAt: document.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

func (s *Service) Cancel(ctx context.Context, userID, reservationID string) (*dto.Detail, error) {
	if _, err := s.Get(ctx, userID, reservationID); err != nil {
		return nil, err
	}
	if err := s.canceller.CancelOnly(ctx, userID, reservationID); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID, reservationID)
}

func (s *Service) toListItem(row *repository.ListRow) dto.ListItem {
	return dto.ListItem{
		ID:                row.ID.String(),
		Code:              row.Code,
		Status:            row.Status,
		Tab:               tabFor(row.Status, row.EndDate, s.today()),
		Trip:              s.tripSummary(row),
		Schedule:          scheduleSummary(row),
		ParticipantsCount: row.ParticipantsCount,
		TotalAmountIDR:    row.TotalAmountIDR,
		Currency:          row.Currency,
		Payment:           paymentSummary(row),
		JoinWAURL:         s.joinWA(row),
		Timer:             s.timer(row),
		CreatedAt:         row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func (s *Service) tripSummary(row *repository.ListRow) dto.TripSummary {
	return dto.TripSummary{
		ID: row.TripID.String(), Slug: row.TripSlug, Title: row.TripTitle,
		Category: row.TripCategory, DepartureCity: row.DepartureCity, DestinationCity: row.DestinationCity,
	}
}

func scheduleSummary(row *repository.ListRow) dto.ScheduleSummary {
	return dto.ScheduleSummary{
		ID:        row.ScheduleID.String(),
		StartDate: row.StartDate.Format("2006-01-02"),
		EndDate:   row.EndDate.Format("2006-01-02"),
	}
}

func paymentSummary(row *repository.ListRow) *dto.PaymentSummary {
	if row.PaymentID == nil || row.PaymentStatus == nil {
		return nil
	}
	out := &dto.PaymentSummary{
		ID:        row.PaymentID.String(),
		Status:    *row.PaymentStatus,
		Method:    value(row.PaymentMethod),
		Currency:  value(row.PaymentCurrency),
		VANumber:  row.PaymentVANumber,
		QRString:  row.PaymentQRString,
		ExpiresAt: "",
	}
	if row.PaymentAmountIDR != nil {
		out.AmountIDR = *row.PaymentAmountIDR
	}
	if row.PaymentExpiresAtDB != nil {
		out.ExpiresAt = row.PaymentExpiresAtDB.UTC().Format(time.RFC3339)
	}
	if row.PaymentPaidAt != nil {
		paidAt := row.PaymentPaidAt.UTC().Format(time.RFC3339)
		out.PaidAt = &paidAt
	}
	return out
}

func (s *Service) joinWA(row *repository.ListRow) *string {
	if row.Status != "PAID" {
		return nil
	}
	if row.TripWAGroupURL != nil && strings.TrimSpace(*row.TripWAGroupURL) != "" {
		value := strings.TrimSpace(*row.TripWAGroupURL)
		return &value
	}
	if value := strings.TrimSpace(s.cfg.WAGroupURL); value != "" {
		return &value
	}
	return nil
}

func (s *Service) timer(row *repository.ListRow) *dto.Timer {
	now := s.now()
	if row.PaymentStatus != nil && *row.PaymentStatus == "PENDING" && row.PaymentExpiresAtDB != nil {
		return timerAt("PAYMENT", *row.PaymentExpiresAtDB, now)
	}
	if row.Status == "PENDING_PAYMENT" {
		return timerAt("LOCK", row.LockExpiresAt, now)
	}
	return nil
}

func timerAt(kind string, expiresAt, now time.Time) *dto.Timer {
	remaining := int64(expiresAt.Sub(now).Seconds())
	if remaining < 0 {
		remaining = 0
	}
	return &dto.Timer{
		Kind:             kind,
		ExpiresAt:        expiresAt.UTC().Format(time.RFC3339),
		RemainingSeconds: remaining,
	}
}

func (s *Service) today() string {
	return s.now().In(wib).Format("2006-01-02")
}

func tabFor(status string, endDate time.Time, today string) string {
	switch status {
	case "PENDING_PAYMENT":
		return "BELUM_BAYAR"
	case "PAID":
		if endDate.Format("2006-01-02") >= today {
			return "SIAP_BERANGKAT"
		}
		return "SELESAI"
	case "COMPLETED":
		return "SELESAI"
	case "CANCELLED", "EXPIRED":
		return "RIWAYAT"
	default:
		return "RIWAYAT"
	}
}

func value(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
