//go:build integration

package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	redisinfra "github.com/khorihutama/kelana-api/internal/infra/redis"
)

func TestIntegrationPaymentExpiryReleasesSeatOnce(t *testing.T) {
	f := newPaymentFixture(t, true)
	ctx := context.Background()

	created, _, err := f.service.Create(ctx, f.userID.String(), f.reservationID.String(), "VA_BCA")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	paymentID := uuid.Must(uuid.Parse(created.ID))
	if available, _ := redisinfra.NewSeatHoldStore(f.rdb).Available(ctx, f.scheduleID); available != 8 {
		t.Fatalf("available before expiry = %d, want 8", available)
	}
	if err := f.db.Exec(`UPDATE payments SET expires_at = ? WHERE id = ?`,
		time.Now().UTC().Add(-time.Minute), paymentID).Error; err != nil {
		t.Fatalf("expire payment: %v", err)
	}

	count, err := f.service.ExpireDue(ctx)
	if err != nil || count != 1 {
		t.Fatalf("ExpireDue count=%d err=%v, want 1", count, err)
	}
	if f.paymentStatus(t, paymentID) != "EXPIRED" {
		t.Fatalf("payment status = %s, want EXPIRED", f.paymentStatus(t, paymentID))
	}
	if f.reservationStatus(t) != "EXPIRED" {
		t.Fatalf("reservation status = %s, want EXPIRED", f.reservationStatus(t))
	}
	if available, _ := redisinfra.NewSeatHoldStore(f.rdb).Available(ctx, f.scheduleID); available != 10 {
		t.Fatalf("available after expiry = %d, want 10", available)
	}

	count, err = f.service.ExpireDue(ctx)
	if err != nil || count != 0 {
		t.Fatalf("second ExpireDue count=%d err=%v, want 0", count, err)
	}
	if available, _ := redisinfra.NewSeatHoldStore(f.rdb).Available(ctx, f.scheduleID); available != 10 {
		t.Fatalf("available after second expiry = %d, want 10", available)
	}
}

func TestIntegrationPaymentExpiryNeverTouchesPaidReservation(t *testing.T) {
	f := newPaymentFixture(t, true)
	ctx := context.Background()

	created, _, err := f.service.Create(ctx, f.userID.String(), f.reservationID.String(), "QRIS")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	paymentID := uuid.Must(uuid.Parse(created.ID))
	headers, body := f.webhook("evt-expiry-paid", paymentID, 5900000)
	if err := f.service.HandleWebhook(ctx, "mock", headers, body); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	if f.seatsSold(t) != 2 {
		t.Fatalf("seats sold after paid = %d, want 2", f.seatsSold(t))
	}

	if err := f.db.Exec(`UPDATE payments SET expires_at = ? WHERE id = ?`,
		time.Now().UTC().Add(-time.Minute), paymentID).Error; err != nil {
		t.Fatalf("expire timestamp: %v", err)
	}
	count, err := f.service.ExpireDue(ctx)
	if err != nil || count != 0 {
		t.Fatalf("ExpireDue count=%d err=%v, want 0", count, err)
	}
	if f.paymentStatus(t, paymentID) != "PAID" || f.reservationStatus(t) != "PAID" {
		t.Fatalf("paid state changed: payment=%s reservation=%s", f.paymentStatus(t, paymentID), f.reservationStatus(t))
	}
	if f.seatsSold(t) != 2 {
		t.Fatalf("seats sold = %d, want 2", f.seatsSold(t))
	}
	if available, _ := redisinfra.NewSeatHoldStore(f.rdb).Available(ctx, f.scheduleID); available != 8 {
		t.Fatalf("available = %d, want 8", available)
	}
}
