package service

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/khorihutama/hsi-go-api/internal/module/mytrip/repository"
)

func TestTabFor(t *testing.T) {
	today := "2026-11-18"
	future := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	past := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		status string
		end    time.Time
		want   string
	}{
		{"PENDING_PAYMENT", future, "BELUM_BAYAR"},
		{"PAID", future, "SIAP_BERANGKAT"},
		{"PAID", time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC), "SIAP_BERANGKAT"},
		{"PAID", past, "SELESAI"},
		{"COMPLETED", past, "SELESAI"},
		{"CANCELLED", future, "RIWAYAT"},
		{"EXPIRED", future, "RIWAYAT"},
	}
	for _, test := range cases {
		if got := tabFor(test.status, test.end, today); got != test.want {
			t.Fatalf("tabFor(%s, %s) = %s, want %s", test.status, test.end.Format("2006-01-02"), got, test.want)
		}
	}
}

func TestJoinWAGating(t *testing.T) {
	tripURL := "https://chat.whatsapp.com/trip"
	svc := &Service{cfg: Config{WAGroupURL: "https://chat.whatsapp.com/fallback"}}

	if got := svc.joinWA(&repository.ListRow{Status: "PENDING_PAYMENT", TripWAGroupURL: &tripURL}); got != nil {
		t.Fatalf("pending join_wa_url = %v, want nil", *got)
	}
	if got := svc.joinWA(&repository.ListRow{Status: "PAID", TripWAGroupURL: &tripURL}); got == nil || *got != tripURL {
		t.Fatalf("paid trip url = %v", got)
	}
	if got := svc.joinWA(&repository.ListRow{Status: "PAID"}); got == nil || *got != svc.cfg.WAGroupURL {
		t.Fatalf("paid fallback url = %v", got)
	}
	svc.cfg.WAGroupURL = ""
	if got := svc.joinWA(&repository.ListRow{Status: "PAID"}); got != nil {
		t.Fatalf("paid without url = %v, want nil", *got)
	}
}

func TestTimer(t *testing.T) {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	svc := &Service{now: func() time.Time { return now }}
	paymentExpiry := now.Add(24 * time.Hour)
	paymentStatus := "PENDING"
	paymentID := uuid.MustParse("01a1eee9-f20b-7ac3-8332-d6e22a675ce5")
	reservationExpiry := now.Add(15 * time.Minute)

	row := &repository.ListRow{
		Status:             "PENDING_PAYMENT",
		LockExpiresAt:      reservationExpiry,
		PaymentID:          &paymentID,
		PaymentStatus:      &paymentStatus,
		PaymentExpiresAtDB: &paymentExpiry,
	}
	timer := svc.timer(row)
	if timer == nil || timer.Kind != "PAYMENT" || timer.RemainingSeconds != 86400 {
		t.Fatalf("payment timer = %+v", timer)
	}

	row.PaymentID = nil
	row.PaymentStatus = nil
	row.PaymentExpiresAtDB = nil
	timer = svc.timer(row)
	if timer == nil || timer.Kind != "LOCK" || timer.RemainingSeconds != 900 {
		t.Fatalf("lock timer = %+v", timer)
	}

	row.Status = "PAID"
	if timer := svc.timer(row); timer != nil {
		t.Fatalf("paid timer = %+v, want nil", timer)
	}
}
