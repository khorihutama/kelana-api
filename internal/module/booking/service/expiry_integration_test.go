//go:build integration

package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIntegrationSweepExpiresDueReservationFromDB(t *testing.T) {
	f := newFixture(t, 5, time.Minute)
	userID := f.createUsers(t, 1)[0]
	ctx := context.Background()

	locked, err := f.service.Lock(ctx, userID.String(), lockRequest(f.scheduleID))
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	reservationID := uuid.Must(uuid.Parse(locked.ID))
	if available, _ := f.store.Available(ctx, f.scheduleID); available != 4 {
		t.Fatalf("available after lock = %d, want 4", available)
	}
	if err := f.db.Exec(`UPDATE reservations SET lock_expires_at = ? WHERE id = ?`,
		time.Now().UTC().Add(-time.Minute), reservationID).Error; err != nil {
		t.Fatalf("expire lock: %v", err)
	}

	count, err := f.service.SweepOnce(ctx)
	if err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	if count != 1 {
		t.Fatalf("sweep count = %d, want 1", count)
	}
	var status string
	if err := f.db.Raw(`SELECT status FROM reservations WHERE id = ?`, reservationID).Scan(&status).Error; err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != "EXPIRED" {
		t.Fatalf("status = %s, want EXPIRED", status)
	}
	if available, _ := f.store.Available(ctx, f.scheduleID); available != 5 {
		t.Fatalf("available after sweep = %d, want 5", available)
	}

	count, err = f.service.SweepOnce(ctx)
	if err != nil || count != 0 {
		t.Fatalf("second sweep count=%d err=%v, want 0", count, err)
	}
	if available, _ := f.store.Available(ctx, f.scheduleID); available != 5 {
		t.Fatalf("available after second sweep = %d, want 5", available)
	}
}
