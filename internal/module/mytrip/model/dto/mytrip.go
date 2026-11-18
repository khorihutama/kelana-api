package dto

type TripSummary struct {
	ID              string `json:"id"`
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	Category        string `json:"category"`
	DepartureCity   string `json:"departure_city"`
	DestinationCity string `json:"destination_city"`
}

type ScheduleSummary struct {
	ID        string `json:"id"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type PaymentSummary struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"`
	Method    string  `json:"method"`
	Currency  string  `json:"currency"`
	AmountIDR int64   `json:"amount_idr"`
	VANumber  *string `json:"va_number,omitempty"`
	QRString  *string `json:"qr_string,omitempty"`
	ExpiresAt string  `json:"expires_at"`
	PaidAt    *string `json:"paid_at,omitempty"`
}

type Timer struct {
	Kind             string `json:"kind"`
	ExpiresAt        string `json:"expires_at"`
	RemainingSeconds int64  `json:"remaining_seconds"`
}

type ListItem struct {
	ID                string          `json:"id"`
	Code              string          `json:"code"`
	Status            string          `json:"status"`
	Tab               string          `json:"tab"`
	Trip              TripSummary     `json:"trip"`
	Schedule          ScheduleSummary `json:"schedule"`
	ParticipantsCount int             `json:"participants_count"`
	TotalAmountIDR    int64           `json:"total_amount_idr"`
	Currency          string          `json:"currency"`
	Payment           *PaymentSummary `json:"payment,omitempty"`
	JoinWAURL         *string         `json:"join_wa_url,omitempty"`
	Timer             *Timer          `json:"timer,omitempty"`
	CreatedAt         string          `json:"created_at"`
}

type Participant struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
	Gender   string `json:"gender"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

type Document struct {
	ID            string `json:"id"`
	ParticipantID string `json:"participant_id"`
	Type          string `json:"type"`
	MimeType      string `json:"mime_type"`
	SizeBytes     int64  `json:"size_bytes"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

type Detail struct {
	ID                string          `json:"id"`
	Code              string          `json:"code"`
	Status            string          `json:"status"`
	Tab               string          `json:"tab"`
	Trip              TripSummary     `json:"trip"`
	Schedule          ScheduleSummary `json:"schedule"`
	Participants      []Participant   `json:"participants"`
	Documents         []Document      `json:"documents"`
	Payment           *PaymentSummary `json:"payment,omitempty"`
	ParticipantsCount int             `json:"participants_count"`
	TotalAmountIDR    int64           `json:"total_amount_idr"`
	DiscountIDR       int64           `json:"discount_idr"`
	Currency          string          `json:"currency"`
	PickupPoint       string          `json:"pickup_point"`
	Notes             *string         `json:"notes,omitempty"`
	LockExpiresAt     string          `json:"lock_expires_at"`
	PaymentExpiresAt  *string         `json:"payment_expires_at,omitempty"`
	PaidAt            *string         `json:"paid_at,omitempty"`
	JoinWAURL         *string         `json:"join_wa_url,omitempty"`
	Timer             *Timer          `json:"timer,omitempty"`
	CreatedAt         string          `json:"created_at"`
}
