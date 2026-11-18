package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/khorihutama/hsi-go-api/internal/common/errs"
	"github.com/khorihutama/hsi-go-api/internal/module/document/model/dto"
	"github.com/khorihutama/hsi-go-api/internal/module/document/model/entity"
	"github.com/khorihutama/hsi-go-api/internal/module/document/repository"
)

const maxDocumentBytes int64 = 3 * 1024 * 1024

var allowedTypes = map[string]struct{}{
	"KTP":        {},
	"BUKU_NIKAH": {},
	"KK":         {},
	"PASSPORT":   {},
	"OTHER":      {},
}

var allowedMIME = map[string]struct{}{
	"application/pdf": {},
	"image/jpeg":      {},
	"image/png":       {},
}

type Store interface {
	Put(ctx context.Context, key string, plaintext []byte) (string, error)
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

type Config struct {
	URLTTL time.Duration
}

type Service struct {
	repo   repository.Repository
	store  Store
	signer *Signer
	cfg    Config
	logger *slog.Logger
	now    func() time.Time
}

func New(repo repository.Repository, store Store, encKey []byte, cfg Config, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		store:  store,
		signer: NewSigner(encKey, cfg.URLTTL),
		cfg:    cfg,
		logger: logger,
		now:    func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Upload(ctx context.Context, userID, participantID, docType string, data []byte) (*dto.UploadResponse, error) {
	ownerID, err := uuid.Parse(userID)
	if err != nil {
		return nil, errs.Unauthorized("Sesi tidak valid")
	}
	participantUUID, err := uuid.Parse(strings.TrimSpace(participantID))
	if err != nil {
		return nil, errs.Validation([]errs.FieldError{{Field: "participant_id", Message: "participant_id tidak valid"}})
	}
	docType = strings.ToUpper(strings.TrimSpace(docType))
	if _, ok := allowedTypes[docType]; !ok {
		return nil, errs.New(errs.CodeInvalidDocType, http.StatusUnprocessableEntity, "Tipe dokumen tidak dikenal")
	}
	mimeType, appErr := validatePayload(data)
	if appErr != nil {
		return nil, appErr
	}

	participant, err := s.repo.GetParticipantContext(ctx, participantUUID)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if participant == nil || participant.UserID != ownerID {
		return nil, errs.NotFound(errs.CodeParticipantMissing, "Peserta tidak ditemukan")
	}
	switch {
	case participant.ReservationState == "PAID":
		return nil, errs.Conflict(errs.CodeAlreadyPaid, "Reservasi sudah dibayar")
	case participant.ReservationState != "PENDING_PAYMENT", s.now().After(participant.LockExpiresAt):
		return nil, errs.Conflict(errs.CodeLockExpired, "Masa lock reservasi sudah berakhir")
	}

	documentID := uuid.Must(uuid.NewV7())
	objectKey := fmt.Sprintf("reservations/%s/%s.enc", participant.ReservationID, documentID)
	checksum, err := s.store.Put(ctx, objectKey, data)
	if err != nil {
		return nil, errs.Internal(err)
	}

	doc := &entity.Document{
		ID:             documentID,
		ReservationID:  participant.ReservationID,
		ParticipantID:  participant.ParticipantID,
		UserID:         ownerID,
		Type:           docType,
		ObjectKey:      objectKey,
		MimeType:       mimeType,
		SizeBytes:      int64(len(data)),
		ChecksumSHA256: checksum,
		Status:         "UPLOADED",
	}
	if err := s.repo.CreateDocument(ctx, doc); err != nil {
		if cleanupErr := s.store.Delete(ctx, objectKey); cleanupErr != nil {
			s.logger.Error("document cleanup failed", "document_id", documentID, "error", cleanupErr)
		}
		return nil, errs.Internal(err)
	}

	return &dto.UploadResponse{
		ID:            doc.ID.String(),
		ReservationID: doc.ReservationID.String(),
		ParticipantID: doc.ParticipantID.String(),
		Type:          doc.Type,
		SizeBytes:     doc.SizeBytes,
		Status:        doc.Status,
		CreatedAt:     doc.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func (s *Service) SignedURL(ctx context.Context, userID, documentID string) (*dto.URLResponse, error) {
	doc, appErr := s.getOwned(ctx, userID, documentID)
	if appErr != nil {
		return nil, appErr
	}
	signature, exp := s.signer.Sign(doc.ID, s.now())
	out := &dto.URLResponse{
		ID:        doc.ID.String(),
		Type:      doc.Type,
		SizeBytes: doc.SizeBytes,
		Status:    doc.Status,
		DownloadURL: fmt.Sprintf(
			"/api/v1/documents/%s/content?exp=%d&sig=%s", doc.ID, exp, signature,
		),
		ExpiresAt: time.Unix(exp, 0).UTC().Format(time.RFC3339),
	}
	return out, nil
}

func (s *Service) Download(ctx context.Context, documentID string, exp int64, signature string) (*entity.Document, []byte, error) {
	id, err := uuid.Parse(documentID)
	if err != nil {
		return nil, nil, errs.NotFound(errs.CodeDocumentNotFound, "Dokumen tidak ditemukan")
	}
	if appErr := s.signer.Verify(id, exp, signature, s.now()); appErr != nil {
		return nil, nil, appErr
	}
	doc, err := s.repo.GetDocument(ctx, id)
	if err != nil {
		return nil, nil, errs.Internal(err)
	}
	if doc == nil {
		return nil, nil, errs.NotFound(errs.CodeDocumentNotFound, "Dokumen tidak ditemukan")
	}
	data, err := s.store.Get(ctx, doc.ObjectKey)
	if err != nil {
		return nil, nil, errs.Internal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != doc.ChecksumSHA256 {
		return nil, nil, errs.Internal(fmt.Errorf("document %s checksum mismatch", doc.ID))
	}
	return doc, data, nil
}

func (s *Service) Delete(ctx context.Context, userID, documentID string) error {
	doc, appErr := s.getOwned(ctx, userID, documentID)
	if appErr != nil {
		return appErr
	}
	reservation, err := s.repo.GetReservationContext(ctx, doc.ReservationID)
	if err != nil {
		return errs.Internal(err)
	}
	if reservation == nil {
		return errs.NotFound(errs.CodeDocumentNotFound, "Dokumen tidak ditemukan")
	}
	if reservation.Status == "PAID" {
		return errs.Conflict(errs.CodeAlreadyPaid, "Dokumen tidak bisa dihapus setelah pembayaran")
	}
	deleted, err := s.repo.DeleteDocument(ctx, doc.ID)
	if err != nil {
		return errs.Internal(err)
	}
	if !deleted {
		return errs.NotFound(errs.CodeDocumentNotFound, "Dokumen tidak ditemukan")
	}
	if err := s.store.Delete(ctx, doc.ObjectKey); err != nil {
		s.logger.Error("document object delete failed", "document_id", doc.ID, "error", err)
	}
	return nil
}

func (s *Service) Completeness(ctx context.Context, userID, reservationID string) (*dto.CompletenessResponse, error) {
	ownerID, err := uuid.Parse(userID)
	if err != nil {
		return nil, errs.Unauthorized("Sesi tidak valid")
	}
	reservationUUID, err := uuid.Parse(reservationID)
	if err != nil {
		return nil, errs.NotFound("RESERVATION_NOT_FOUND", "Reservasi tidak ditemukan")
	}
	reservation, err := s.repo.GetReservationContext(ctx, reservationUUID)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if reservation == nil || reservation.UserID != ownerID {
		return nil, errs.NotFound("RESERVATION_NOT_FOUND", "Reservasi tidak ditemukan")
	}
	return s.buildCompleteness(ctx, reservation)
}

// CompleteForReservation is the payment gate. It intentionally skips the owner
// check: the caller has already authorized the reservation.
func (s *Service) CompleteForReservation(ctx context.Context, reservationID uuid.UUID) (bool, error) {
	reservation, err := s.repo.GetReservationContext(ctx, reservationID)
	if err != nil {
		return false, errs.Internal(err)
	}
	if reservation == nil {
		return false, errs.NotFound("RESERVATION_NOT_FOUND", "Reservasi tidak ditemukan")
	}
	out, err := s.buildCompleteness(ctx, reservation)
	if err != nil {
		return false, err
	}
	return out.Complete, nil
}

func (s *Service) buildCompleteness(ctx context.Context, reservation *repository.ReservationContext) (*dto.CompletenessResponse, error) {
	required := make([]string, 0, len(reservation.RequiredDocuments))
	for _, docType := range reservation.RequiredDocuments {
		if _, ok := allowedTypes[docType]; ok {
			required = append(required, docType)
		}
	}
	if len(required) == 0 {
		required = []string{"KTP"}
	}

	participants, err := s.repo.ListParticipants(ctx, reservation.ReservationID)
	if err != nil {
		return nil, errs.Internal(err)
	}
	documents, err := s.repo.ListDocuments(ctx, reservation.ReservationID)
	if err != nil {
		return nil, errs.Internal(err)
	}
	uploaded := make(map[uuid.UUID]map[string]struct{}, len(participants))
	for _, doc := range documents {
		if uploaded[doc.ParticipantID] == nil {
			uploaded[doc.ParticipantID] = map[string]struct{}{}
		}
		uploaded[doc.ParticipantID][doc.Type] = struct{}{}
	}

	out := &dto.CompletenessResponse{
		ReservationID:     reservation.ReservationID.String(),
		RequiredDocuments: required,
		Complete:          true,
		Participants:      make([]dto.ParticipantCompleteness, 0, len(participants)),
		Missing:           make([]dto.MissingDocument, 0),
	}
	for _, participant := range participants {
		item := dto.ParticipantCompleteness{
			ParticipantID: participant.ID.String(),
			FullName:      participant.FullName,
			Uploaded:      make([]string, 0, len(uploaded[participant.ID])),
			Missing:       make([]string, 0),
		}
		for docType := range uploaded[participant.ID] {
			item.Uploaded = append(item.Uploaded, docType)
		}
		sort.Strings(item.Uploaded)
		for _, docType := range required {
			if _, ok := uploaded[participant.ID][docType]; ok {
				continue
			}
			item.Missing = append(item.Missing, docType)
			out.Missing = append(out.Missing, dto.MissingDocument{
				ParticipantID: participant.ID.String(),
				FullName:      participant.FullName,
				Type:          docType,
			})
		}
		if len(item.Missing) > 0 {
			out.Complete = false
		}
		out.Participants = append(out.Participants, item)
	}
	return out, nil
}

func (s *Service) getOwned(ctx context.Context, userID, documentID string) (*entity.Document, *errs.AppError) {
	ownerID, err := uuid.Parse(userID)
	if err != nil {
		return nil, errs.Unauthorized("Sesi tidak valid")
	}
	id, err := uuid.Parse(documentID)
	if err != nil {
		return nil, errs.NotFound(errs.CodeDocumentNotFound, "Dokumen tidak ditemukan")
	}
	doc, err := s.repo.GetDocument(ctx, id)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if doc == nil || doc.UserID != ownerID {
		return nil, errs.NotFound(errs.CodeDocumentNotFound, "Dokumen tidak ditemukan")
	}
	return doc, nil
}

func validatePayload(data []byte) (string, *errs.AppError) {
	if int64(len(data)) > maxDocumentBytes {
		return "", errs.New(errs.CodeFileTooLarge, http.StatusRequestEntityTooLarge, "Ukuran dokumen maksimal 3 MB")
	}
	if len(data) == 0 {
		return "", errs.New(errs.CodeUnsupportedMedia, http.StatusUnsupportedMediaType, "File dokumen kosong")
	}
	mimeType := http.DetectContentType(data)
	if _, ok := allowedMIME[mimeType]; !ok {
		return "", errs.New(errs.CodeUnsupportedMedia, http.StatusUnsupportedMediaType, "Format dokumen harus JPEG, PNG, atau PDF")
	}
	return mimeType, nil
}

func extensionFor(mimeType string) string {
	switch mimeType {
	case "application/pdf":
		return ".pdf"
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	default:
		return ".bin"
	}
}

func ContentDisposition(doc *entity.Document) string {
	return fmt.Sprintf("inline; filename=%q", doc.Type+"-"+doc.ID.String()+extensionFor(doc.MimeType))
}
