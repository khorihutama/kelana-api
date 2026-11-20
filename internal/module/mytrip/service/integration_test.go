//go:build integration

package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"github.com/khorihutama/kelana-api/internal/common/errs"
	"github.com/khorihutama/kelana-api/internal/config"
	"github.com/khorihutama/kelana-api/internal/infra/database"
	redisinfra "github.com/khorihutama/kelana-api/internal/infra/redis"
	bookingrepo "github.com/khorihutama/kelana-api/internal/module/booking/repository"
	bookingsvc "github.com/khorihutama/kelana-api/internal/module/booking/service"
	mytriprepo "github.com/khorihutama/kelana-api/internal/module/mytrip/repository"
)

type myTripFixture struct {
	db          *gorm.DB
	service     *Service
	userID      uuid.UUID
	otherUserID uuid.UUID
	tripID      uuid.UUID
	pendingID   uuid.UUID
	siapID      uuid.UUID
	selesaiID   uuid.UUID
	cancelledID uuid.UUID
	expiredID   uuid.UUID
	otherID     uuid.UUID
	scheduleIDs []uuid.UUID
	paymentID   uuid.UUID
}

func newMyTripFixture(t *testing.T) *myTripFixture {
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
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	seatStore := redisinfra.NewSeatHoldStore(rdb)
	bookingSvc := bookingsvc.New(bookingrepo.New(db), seatStore, bookingsvc.Config{
		SeatLockTTL: 15 * time.Minute, SeatHoldSafetyTTL: 30 * time.Minute, SeatSweepInterval: time.Minute,
	}, logger)
	svc := New(mytriprepo.New(db), bookingSvc, Config{WAGroupURL: "https://chat.whatsapp.com/fallback"}, logger)

	suffix := uuid.NewString()[:8]
	userID := uuid.Must(uuid.NewV7())
	otherUserID := uuid.Must(uuid.NewV7())
	tripID := uuid.Must(uuid.NewV7())
	for _, user := range []uuid.UUID{userID, otherUserID} {
		if err := db.Exec(`
			INSERT INTO users (id, email, password_hash, name, role, status, email_verified_at)
			VALUES (?, ?, 'x', 'MyTrip Test', 'CUSTOMER', 'ACTIVE', now())
		`, user, "mytrip-test-"+user.String()+"@example.com").Error; err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	tripWA := "https://chat.whatsapp.com/trip-mytrip"
	if err := db.Exec(`
		INSERT INTO trips (id, slug, title, description, category, departure_city, destination_city,
			duration_days, base_price_idr, currency, status, required_documents, rating_avg, rating_count, wa_group_url)
		VALUES (?, ?, 'MyTrip Trip', 'Test', 'OPEN', 'Jakarta', 'Bali', 2, 1000000, 'IDR', 'PUBLISHED', '{KTP}', 0, 0, ?)
	`, tripID, "mytrip-trip-"+suffix, tripWA).Error; err != nil {
		t.Fatalf("insert trip: %v", err)
	}

	// Distinct schedules keep the active-reservation unique index valid.
	offsets := []int{10, 20, 30, 40, 50, -10}
	scheduleIDs := make([]uuid.UUID, 0, len(offsets))
	for _, offset := range offsets {
		id := uuid.Must(uuid.NewV7())
		start := time.Now().UTC().AddDate(0, 0, offset)
		if err := db.Exec(`
			INSERT INTO trip_schedules (id, trip_id, start_date, end_date, departure_point, quota_total, seats_sold, price_idr, currency, status)
			VALUES (?, ?, ?, ?, 'Jakarta', 10, 0, 1000000, 'IDR', 'OPEN')
		`, id, tripID, start.Format("2006-01-02"), start.AddDate(0, 0, 2).Format("2006-01-02")).Error; err != nil {
			t.Fatalf("insert schedule: %v", err)
		}
		scheduleIDs = append(scheduleIDs, id)
	}

	pendingID := uuid.Must(uuid.NewV7())
	siapID := uuid.Must(uuid.NewV7())
	selesaiID := uuid.Must(uuid.NewV7())
	cancelledID := uuid.Must(uuid.NewV7())
	expiredID := uuid.Must(uuid.NewV7())
	otherID := uuid.Must(uuid.NewV7())
	insertReservation := func(id, user uuid.UUID, schedule uuid.UUID, status, code string, lockExpires time.Time) {
		t.Helper()
		if err := db.Exec(`
			INSERT INTO reservations (id, code, user_id, trip_id, schedule_id, status, participants_count,
				unit_price_idr, total_amount_idr, discount_idr, currency, pickup_point, lock_expires_at, paid_at)
			VALUES (?, ?, ?, ?, ?, ?, 1, 1000000, 1000000, 0, 'IDR', 'Jakarta Pusat', ?, ?)
		`, id, code, user, tripID, schedule, status, lockExpires,
			func() any {
				if status == "PAID" {
					return time.Now().UTC()
				}
				return nil
			}()).Error; err != nil {
			t.Fatalf("insert reservation %s: %v", status, err)
		}
	}
	insertReservation(pendingID, userID, scheduleIDs[0], "PENDING_PAYMENT", "MYT-PENDING-"+suffix, time.Now().UTC().Add(15*time.Minute))
	insertReservation(siapID, userID, scheduleIDs[1], "PAID", "MYT-SIAP-"+suffix, time.Now().UTC().Add(15*time.Minute))
	insertReservation(selesaiID, userID, scheduleIDs[5], "PAID", "MYT-SELESAI-"+suffix, time.Now().UTC().Add(15*time.Minute))
	insertReservation(cancelledID, userID, scheduleIDs[2], "CANCELLED", "MYT-CANCEL-"+suffix, time.Now().UTC().Add(15*time.Minute))
	insertReservation(expiredID, userID, scheduleIDs[3], "EXPIRED", "MYT-EXPIRED-"+suffix, time.Now().UTC().Add(15*time.Minute))
	insertReservation(otherID, otherUserID, scheduleIDs[4], "PENDING_PAYMENT", "MYT-OTHER-"+suffix, time.Now().UTC().Add(15*time.Minute))

	participantID := uuid.Must(uuid.NewV7())
	if err := db.Exec(`
		INSERT INTO participants (id, reservation_id, full_name, gender, email, phone, is_primary)
		VALUES (?, ?, 'Peserta MyTrip', 'MALE', 'mytrip-participant@example.com', '081200000000', true)
	`, participantID, pendingID).Error; err != nil {
		t.Fatalf("insert participant: %v", err)
	}

	paymentID := uuid.Must(uuid.NewV7())
	if err := db.Exec(`
		INSERT INTO payments (id, reservation_id, provider, provider_ref, method, amount_idr, currency, status, expires_at)
		VALUES (?, ?, 'MOCK', ?, 'VA_BCA', 1000000, 'IDR', 'PENDING', ?)
	`, paymentID, pendingID, "MOCK-"+pendingID.String(), time.Now().UTC().Add(24*time.Hour)).Error; err != nil {
		t.Fatalf("insert pending payment: %v", err)
	}
	for _, reservationID := range []uuid.UUID{siapID, selesaiID} {
		if err := db.Exec(`
			INSERT INTO payments (id, reservation_id, provider, provider_ref, method, amount_idr, currency, status, expires_at, paid_at)
			VALUES (?, ?, 'MOCK', ?, 'VA_BCA', 1000000, 'IDR', 'PAID', ?, ?)
		`, uuid.Must(uuid.NewV7()), reservationID, "MOCK-"+reservationID.String(),
			time.Now().UTC().Add(24*time.Hour), time.Now().UTC()).Error; err != nil {
			t.Fatalf("insert paid payment: %v", err)
		}
	}
	if err := db.Exec(`
		INSERT INTO documents (id, reservation_id, participant_id, user_id, type, object_key, mime_type, size_bytes, checksum_sha256, status)
		VALUES (?, ?, ?, ?, 'KTP', ?, 'application/pdf', 1024, 'abc', 'UPLOADED')
	`, uuid.Must(uuid.NewV7()), pendingID, participantID, userID,
		fmt.Sprintf("reservations/%s/doc.enc", pendingID)).Error; err != nil {
		t.Fatalf("insert document: %v", err)
	}

	fixture := &myTripFixture{
		db: db, service: svc, userID: userID, otherUserID: otherUserID, tripID: tripID,
		pendingID: pendingID, siapID: siapID, selesaiID: selesaiID,
		cancelledID: cancelledID, expiredID: expiredID, otherID: otherID,
		scheduleIDs: scheduleIDs, paymentID: paymentID,
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM reservations WHERE user_id IN (?, ?)`, userID, otherUserID)
		db.Exec(`DELETE FROM trip_schedules WHERE trip_id = ?`, tripID)
		db.Exec(`DELETE FROM trips WHERE id = ?`, tripID)
		db.Exec(`DELETE FROM users WHERE id IN (?, ?)`, userID, otherUserID)
		for _, scheduleID := range scheduleIDs {
			rdb.Del(context.Background(), "seat:avail:"+scheduleID.String(), "seat:holds:"+scheduleID.String())
		}
	})
	return fixture
}

func TestIntegrationMyTripsTabsOwnershipAndDetail(t *testing.T) {
	f := newMyTripFixture(t)
	ctx := context.Background()

	tabs := []struct {
		tab  string
		want []uuid.UUID
	}{
		{"BELUM_BAYAR", []uuid.UUID{f.pendingID}},
		{"SIAP_BERANGKAT", []uuid.UUID{f.siapID}},
		{"SELESAI", []uuid.UUID{f.selesaiID}},
		{"RIWAYAT", []uuid.UUID{f.cancelledID, f.expiredID}},
	}
	for _, test := range tabs {
		items, total, _, _, err := f.service.List(ctx, f.userID.String(), test.tab, 1, 10)
		if err != nil {
			t.Fatalf("List %s: %v", test.tab, err)
		}
		if total != int64(len(test.want)) || len(items) != len(test.want) {
			t.Fatalf("List %s total=%d items=%d want=%d", test.tab, total, len(items), len(test.want))
		}
		got := make(map[string]struct{}, len(items))
		for _, item := range items {
			got[item.ID] = struct{}{}
			if item.Tab != test.tab {
				t.Fatalf("item %s tab=%s want=%s", item.ID, item.Tab, test.tab)
			}
			if item.ID == f.otherID.String() {
				t.Fatal("other user's reservation leaked")
			}
		}
		for _, id := range test.want {
			if _, ok := got[id.String()]; !ok {
				t.Fatalf("List %s missing %s", test.tab, id)
			}
		}
	}

	all, total, page, limit, err := f.service.List(ctx, f.userID.String(), "", 1, 2)
	if err != nil || total != 5 || len(all) != 2 || page != 1 || limit != 2 {
		t.Fatalf("all page 1: total=%d len=%d page=%d limit=%d err=%v", total, len(all), page, limit, err)
	}
	last, total, _, _, err := f.service.List(ctx, f.userID.String(), "", 3, 2)
	if err != nil || total != 5 || len(last) != 1 {
		t.Fatalf("all page 3: total=%d len=%d err=%v", total, len(last), err)
	}
	if _, _, _, _, err := f.service.List(ctx, f.userID.String(), "NOPE", 1, 10); err == nil {
		t.Fatal("invalid tab accepted")
	}

	pending, err := f.service.Get(ctx, f.userID.String(), f.pendingID.String())
	if err != nil {
		t.Fatalf("Get pending: %v", err)
	}
	if pending.JoinWAURL != nil {
		t.Fatal("join_wa_url leaked before PAID")
	}
	if pending.Payment == nil || pending.Timer == nil || pending.Timer.Kind != "PAYMENT" {
		t.Fatalf("pending payment/timer = %+v %+v", pending.Payment, pending.Timer)
	}
	if len(pending.Participants) != 1 || len(pending.Documents) != 1 {
		t.Fatalf("detail participants=%d documents=%d", len(pending.Participants), len(pending.Documents))
	}

	paid, err := f.service.Get(ctx, f.userID.String(), f.siapID.String())
	if err != nil {
		t.Fatalf("Get paid: %v", err)
	}
	if paid.JoinWAURL == nil || *paid.JoinWAURL == "" {
		t.Fatal("join_wa_url missing after PAID")
	}

	if _, err := f.service.Get(ctx, f.otherUserID.String(), f.pendingID.String()); err == nil {
		t.Fatal("other user read detail")
	} else if appErr, ok := errs.As(err); !ok || appErr.Code != "RESERVATION_NOT_FOUND" {
		t.Fatalf("other user error = %v", err)
	}
}

func TestIntegrationMyTripsCancelRules(t *testing.T) {
	f := newMyTripFixture(t)
	ctx := context.Background()

	cancelled, err := f.service.Cancel(ctx, f.userID.String(), f.pendingID.String())
	if err != nil {
		t.Fatalf("Cancel pending: %v", err)
	}
	if cancelled.Status != "CANCELLED" || cancelled.Tab != "RIWAYAT" {
		t.Fatalf("cancelled detail = %+v", cancelled)
	}
	if _, err := f.service.Cancel(ctx, f.userID.String(), f.pendingID.String()); err == nil {
		t.Fatal("second cancel succeeded")
	} else if appErr, ok := errs.As(err); !ok || appErr.Code != errs.CodeInvalidState {
		t.Fatalf("second cancel code = %v", err)
	}
	if _, err := f.service.Cancel(ctx, f.userID.String(), f.siapID.String()); err == nil {
		t.Fatal("cancel paid succeeded")
	} else if appErr, ok := errs.As(err); !ok || appErr.Code != errs.CodeAlreadyPaid {
		t.Fatalf("cancel paid code = %v", err)
	}
}
