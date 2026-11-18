package mailer

import (
	"context"
	"log/slog"
)

type Mailer interface {
	SendVerificationCode(ctx context.Context, email, code string) error
}

type MockMailer struct {
	logger *slog.Logger
}

func NewMock(logger *slog.Logger) *MockMailer {
	return &MockMailer{logger: logger}
}

func (m *MockMailer) SendVerificationCode(ctx context.Context, email, code string) error {
	m.logger.Info("mock verification email",
		"email", email,
		"otp_code", code,
		"driver", "mock",
	)
	return nil
}
