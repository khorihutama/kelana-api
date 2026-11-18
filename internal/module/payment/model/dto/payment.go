package dto

type CreateRequest struct {
	Method string `json:"method" binding:"required,oneof=VA_BCA VA_BSI VA_MANDIRI QRIS CARD"`
}

type Response struct {
	ID            string  `json:"id"`
	ReservationID string  `json:"reservation_id"`
	Provider      string  `json:"provider"`
	Method        string  `json:"method"`
	Status        string  `json:"status"`
	Currency      string  `json:"currency"`
	AmountIDR     int64   `json:"amount_idr"`
	VANumber      *string `json:"va_number,omitempty"`
	QRString      *string `json:"qr_string,omitempty"`
	ExpiresAt     string  `json:"expires_at"`
	PaidAt        *string `json:"paid_at,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

type WebhookResponse struct {
	Received bool `json:"received"`
}
