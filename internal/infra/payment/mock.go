package payment

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/khorihutama/hsi-go-api/internal/module/payment/port"
)

const SignatureHeader = "X-Mock-Signature"

type Mock struct {
	secret []byte
	mu     sync.Mutex
	seq    uint64
}

func NewMock(secret string) *Mock {
	return &Mock{secret: []byte(secret)}
}

func (m *Mock) Name() string { return "mock" }

func (m *Mock) CreateCharge(ctx context.Context, req port.ChargeRequest) (port.Charge, error) {
	if req.AmountIDR <= 0 {
		return port.Charge{}, fmt.Errorf("amount must be positive")
	}
	charge := port.Charge{
		ProviderRef: "MOCK-" + req.ReservationID.String(),
		Status:      port.StatusPending,
	}
	switch req.Method {
	case port.MethodVABCA, port.MethodVABSI, port.MethodVAMandiri:
		charge.VANumber = m.vaNumber()
	case port.MethodQRIS:
		charge.QRString = fmt.Sprintf("00020101021226%04dID.MOCK.%s5204000053033605802ID5910HSI%%20GO6007JAKARTA6304%s",
			len(req.PaymentID.String()), req.PaymentID.String(), m.checksum(req.PaymentID.String()))
	case port.MethodCard:
		// Mock does not emulate a card processor; the payment can still be settled
		// through a signed mock webhook.
	default:
		return port.Charge{}, fmt.Errorf("unsupported method %q", req.Method)
	}
	return charge, nil
}

func (m *Mock) ParseWebhook(ctx context.Context, headers http.Header, body []byte) (port.WebhookEvent, error) {
	signature := strings.TrimSpace(headers.Get(SignatureHeader))
	signature = strings.TrimPrefix(signature, "sha256=")
	expected := hmac.New(sha256.New, m.secret)
	expected.Write(body)
	if signature == "" || !hmac.Equal([]byte(signature), []byte(hex.EncodeToString(expected.Sum(nil)))) {
		return port.WebhookEvent{}, port.ErrInvalidSignature
	}

	var payload struct {
		EventID   string     `json:"event_id"`
		PaymentID string     `json:"payment_id"`
		Status    string     `json:"status"`
		AmountIDR int64      `json:"amount_idr"`
		PaidAt    *time.Time `json:"paid_at"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return port.WebhookEvent{}, port.ErrInvalidEvent
	}
	if strings.TrimSpace(payload.EventID) == "" || strings.TrimSpace(payload.PaymentID) == "" {
		return port.WebhookEvent{}, port.ErrInvalidEvent
	}
	paymentID, err := uuid.Parse(payload.PaymentID)
	if err != nil {
		return port.WebhookEvent{}, port.ErrInvalidEvent
	}
	status := m.MapStatus(payload.Status)
	if status != port.StatusPaid {
		return port.WebhookEvent{}, port.ErrInvalidEvent
	}
	return port.WebhookEvent{
		ProviderEventID: payload.EventID,
		PaymentID:       &paymentID,
		Type:            string(status),
		Status:          status,
		AmountIDR:       payload.AmountIDR,
		PaidAt:          payload.PaidAt,
		Raw:             append(json.RawMessage(nil), body...),
	}, nil
}

func (m *Mock) MapStatus(raw string) port.Status {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "PAID", "SETTLEMENT", "CAPTURE":
		return port.StatusPaid
	case "EXPIRED":
		return port.StatusExpired
	case "FAILED", "DENY", "CANCEL":
		return port.StatusFailed
	default:
		return port.StatusPending
	}
}

func (m *Mock) Refund(ctx context.Context, req port.RefundRequest) error {
	return port.ErrNotSupported
}

func (m *Mock) vaNumber() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	var random [4]byte
	_, _ = rand.Read(random[:])
	value := (uint64(binary.BigEndian.Uint32(random[:])) + m.seq) % 1_000_000_000_000
	return fmt.Sprintf("8808%012d", value)
}

func (m *Mock) checksum(value string) string {
	sum := sha256.Sum256([]byte(value))
	return strings.ToUpper(hex.EncodeToString(sum[:4]))
}
