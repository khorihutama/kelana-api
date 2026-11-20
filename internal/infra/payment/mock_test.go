package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/khorihutama/kelana-api/internal/module/payment/port"
)

func signedHeader(secret string, body []byte) http.Header {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	headers := http.Header{}
	headers.Set(SignatureHeader, hex.EncodeToString(mac.Sum(nil)))
	return headers
}

func TestMockCreateCharge(t *testing.T) {
	provider := NewMock("test-secret-123456", "KELANA")
	ctx := context.Background()
	req := port.ChargeRequest{
		PaymentID:     uuid.Must(uuid.NewV7()),
		ReservationID: uuid.Must(uuid.NewV7()),
		AmountIDR:     5900000,
		Method:        port.MethodVABCA,
	}

	charge, err := provider.CreateCharge(ctx, req)
	if err != nil {
		t.Fatalf("CreateCharge: %v", err)
	}
	if charge.ProviderRef != "MOCK-"+req.ReservationID.String() {
		t.Fatalf("provider_ref = %s", charge.ProviderRef)
	}
	if charge.Status != port.StatusPending {
		t.Fatalf("status = %s", charge.Status)
	}
	if len(charge.VANumber) != 16 || !strings.HasPrefix(charge.VANumber, "8808") {
		t.Fatalf("va_number = %q", charge.VANumber)
	}

	req.Method = port.MethodQRIS
	qr, err := provider.CreateCharge(ctx, req)
	if err != nil || qr.QRString == "" {
		t.Fatalf("QRIS charge = %+v, %v", qr, err)
	}
	if !strings.Contains(qr.QRString, "5906KELANA") {
		t.Fatalf("QRIS merchant is not KELANA: %q", qr.QRString)
	}
	req.Method = port.MethodCard
	card, err := provider.CreateCharge(ctx, req)
	if err != nil || card.VANumber != "" || card.QRString != "" {
		t.Fatalf("CARD charge = %+v, %v", card, err)
	}
}

func TestMockParseWebhookSignature(t *testing.T) {
	secret := "test-secret-123456"
	provider := NewMock(secret, "KELANA")
	paymentID := uuid.Must(uuid.NewV7())
	body := []byte(`{"event_id":"evt-1","payment_id":"` + paymentID.String() + `","status":"PAID","amount_idr":5900000}`)

	event, err := provider.ParseWebhook(context.Background(), signedHeader(secret, body), body)
	if err != nil {
		t.Fatalf("ParseWebhook: %v", err)
	}
	if event.ProviderEventID != "evt-1" || event.PaymentID == nil || *event.PaymentID != paymentID || event.Status != port.StatusPaid {
		t.Fatalf("event = %+v", event)
	}

	if _, err := provider.ParseWebhook(context.Background(), http.Header{}, body); err != port.ErrInvalidSignature {
		t.Fatalf("missing signature err = %v", err)
	}
	badHeader := signedHeader(secret, []byte("other"))
	if _, err := provider.ParseWebhook(context.Background(), badHeader, body); err != port.ErrInvalidSignature {
		t.Fatalf("wrong signature err = %v", err)
	}
	goodHeader := signedHeader(secret, body)
	if _, err := provider.ParseWebhook(context.Background(), goodHeader, append(body, ' ')); err != port.ErrInvalidSignature {
		t.Fatalf("tampered body err = %v", err)
	}

	invalid := []byte(`{"event_id":"evt-2","payment_id":"` + paymentID.String() + `","status":"FAILED"}`)
	if _, err := provider.ParseWebhook(context.Background(), signedHeader(secret, invalid), invalid); err != port.ErrInvalidEvent {
		t.Fatalf("failed status err = %v", err)
	}
}

func TestMockRefundNotSupported(t *testing.T) {
	provider := NewMock("test-secret-123456", "KELANA")
	if err := provider.Refund(context.Background(), port.RefundRequest{}); err != port.ErrNotSupported {
		t.Fatalf("Refund err = %v", err)
	}
}

func TestMockMapStatus(t *testing.T) {
	provider := NewMock("x", "KELANA")
	cases := map[string]port.Status{
		"paid":       port.StatusPaid,
		"settlement": port.StatusPaid,
		"expired":    port.StatusExpired,
		"deny":       port.StatusFailed,
		"pending":    port.StatusPending,
	}
	for raw, want := range cases {
		if got := provider.MapStatus(raw); got != want {
			t.Fatalf("MapStatus(%q) = %s, want %s", raw, got, want)
		}
	}
}
