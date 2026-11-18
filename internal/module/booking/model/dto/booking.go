package dto

type ParticipantInput struct {
	FullName string `json:"full_name" binding:"required,min=2,max=100"`
	Gender   string `json:"gender" binding:"required,oneof=MALE FEMALE"`
	Phone    string `json:"phone" binding:"required,min=6,max=20"`
	Email    string `json:"email" binding:"required,email"`
}

type LockRequest struct {
	ScheduleID   string             `json:"schedule_id" binding:"required,uuid"`
	Participants []ParticipantInput `json:"participants" binding:"required,min=1,max=10,dive"`
	HotelID      *string            `json:"hotel_id" binding:"omitempty,uuid"`
	PickupPoint  string             `json:"pickup_point" binding:"required,min=3,max=200"`
	Notes        *string            `json:"notes" binding:"omitempty,max=500"`
	PromoCode    *string            `json:"promo_code" binding:"omitempty,max=50"`
}

type ParticipantResponse struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
	Gender   string `json:"gender"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
}

type ReservationResponse struct {
	ID             string                `json:"id"`
	Code           string                `json:"code"`
	Status         string                `json:"status"`
	TripID         string                `json:"trip_id"`
	ScheduleID     string                `json:"schedule_id"`
	Participants   []ParticipantResponse `json:"participants"`
	UnitPriceIDR   int64                 `json:"unit_price_idr"`
	TotalAmountIDR int64                 `json:"total_amount_idr"`
	DiscountIDR    int64                 `json:"discount_idr"`
	Currency       string                `json:"currency"`
	PickupPoint    string                `json:"pickup_point"`
	Notes          *string               `json:"notes,omitempty"`
	HotelID        *string               `json:"hotel_id,omitempty"`
	PromoCodeID    *string               `json:"promo_code_id,omitempty"`
	LockExpiresAt  string                `json:"lock_expires_at"`
	SeatsRemaining int64                 `json:"seats_remaining,omitempty"`
}

type LockResponse struct {
	ID             string `json:"id"`
	Code           string `json:"code"`
	Status         string `json:"status"`
	Currency       string `json:"currency"`
	LockExpiresAt  string `json:"lock_expires_at"`
	TotalAmountIDR int64  `json:"total_amount_idr"`
	DiscountIDR    int64  `json:"discount_idr"`
	SeatsRemaining int64  `json:"seats_remaining"`
}

type PromoValidateRequest struct {
	Code        string `json:"code" binding:"required,min=3,max=50"`
	SubtotalIDR int64  `json:"subtotal_idr" binding:"required,gt=0"`
}

type PromoValidateResponse struct {
	Valid         bool   `json:"valid"`
	Code          string `json:"code"`
	DiscountIDR   int64  `json:"discount_idr"`
	FinalTotalIDR int64  `json:"final_total_idr"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
