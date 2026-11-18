//go:build integration

package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/khorihutama/hsi-go-api/internal/common/errs"
	"github.com/khorihutama/hsi-go-api/internal/config"
	"github.com/khorihutama/hsi-go-api/internal/infra/database"
	paymentinfra "github.com/khorihutama/hsi-go-api/internal/infra/payment"
	redisinfra "github.com/khorihutama/hsi-go-api/internal/infra/redis"
	"github.com/khorihutama/hsi-go-api/internal/module/payment/port"
	paymentrepo "github.com/khorihutama/hsi-go-api/internal/module/payment/repository"
)

const testWebhookSecret = "integration-secret-123456"

type stubDocumentGate struct {
	complete bool
	err      error
}

func (s stubDocumentGate) CompleteForReservation(ctx context.Context, reservationID uuid.UUID) (bool, error) {
	return s.complete, s.err
}

type paymentFixture struct {
	db            *gorm.DB
	rdb           *goredis.Client
	service       *Service
	userID        uuid.UUID
	otherUserID   uuid.UUID
	scheduleID    uuid.UUID
	reservationID uuid.UUID
}

func newPaymentFixture(t *testing.T, docsComplete bool) *paymentFixture {
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
	rdb, err := redisinfra.Connect(context.Background(), cfg.RedisAddr)
	if err != nil {
		t.Fatalf("redis: %v", err)
	}
	seatStore := redisinfra.NewSeatHoldStore(rdb)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := New(paymentrepo.New(db), seatStore, stubDocumentGate{complete: docsComplete},
		[]port.Provider{paymentinfra.NewMock(testWebhookSecret)},
		Config{Provider: "mock", PaymentTTL: 24 * time.Hour}, logger)

	suffix := uuid.NewString()[:8]
	userID := uuid.Must(uuid.NewV7())
	otherUserID := uuid.Must(uuid.NewV7())
	tripID := uuid.Must(uuid.NewV7())
	scheduleID := uuid.Must(uuid.NewV7())
	reservationID := uuid.Must(uuid.NewV7())

	for _, user := range []uuid.UUID{userID, otherUserID} {
		if err := db.Exec(`
			INSERT INTO users (id, email, password_hash, name, role, status, email_verified_at)
			VALUES (?, ?, 'x', 'Payment Test', 'CUSTOMER', 'ACTIVE', now())
		`, user, "pay-test-"+user.String()+"@example.com").Error; err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	if err := db.Exec(`
		INSERT INTO trips (id, slug, title, description, category, departure_city, destination_city,
			duration_days, base_price_idr, currency, status, required_documents, rating_avg, rating_count)
		VALUES (?, ?, 'Payment Trip', 'Test', 'OPEN', 'Jakarta', 'Bali', 2, 2950000, 'IDR', 'PUBLISHED', '{KTP}', 0, 0)
	`, tripID, "pay-trip-"+suffix).Error; err != nil {
		t.Fatalf("insert trip: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO trip_schedules (id, trip_id, start_date, end_date, departure_point, quota_total, seats_sold, price_idr, currency, status)
		VALUES (?, ?, '2027-05-01', '2027-05-02', 'Jakarta', 10, 0, 2950000, 'IDR', 'OPEN')
	`, scheduleID, tripID).Error; err != nil {
		t.Fatalf("insert schedule: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO reservations (id, code, user_id, trip_id, schedule_id, status, participants_count,
			unit_price_idr, total_amount_idr, discount_idr, currency, pickup_point, lock_expires_at)
		VALUES (?, ?, ?, ?, ?, 'PENDING_PAYMENT', 2, 2950000, 5900000, 0, 'IDR', 'Jakarta Pusat', ?)
	`, reservationID, "HSI-PAY-"+suffix, userID, tripID, scheduleID, time.Now().UTC().Add(15*time.Minute)).Error; err != nil {
		t.Fatalf("insert reservation: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := db.Exec(`
			INSERT INTO participants (id, reservation_id, full_name, gender, email, phone, is_primary)
			VALUES (?, ?, ?, 'MALE', ?, '081200000000', ?)
		`, uuid.Must(uuid.NewV7()), reservationID, fmt.Sprintf("Peserta %d", i+1),
			fmt.Sprintf("pay-peserta-%s-%d@example.com", suffix, i), i == 0).Error; err != nil {
			t.Fatalf("insert participant: %v", err)
		}
	}

	ctx := context.Background()
	if err := seatStore.InitIfMissing(ctx, scheduleID, 10); err != nil {
		t.Fatalf("init seats: %v", err)
	}
	if err := seatStore.SetAvailable(ctx, scheduleID, 10); err != nil {
		t.Fatalf("set seats: %v", err)
	}
	if remaining, err := seatStore.Lock(ctx, scheduleID, reservationID, 2, 15*time.Minute, 30*time.Minute); err != nil || remaining != 8 {
		t.Fatalf("lock seats: remaining=%d err=%v", remaining, err)
	}

	fixture := &paymentFixture{
		db: db, rdb: rdb, service: svc,
		userID: userID, otherUserID: otherUserID,
		scheduleID: scheduleID, reservationID: reservationID,
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM reservations WHERE id = ?`, reservationID)
		db.Exec(`DELETE FROM trip_schedules WHERE id = ?`, scheduleID)
		db.Exec(`DELETE FROM trips WHERE id = ?`, tripID)
		db.Exec(`DELETE FROM users WHERE id IN (?, ?)`, userID, otherUserID)
		rdb.Del(context.Background(),
			"seat:avail:"+scheduleID.String(),
			"seat:holds:"+scheduleID.String(),
			"seat:hold:"+reservationID.String(),
		)
	})
	return fixture
}

func (f *paymentFixture) webhook(eventID string, paymentID uuid.UUID, amount int64) (http.Header, []byte) {
	body := []byte(fmt.Sprintf(`{"event_id":%q,"payment_id":%q,"status":"PAID","amount_idr":%d}`,
		eventID, paymentID.String(), amount))
	mac := hmac.New(sha256.New, []byte(testWebhookSecret))
	mac.Write(body)
	headers := http.Header{}
	headers.Set(paymentinfra.SignatureHeader, hex.EncodeToString(mac.Sum(nil)))
	return headers, body
}

func (f *paymentFixture) seatsSold(t *testing.T) int {
	t.Helper()
	var value int
	if err := f.db.Raw(`SELECT seats_sold FROM trip_schedules WHERE id = ?`, f.scheduleID).Scan(&value).Error; err != nil {
		t.Fatalf("seats sold: %v", err)
	}
	return value
}

func (f *paymentFixture) reservationStatus(t *testing.T) string {
	t.Helper()
	var value string
	if err := f.db.Raw(`SELECT status FROM reservations WHERE id = ?`, f.reservationID).Scan(&value).Error; err != nil {
		t.Fatalf("reservation status: %v", err)
	}
	return value
}

func (f *paymentFixture) paymentStatus(t *testing.T, paymentID uuid.UUID) string {
	t.Helper()
	var value string
	if err := f.db.Raw(`SELECT status FROM payments WHERE id = ?`, paymentID).Scan(&value).Error; err != nil {
		t.Fatalf("payment status: %v", err)
	}
	return value
}

func (f *paymentFixture) eventCount(t *testing.T, eventID string) int {
	t.Helper()
	var value int
	if err := f.db.Raw(`SELECT count(*) FROM payment_events WHERE provider = 'MOCK' AND provider_event_id = ?`, eventID).Scan(&value).Error; err != nil {
		t.Fatalf("event count: %v", err)
	}
	return value
}

func mustCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s, got nil", code)
	}
	appErr, ok := errs.As(err)
	if !ok || appErr.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}

func TestIntegrationPaymentWebhookIdempotent(t *testing.T) {
	f := newPaymentFixture(t, true)
	ctx := context.Background()

	available, err := redisinfra.NewSeatHoldStore(f.rdb).Available(ctx, f.scheduleID)
	if err != nil || available != 8 {
		t.Fatalf("available before = %d, %v", available, err)
	}

	created, isNew, err := f.service.Create(ctx, f.userID.String(), f.reservationID.String(), "VA_BCA")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !isNew || created.AmountIDR != 5900000 || created.Method != "VA_BCA" || created.VANumber == nil {
		t.Fatalf("created = %+v new=%v", created, isNew)
	}
	expires, err := time.Parse(time.RFC3339, created.ExpiresAt)
	if err != nil {
		t.Fatalf("expires_at: %v", err)
	}
	if delta := time.Until(expires); delta < 23*time.Hour || delta > 25*time.Hour {
		t.Fatalf("expires_at delta = %v", delta)
	}
	paymentID := uuid.Must(uuid.Parse(created.ID))

	reused, isNew, err := f.service.Create(ctx, f.userID.String(), f.reservationID.String(), "VA_BCA")
	if err != nil || isNew || reused.ID != created.ID {
		t.Fatalf("reused = %+v new=%v err=%v", reused, isNew, err)
	}

	headers, body := f.webhook("evt-1", paymentID, 5900000)
	badHeaders := http.Header{}
	badHeaders.Set(paymentinfra.SignatureHeader, "deadbeef")
	mustCode(t, f.service.HandleWebhook(ctx, "mock", badHeaders, body), errs.CodeInvalidSignature)
	if f.reservationStatus(t) != "PENDING_PAYMENT" || f.seatsSold(t) != 0 {
		t.Fatal("invalid signature changed state")
	}

	if err := f.service.HandleWebhook(ctx, "mock", headers, body); err != nil {
		t.Fatalf("valid webhook: %v", err)
	}
	if f.reservationStatus(t) != "PAID" || f.paymentStatus(t, paymentID) != "PAID" || f.seatsSold(t) != 2 {
		t.Fatalf("after paid: reservation=%s payment=%s sold=%d", f.reservationStatus(t), f.paymentStatus(t, paymentID), f.seatsSold(t))
	}
	if available, _ := redisinfra.NewSeatHoldStore(f.rdb).Available(ctx, f.scheduleID); available != 8 {
		t.Fatalf("available after paid = %d, want 8", available)
	}
	if exists := f.rdb.Exists(ctx, "seat:hold:"+f.reservationID.String()).Val(); exists != 0 {
		t.Fatal("hold key still exists after paid")
	}

	if err := f.service.HandleWebhook(ctx, "mock", headers, body); err != nil {
		t.Fatalf("duplicate webhook: %v", err)
	}
	if f.seatsSold(t) != 2 || f.eventCount(t, "evt-1") != 1 {
		t.Fatalf("duplicate changed state: sold=%d events=%d", f.seatsSold(t), f.eventCount(t, "evt-1"))
	}
	if available, _ := redisinfra.NewSeatHoldStore(f.rdb).Available(ctx, f.scheduleID); available != 8 {
		t.Fatalf("available after duplicate = %d, want 8", available)
	}

	got, err := f.service.Get(ctx, f.userID.String(), paymentID.String())
	if err != nil || got.Status != "PAID" {
		t.Fatalf("Get owner = %+v, %v", got, err)
	}
	if _, err := f.service.Get(ctx, f.otherUserID.String(), paymentID.String()); err == nil {
		t.Fatal("other user could read payment")
	}
	if _, _, err := f.service.Create(ctx, f.userID.String(), f.reservationID.String(), "QRIS"); err == nil {
		t.Fatal("created payment after PAID")
	}
	if err := f.service.Refund(ctx, "mock", "MOCK-1", 5900000, "test"); err != port.ErrNotSupported {
		t.Fatalf("Refund err = %v", err)
	}
}

func TestIntegrationConcurrentIdenticalWebhooks(t *testing.T) {
	f := newPaymentFixture(t, true)
	ctx := context.Background()
	created, _, err := f.service.Create(ctx, f.userID.String(), f.reservationID.String(), "QRIS")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	paymentID := uuid.Must(uuid.Parse(created.ID))
	headers, body := f.webhook("evt-parallel", paymentID, 5900000)

	const workers = 20
	start := make(chan struct{})
	errsCh := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errsCh <- f.service.HandleWebhook(ctx, "mock", headers, body)
		}()
	}
	close(start)
	wg.Wait()
	close(errsCh)
	for err := range errsCh {
		if err != nil {
			t.Fatalf("parallel webhook: %v", err)
		}
	}

	if f.seatsSold(t) != 2 {
		t.Fatalf("seats_sold = %d, want 2", f.seatsSold(t))
	}
	if f.eventCount(t, "evt-parallel") != 1 {
		t.Fatalf("event rows = %d, want 1", f.eventCount(t, "evt-parallel"))
	}
	if available, _ := redisinfra.NewSeatHoldStore(f.rdb).Available(ctx, f.scheduleID); available != 8 {
		t.Fatalf("available = %d, want 8", available)
	}
}

func TestIntegrationPaymentGuards(t *testing.T) {
	f := newPaymentFixture(t, false)
	ctx := context.Background()
	_, _, err := f.service.Create(ctx, f.userID.String(), f.reservationID.String(), "VA_BCA")
	mustCode(t, err, errs.CodeDocumentsIncomplete)

	f2 := newPaymentFixture(t, true)
	if err := f2.db.Exec(`UPDATE reservations SET lock_expires_at = ? WHERE id = ?`,
		time.Now().UTC().Add(-time.Minute), f2.reservationID).Error; err != nil {
		t.Fatalf("expire lock: %v", err)
	}
	_, _, err = f2.service.Create(ctx, f2.userID.String(), f2.reservationID.String(), "VA_BCA")
	mustCode(t, err, errs.CodeLockExpired)

	f3 := newPaymentFixture(t, true)
	if err := f3.db.Exec(`UPDATE reservations SET status = 'PAID' WHERE id = ?`, f3.reservationID).Error; err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	_, _, err = f3.service.Create(ctx, f3.userID.String(), f3.reservationID.String(), "VA_BCA")
	mustCode(t, err, errs.CodeAlreadyPaid)

	f4 := newPaymentFixture(t, true)
	created, _, err := f4.service.Create(ctx, f4.userID.String(), f4.reservationID.String(), "VA_BCA")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := f4.db.Exec(`UPDATE trip_schedules SET quota_total = 1 WHERE id = ?`, f4.scheduleID).Error; err != nil {
		t.Fatalf("shrink quota: %v", err)
	}
	headers, body := f4.webhook("evt-quota", uuid.Must(uuid.Parse(created.ID)), 5900000)
	mustCode(t, f4.service.HandleWebhook(ctx, "mock", headers, body), errs.CodeSeatUnavailable)
	if f4.reservationStatus(t) != "PENDING_PAYMENT" {
		t.Fatal("quota failure changed reservation")
	}
}
