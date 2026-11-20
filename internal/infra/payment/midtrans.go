package payment

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"net/http"

	"github.com/khorihutama/kelana-api/internal/module/payment/port"
)

// Midtrans is a skeleton adapter. It is intentionally not registered in DI
// until a real Midtrans account and server key are available.
type Midtrans struct {
	ServerKey string
}

func NewMidtrans(serverKey string) *Midtrans {
	return &Midtrans{ServerKey: serverKey}
}

func (m *Midtrans) Name() string { return "midtrans" }

// Signature follows Midtrans: SHA512(order_id + status_code + gross_amount + server_key).
func (m *Midtrans) Signature(orderID, statusCode, grossAmount string) string {
	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + m.ServerKey))
	return hex.EncodeToString(sum[:])
}

// MapStatus maps Midtrans transaction_status values to the internal status.
func (m *Midtrans) MapStatus(raw string) port.Status {
	switch raw {
	case "settlement", "capture":
		return port.StatusPaid
	case "expire":
		return port.StatusExpired
	case "deny", "cancel", "failure":
		return port.StatusFailed
	default:
		return port.StatusPending
	}
}

// TODO(phase 7): implement CreateCharge against /v2/charge, ParseWebhook with
// signature verification, and settlement/expiry mapping.
func (m *Midtrans) CreateCharge(ctx context.Context, req port.ChargeRequest) (port.Charge, error) {
	return port.Charge{}, port.ErrNotSupported
}

func (m *Midtrans) ParseWebhook(ctx context.Context, headers http.Header, body []byte) (port.WebhookEvent, error) {
	return port.WebhookEvent{}, port.ErrNotSupported
}

func (m *Midtrans) Refund(ctx context.Context, req port.RefundRequest) error {
	return port.ErrNotSupported
}
