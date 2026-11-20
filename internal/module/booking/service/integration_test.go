//go:build integration

package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"github.com/khorihutama/kelana-api/internal/common/errs"
	"github.com/khorihutama/kelana-api/internal/config"
	"github.com/khorihutama/kelana-api/internal/infra/database"
	redisinfra "github.com/khorihutama/kelana-api/internal/infra/redis"
	"github.com/khorihutama/kelana-api/internal/module/booking/model/dto"
	bookingrepo "github.com/khorihutama/kelana-api/internal/module/booking/repository"
)

type fixture struct {
	db         *gorm.DB
	repo       bookingrepo.Repository
	store      *redisinfra.SeatHoldStore
	service    *Service
	tripID     uuid.UUID
	scheduleID uuid.UUID
	userIDs    []uuid.UUID
}

func newFixture(t *testing.T, quota int, ttl time.Duration) *fixture {
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
	repo := bookingrepo.New(db)
	store := redisinfra.NewSeatHoldStore(rdb)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := New(repo, store, Config{SeatLockTTL: ttl, SeatSweepInterval: time.Second}, logger)

	suffix := uuid.NewString()[:8]
	tripID := uuid.Must(uuid.NewV7())
	scheduleID := uuid.Must(uuid.NewV7())

	if err := db.Exec(`
		INSERT INTO trips (id, slug, title, description, category, departure_city, destination_city,
			duration_days, base_price_idr, currency, status, required_documents, rating_avg, rating_count)
		VALUES (?, ?, 'Test Trip', 'Test', 'OPEN', 'Jakarta', 'Bali', 2, 1000000, 'IDR', 'PUBLISHED', '{KTP}', 0, 0)
	`, tripID, "test-trip-"+suffix).Error; err != nil {
		t.Fatalf("insert trip: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO trip_schedules (id, trip_id, start_date, end_date, departure_point, quota_total, seats_sold, price_idr, currency, status)
		VALUES (?, ?, '2027-03-01', '2027-03-02', 'Jakarta', ?, 0, 1000000, 'IDR', 'OPEN')
	`, scheduleID, tripID, quota).Error; err != nil {
		t.Fatalf("insert schedule: %v", err)
	}

	f := &fixture{db: db, repo: repo, store: store, service: svc, tripID: tripID, scheduleID: scheduleID}
	t.Cleanup(f.cleanup)
	return f
}

func (f *fixture) createUsers(t *testing.T, count int) []uuid.UUID {
	t.Helper()
	ids := make([]uuid.UUID, 0, count)
	for i := 0; i < count; i++ {
		id := uuid.Must(uuid.NewV7())
		email := fmt.Sprintf("lock-test-%s-%d@example.com", uuid.NewString()[:8], i)
		if err := f.db.Exec(`
			INSERT INTO users (id, email, password_hash, name, role, status, email_verified_at)
			VALUES (?, ?, 'x', 'Lock Test', 'CUSTOMER', 'ACTIVE', now())
		`, id, email).Error; err != nil {
			t.Fatalf("insert user: %v", err)
		}
		ids = append(ids, id)
	}
	f.userIDs = ids
	return ids
}

func (f *fixture) cleanup() {
	f.db.Exec(`DELETE FROM participants WHERE reservation_id IN (SELECT id FROM reservations WHERE schedule_id = ?)`, f.scheduleID)
	f.db.Exec(`DELETE FROM reservations WHERE schedule_id = ?`, f.scheduleID)
	f.db.Exec(`DELETE FROM trip_schedules WHERE id = ?`, f.scheduleID)
	f.db.Exec(`DELETE FROM trips WHERE id = ?`, f.tripID)
	for _, id := range f.userIDs {
		f.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	}
}

func lockRequest(scheduleID uuid.UUID) dto.LockRequest {
	return dto.LockRequest{
		ScheduleID: scheduleID.String(),
		Participants: []dto.ParticipantInput{{
			FullName: "Lock Test", Gender: "MALE", Phone: "081200000000", Email: "lock@example.com",
		}},
		PickupPoint: "Jakarta Pusat",
	}
}

func TestConcurrentLockNoOversell(t *testing.T) {
	f := newFixture(t, 5, 15*time.Minute)
	users := f.createUsers(t, 100)
	ctx := context.Background()

	stopSweeper := make(chan struct{})
	var sweeperWG sync.WaitGroup
	sweeperWG.Add(1)
	go func() {
		defer sweeperWG.Done()
		ticker := time.NewTicker(2 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopSweeper:
				return
			case <-ticker.C:
				_, _ = f.service.SweepOnce(ctx)
			}
		}
	}()

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, len(users))
	for _, userID := range users {
		wg.Add(1)
		go func(id uuid.UUID) {
			defer wg.Done()
			<-start
			_, err := f.service.Lock(ctx, id.String(), lockRequest(f.scheduleID))
			results <- err
		}(userID)
	}
	close(start)
	wg.Wait()
	close(results)
	close(stopSweeper)
	sweeperWG.Wait()

	success, seatUnavailable, other := 0, 0, 0
	for err := range results {
		if err == nil {
			success++
			continue
		}
		if appErr, ok := errs.As(err); ok && appErr.Code == errs.CodeSeatUnavailable {
			seatUnavailable++
			continue
		}
		other++
		t.Logf("unexpected lock error: %v", err)
	}
	available, err := f.store.Available(ctx, f.scheduleID)
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	t.Logf("success=%d seatUnavailable=%d other=%d available=%d", success, seatUnavailable, other, available)
	if success != 5 || seatUnavailable != 95 || other != 0 || available != 0 {
		t.Fatalf("success=%d seatUnavailable=%d other=%d available=%d; want 5/95/0/0",
			success, seatUnavailable, other, available)
	}
}

func TestExpiryReleasesSeatsAndMarksExpired(t *testing.T) {
	f := newFixture(t, 2, 2*time.Second)
	users := f.createUsers(t, 1)
	ctx := context.Background()

	result, err := f.service.Lock(ctx, users[0].String(), lockRequest(f.scheduleID))
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	availableBefore, _ := f.store.Available(ctx, f.scheduleID)
	if availableBefore != 1 {
		t.Fatalf("available before expiry = %d; want 1", availableBefore)
	}

	time.Sleep(3 * time.Second)
	if _, err := f.service.SweepOnce(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	reservation, err := f.repo.GetReservation(ctx, uuid.MustParse(result.ID))
	if err != nil || reservation == nil {
		t.Fatalf("get reservation: %v", err)
	}
	availableAfter, _ := f.store.Available(ctx, f.scheduleID)
	t.Logf("status=%s available_after=%d", reservation.Status, availableAfter)
	if reservation.Status != "EXPIRED" || availableAfter != 2 {
		t.Fatalf("status=%s available=%d; want EXPIRED/2", reservation.Status, availableAfter)
	}
}

func TestCancelReturnsSeats(t *testing.T) {
	f := newFixture(t, 2, 15*time.Minute)
	users := f.createUsers(t, 1)
	ctx := context.Background()

	result, err := f.service.Lock(ctx, users[0].String(), lockRequest(f.scheduleID))
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	cancelled, err := f.service.Cancel(ctx, users[0].String(), result.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	available, _ := f.store.Available(ctx, f.scheduleID)
	t.Logf("status=%s available_after=%d", cancelled.Status, available)
	if cancelled.Status != "CANCELLED" || available != 2 {
		t.Fatalf("status=%s available=%d; want CANCELLED/2", cancelled.Status, available)
	}
}
