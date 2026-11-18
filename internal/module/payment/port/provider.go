package port

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotSupported     = errors.New("payment provider: operation not supported")
	ErrInvalidSignature = errors.New("payment provider: invalid signature")
	ErrInvalidEvent     = errors.New("payment provider: invalid event")
)

type Method string

const (
	MethodVABCA     Method = "VA_BCA"
	MethodVABSI     Method = "VA_BSI"
	MethodVAMandiri Method = "VA_MANDIRI"
	MethodQRIS      Method = "QRIS"
	MethodCard      Method = "CARD"
)

type Status string

const (
	StatusPending  Status = "PENDING"
	StatusPaid     Status = "PAID"
	StatusExpired  Status = "EXPIRED"
	StatusFailed   Status = "FAILED"
	StatusRefunded Status = "REFUNDED"
)

type ChargeRequest struct {
	PaymentID       uuid.UUID
	ReservationID   uuid.UUID
	ReservationCode string
	AmountIDR       int64
	Method          Method
	CustomerName    string
	CustomerEmail   string
	Description     string
}

type Charge struct {
	ProviderRef string
	Status      Status
	VANumber    string
	QRString    string
	Raw         json.RawMessage
}

type WebhookEvent struct {
	ProviderEventID string
	PaymentID       *uuid.UUID
	ProviderRef     string
	Type            string
	Status          Status
	AmountIDR       int64
	PaidAt          *time.Time
	Raw             json.RawMessage
}

type RefundRequest struct {
	ProviderRef string
	AmountIDR   int64
	Reason      string
}

type Provider interface {
	Name() string
	CreateCharge(ctx context.Context, req ChargeRequest) (Charge, error)
	ParseWebhook(ctx context.Context, headers http.Header, body []byte) (WebhookEvent, error)
	MapStatus(raw string) Status
	Refund(ctx context.Context, req RefundRequest) error
}
