//go:build integration

package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"github.com/khorihutama/kelana-api/internal/common/errs"
	"github.com/khorihutama/kelana-api/internal/config"
	"github.com/khorihutama/kelana-api/internal/infra/database"
	"github.com/khorihutama/kelana-api/internal/infra/storage"
	documentrepo "github.com/khorihutama/kelana-api/internal/module/document/repository"
)

type documentFixture struct {
	db            *gorm.DB
	service       *Service
	storeRoot     string
	userID        uuid.UUID
	otherUserID   uuid.UUID
	reservationID uuid.UUID
	participants  []uuid.UUID
}

func newDocumentFixture(t *testing.T) *documentFixture {
	t.Helper()
	root := filepath.Join("..", "..", "..", "..")
	_ = godotenv.Load(filepath.Join(root, ".env"))
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	db, err := database.Connect(cfg.DSN(), false)
	if err != nil {
		t.Fatalf("db: %v", err)
	}

	storeRoot := t.TempDir()
	store, err := storage.NewLocal(storeRoot, cfg.DocumentEncKey)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := New(documentrepo.New(db), store, cfg.DocumentEncKey, Config{URLTTL: 5 * time.Minute}, logger)

	suffix := uuid.NewString()[:8]
	userID := uuid.Must(uuid.NewV7())
	otherUserID := uuid.Must(uuid.NewV7())
	tripID := uuid.Must(uuid.NewV7())
	scheduleID := uuid.Must(uuid.NewV7())
	reservationID := uuid.Must(uuid.NewV7())
	participantA := uuid.Must(uuid.NewV7())
	participantB := uuid.Must(uuid.NewV7())

	for _, user := range []uuid.UUID{userID, otherUserID} {
		if err := db.Exec(`
			INSERT INTO users (id, email, password_hash, name, role, status, email_verified_at)
			VALUES (?, ?, 'x', 'Document Test', 'CUSTOMER', 'ACTIVE', now())
		`, user, "doc-test-"+user.String()+"@example.com").Error; err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	if err := db.Exec(`
		INSERT INTO trips (id, slug, title, description, category, departure_city, destination_city,
			duration_days, base_price_idr, currency, status, required_documents, rating_avg, rating_count)
		VALUES (?, ?, 'Document Trip', 'Test', 'OPEN', 'Jakarta', 'Bali', 2, 1000000, 'IDR', 'PUBLISHED', '{KTP}', 0, 0)
	`, tripID, "doc-trip-"+suffix).Error; err != nil {
		t.Fatalf("insert trip: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO trip_schedules (id, trip_id, start_date, end_date, departure_point, quota_total, seats_sold, price_idr, currency, status)
		VALUES (?, ?, '2027-04-01', '2027-04-02', 'Jakarta', 10, 0, 1000000, 'IDR', 'OPEN')
	`, scheduleID, tripID).Error; err != nil {
		t.Fatalf("insert schedule: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO reservations (id, code, user_id, trip_id, schedule_id, status, participants_count,
			unit_price_idr, total_amount_idr, discount_idr, currency, pickup_point, lock_expires_at)
		VALUES (?, ?, ?, ?, ?, 'PENDING_PAYMENT', 2, 1000000, 2000000, 0, 'IDR', 'Jakarta Pusat', ?)
	`, reservationID, "KLN-DOC-"+suffix, userID, tripID, scheduleID, time.Now().UTC().Add(15*time.Minute)).Error; err != nil {
		t.Fatalf("insert reservation: %v", err)
	}
	for i, participantID := range []uuid.UUID{participantA, participantB} {
		if err := db.Exec(`
			INSERT INTO participants (id, reservation_id, full_name, gender, email, phone, is_primary)
			VALUES (?, ?, ?, 'MALE', ?, '081200000000', ?)
		`, participantID, reservationID, "Peserta "+strconv.Itoa(i+1), "peserta"+strconv.Itoa(i+1)+"@example.com", i == 0).Error; err != nil {
			t.Fatalf("insert participant: %v", err)
		}
	}

	fixture := &documentFixture{
		db: db, service: svc, storeRoot: storeRoot,
		userID: userID, otherUserID: otherUserID, reservationID: reservationID,
		participants: []uuid.UUID{participantA, participantB},
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM documents WHERE reservation_id = ?`, reservationID)
		db.Exec(`DELETE FROM participants WHERE reservation_id = ?`, reservationID)
		db.Exec(`DELETE FROM reservations WHERE id = ?`, reservationID)
		db.Exec(`DELETE FROM trip_schedules WHERE id = ?`, scheduleID)
		db.Exec(`DELETE FROM trips WHERE id = ?`, tripID)
		db.Exec(`DELETE FROM users WHERE id IN (?, ?)`, userID, otherUserID)
	})
	return fixture
}

func testPDF(size int) []byte {
	data := make([]byte, size)
	copy(data, "%PDF-1.7\n")
	copy(data[size-5:], "%%EOF")
	return data
}

func errorCode(t *testing.T, err error) string {
	t.Helper()
	appErr, ok := errs.As(err)
	if !ok {
		t.Fatalf("expected AppError, got %v", err)
	}
	return appErr.Code
}

func TestIntegrationDocumentLifecycle(t *testing.T) {
	f := newDocumentFixture(t)
	ctx := context.Background()
	data := testPDF(2 * 1024 * 1024)
	sum := sha256.Sum256(data)

	uploaded, err := f.service.Upload(ctx, f.userID.String(), f.participants[0].String(), "KTP", data)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if uploaded.SizeBytes != int64(len(data)) || uploaded.Status != "UPLOADED" {
		t.Fatalf("upload response = %+v", uploaded)
	}

	var storedPath string
	err = filepath.Walk(f.storeRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			storedPath = path
		}
		return nil
	})
	if err != nil || storedPath == "" {
		t.Fatalf("walk store: %v, path=%q", err, storedPath)
	}
	raw, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if bytes.HasPrefix(raw, []byte("%PDF")) {
		t.Fatal("stored file is not encrypted")
	}
	if len(raw) != len(data)+12+16 {
		t.Fatalf("stored size = %d, want %d", len(raw), len(data)+28)
	}

	signed, err := f.service.SignedURL(ctx, f.userID.String(), uploaded.ID)
	if err != nil {
		t.Fatalf("SignedURL: %v", err)
	}
	parsed, err := url.Parse(signed.DownloadURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	exp, err := strconv.ParseInt(parsed.Query().Get("exp"), 10, 64)
	if err != nil {
		t.Fatalf("parse exp: %v", err)
	}
	doc, downloaded, err := f.service.Download(ctx, uploaded.ID, exp, parsed.Query().Get("sig"))
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if doc.MimeType != "application/pdf" {
		t.Fatalf("mime = %s", doc.MimeType)
	}
	if !bytes.Equal(downloaded, data) {
		t.Fatal("downloaded bytes differ from original")
	}
	if got := sha256.Sum256(downloaded); hex.EncodeToString(got[:]) != hex.EncodeToString(sum[:]) {
		t.Fatal("downloaded sha256 differs")
	}
	if _, _, err := f.service.Download(ctx, uploaded.ID, exp, "bad-signature"); errorCode(t, err) != errs.CodeSignatureInvalid {
		t.Fatalf("bad signature code = %s", errorCode(t, err))
	}

	if _, err := f.service.SignedURL(ctx, f.otherUserID.String(), uploaded.ID); errorCode(t, err) != errs.CodeDocumentNotFound {
		t.Fatalf("other user SignedURL code = %s", errorCode(t, err))
	}

	completeness, err := f.service.Completeness(ctx, f.userID.String(), f.reservationID.String())
	if err != nil {
		t.Fatalf("Completeness: %v", err)
	}
	if completeness.Complete || len(completeness.Missing) != 1 {
		t.Fatalf("completeness before second upload = %+v", completeness)
	}
	if _, err := f.service.Upload(ctx, f.userID.String(), f.participants[1].String(), "KTP", data); err != nil {
		t.Fatalf("Upload second participant: %v", err)
	}
	completeness, err = f.service.Completeness(ctx, f.userID.String(), f.reservationID.String())
	if err != nil {
		t.Fatalf("Completeness: %v", err)
	}
	if !completeness.Complete || len(completeness.Missing) != 0 {
		t.Fatalf("completeness after second upload = %+v", completeness)
	}

	if err := f.service.Delete(ctx, f.userID.String(), uploaded.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(storedPath); !os.IsNotExist(err) {
		t.Fatalf("stored file still exists, stat err=%v", err)
	}
	if err := f.service.Delete(ctx, f.userID.String(), uploaded.ID); errorCode(t, err) != errs.CodeDocumentNotFound {
		t.Fatalf("second delete code = %s", errorCode(t, err))
	}

	if err := f.db.Exec(`UPDATE reservations SET status = 'PAID' WHERE id = ?`, f.reservationID).Error; err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	docs, err := f.db.Raw(`SELECT id FROM documents WHERE reservation_id = ?`, f.reservationID).Rows()
	if err != nil {
		t.Fatalf("list docs: %v", err)
	}
	var remainingID string
	for docs.Next() {
		if err := docs.Scan(&remainingID); err != nil {
			t.Fatalf("scan doc: %v", err)
		}
	}
	docs.Close()
	if remainingID == "" {
		t.Fatal("expected one remaining document")
	}
	if err := f.service.Delete(ctx, f.userID.String(), remainingID); errorCode(t, err) != errs.CodeAlreadyPaid {
		t.Fatalf("delete after paid code = %s", errorCode(t, err))
	}
	if _, err := f.service.Upload(ctx, f.userID.String(), f.participants[0].String(), "KTP", data); errorCode(t, err) != errs.CodeAlreadyPaid {
		t.Fatalf("upload after paid code = %s", errorCode(t, err))
	}
}

func TestIntegrationDocumentRejectsExpiredLockAndForeignParticipant(t *testing.T) {
	f := newDocumentFixture(t)
	ctx := context.Background()
	data := testPDF(1024)

	if _, err := f.service.Upload(ctx, f.otherUserID.String(), f.participants[0].String(), "KTP", data); errorCode(t, err) != errs.CodeParticipantMissing {
		t.Fatalf("foreign participant code = %s", errorCode(t, err))
	}
	if err := f.db.Exec(`UPDATE reservations SET lock_expires_at = ? WHERE id = ?`, time.Now().UTC().Add(-time.Minute), f.reservationID).Error; err != nil {
		t.Fatalf("expire lock: %v", err)
	}
	if _, err := f.service.Upload(ctx, f.userID.String(), f.participants[0].String(), "KTP", data); errorCode(t, err) != errs.CodeLockExpired {
		t.Fatalf("expired lock code = %s", errorCode(t, err))
	}
}
