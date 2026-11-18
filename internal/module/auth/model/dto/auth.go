package dto

import "time"

type RegisterRequest struct {
	Email    string  `json:"email" binding:"required,email"`
	Password string  `json:"password" binding:"required,min=8,max=72"`
	Name     string  `json:"name" binding:"required,min=2,max=100"`
	Phone    *string `json:"phone" binding:"omitempty,max=20"`
}

type VerifyEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

type ResendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type UserResponse struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	Name            string     `json:"name"`
	Phone           *string    `json:"phone,omitempty"`
	Role            string     `json:"role"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
}

type RegisterResponse struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type AuthResponse struct {
	AccessToken string       `json:"access_token"`
	ExpiresIn   int          `json:"expires_in"`
	User        UserResponse `json:"user"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type LoginResult struct {
	AccessToken      string
	ExpiresIn        int
	User             UserResponse
	RefreshToken     string
	RefreshExpiresAt time.Time
}
