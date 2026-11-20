package payment

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"testing"

	"github.com/khorihutama/kelana-api/internal/module/payment/port"
)

func TestMidtransSignatureAndMapping(t *testing.T) {
	adapter := NewMidtrans("server-key")
	raw := "order-1" + "200" + "5900000.00" + "server-key"
	sum := sha512.Sum512([]byte(raw))
	want := hex.EncodeToString(sum[:])
	if got := adapter.Signature("order-1", "200", "5900000.00"); got != want {
		t.Fatalf("Signature = %s, want %s", got, want)
	}

	cases := map[string]port.Status{
		"settlement": port.StatusPaid,
		"capture":    port.StatusPaid,
		"expire":     port.StatusExpired,
		"deny":       port.StatusFailed,
		"cancel":     port.StatusFailed,
		"failure":    port.StatusFailed,
		"pending":    port.StatusPending,
	}
	for rawStatus, wantStatus := range cases {
		if got := adapter.MapStatus(rawStatus); got != wantStatus {
			t.Fatalf("MapStatus(%q) = %s, want %s", rawStatus, got, wantStatus)
		}
	}
}

func TestMidtransAdapterNotActive(t *testing.T) {
	adapter := NewMidtrans("server-key")
	if _, err := adapter.CreateCharge(context.Background(), port.ChargeRequest{}); err != port.ErrNotSupported {
		t.Fatalf("CreateCharge err = %v", err)
	}
	if _, err := adapter.ParseWebhook(context.Background(), nil, nil); err != port.ErrNotSupported {
		t.Fatalf("ParseWebhook err = %v", err)
	}
	if err := adapter.Refund(context.Background(), port.RefundRequest{}); err != port.ErrNotSupported {
		t.Fatalf("Refund err = %v", err)
	}
}
