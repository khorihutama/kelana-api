package service

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/khorihutama/kelana-api/internal/common/errs"
	"github.com/khorihutama/kelana-api/internal/module/auth/model/dto"
	"github.com/khorihutama/kelana-api/internal/module/auth/model/entity"
	"github.com/khorihutama/kelana-api/internal/module/auth/token"
)

type fakeRepo struct {
	users         map[string]*entity.User
	byID          map[uuid.UUID]*entity.User
	verifications []*entity.EmailVerification
	sessions      map[string]*entity.RefreshSession
	bySessionID   map[uuid.UUID]*entity.RefreshSession
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:       map[string]*entity.User{},
		byID:        map[uuid.UUID]*entity.User{},
		sessions:    map[string]*entity.RefreshSession{},
		bySessionID: map[uuid.UUID]*entity.RefreshSession{},
	}
}

func (f *fakeRepo) CreateUser(_ context.Context, user *entity.User) error {
	f.users[user.Email] = user
	f.byID[user.ID] = user
	return nil
}

func (f *fakeRepo) FindUserByEmail(_ context.Context, email string) (*entity.User, error) {
	return f.users[email], nil
}

func (f *fakeRepo) FindUserByID(_ context.Context, id uuid.UUID) (*entity.User, error) {
	return f.byID[id], nil
}

func (f *fakeRepo) SetEmailVerified(_ context.Context, userID uuid.UUID, at time.Time) error {
	f.byID[userID].EmailVerifiedAt = &at
	return nil
}

func (f *fakeRepo) CreateVerification(_ context.Context, v *entity.EmailVerification) error {
	for _, old := range f.verifications {
		if old.UserID == v.UserID && old.ConsumedAt == nil {
			now := time.Now().UTC()
			old.ConsumedAt = &now
		}
	}
	f.verifications = append(f.verifications, v)
	return nil
}

func (f *fakeRepo) FindActiveVerification(_ context.Context, userID uuid.UUID) (*entity.EmailVerification, error) {
	for i := len(f.verifications) - 1; i >= 0; i-- {
		v := f.verifications[i]
		if v.UserID == userID && v.ConsumedAt == nil {
			return v, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) UpdateVerificationAttempts(_ context.Context, id uuid.UUID, attempts int) error {
	for _, v := range f.verifications {
		if v.ID == id {
			v.Attempts = attempts
		}
	}
	return nil
}

func (f *fakeRepo) ConsumeVerification(_ context.Context, id uuid.UUID, at time.Time) error {
	for _, v := range f.verifications {
		if v.ID == id {
			v.ConsumedAt = &at
		}
	}
	return nil
}

func (f *fakeRepo) CreateSession(_ context.Context, s *entity.RefreshSession) error {
	f.sessions[s.TokenHash] = s
	f.bySessionID[s.ID] = s
	return nil
}

func (f *fakeRepo) FindSessionByHash(_ context.Context, hash string) (*entity.RefreshSession, error) {
	return f.sessions[hash], nil
}

func (f *fakeRepo) RevokeSession(_ context.Context, id uuid.UUID, at time.Time) error {
	if s := f.bySessionID[id]; s != nil {
		s.RevokedAt = &at
	}
	return nil
}

func (f *fakeRepo) RevokeSessionReplaced(_ context.Context, id uuid.UUID, at time.Time, replacedBy uuid.UUID) error {
	if s := f.bySessionID[id]; s != nil {
		s.RevokedAt = &at
		s.ReplacedBy = &replacedBy
	}
	return nil
}

func (f *fakeRepo) RevokeFamily(_ context.Context, familyID uuid.UUID, at time.Time) error {
	for _, s := range f.sessions {
		if s.FamilyID == familyID && s.RevokedAt == nil {
			s.RevokedAt = &at
		}
	}
	return nil
}

type captureMailer struct {
	lastCode string
	lastTo   string
}

func (m *captureMailer) SendVerificationCode(_ context.Context, email, code string) error {
	m.lastTo = email
	m.lastCode = code
	return nil
}

func newTestService() (*Service, *fakeRepo, *captureMailer) {
	repo := newFakeRepo()
	mail := &captureMailer{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := New(repo, token.New("01234567890123456789012345678901", 15*time.Minute, "kelana", "kelana-api"), mail, Config{
		JWTSecret:       "01234567890123456789012345678901",
		RefreshTokenTTL: 30 * 24 * time.Hour,
		OTPExpiry:       10 * time.Minute,
	}, logger)
	return svc, repo, mail
}

func registerAndVerify(t *testing.T) (*Service, *fakeRepo, *captureMailer, *dto.LoginResult) {
	t.Helper()
	svc, repo, mail := newTestService()
	ctx := context.Background()

	if _, err := svc.Register(ctx, dto.RegisterRequest{
		Email: "new@example.com", Password: "password123", Name: "New User",
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if mail.lastCode == "" || len(mail.lastCode) != 6 {
		t.Fatalf("expected 6-digit OTP, got %q", mail.lastCode)
	}

	if _, err := svc.Login(ctx, dto.LoginRequest{Email: "new@example.com", Password: "password123"}, "ua", "127.0.0.1"); err == nil {
		t.Fatal("expected login before verification to fail")
	} else if appErr, ok := errs.As(err); !ok || appErr.Code != errs.CodeEmailNotVerified {
		t.Fatalf("expected EMAIL_NOT_VERIFIED, got %v", err)
	}

	result, err := svc.VerifyEmail(ctx, dto.VerifyEmailRequest{Email: "new@example.com", Code: mail.lastCode}, "ua", "127.0.0.1")
	if err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	if result.AccessToken == "" || result.RefreshToken == "" {
		t.Fatal("expected access and refresh token after verification")
	}
	return svc, repo, mail, result
}

func TestRegisterVerifyAndLogin(t *testing.T) {
	svc, repo, _, verified := registerAndVerify(t)
	ctx := context.Background()

	if repo.users["new@example.com"].EmailVerifiedAt == nil {
		t.Fatal("expected email to be marked verified")
	}
	login, err := svc.Login(ctx, dto.LoginRequest{Email: "new@example.com", Password: "password123"}, "ua", "127.0.0.1")
	if err != nil {
		t.Fatalf("Login after verify: %v", err)
	}
	if login.User.Email != "new@example.com" || login.AccessToken == "" {
		t.Fatalf("unexpected login result: %+v", login.User)
	}
	if verified.User.EmailVerifiedAt == nil {
		t.Fatal("expected verified user response")
	}
}

func TestRefreshRotatesAndDetectsReuse(t *testing.T) {
	svc, _, _, first := registerAndVerify(t)
	ctx := context.Background()

	second, err := svc.Refresh(ctx, first.RefreshToken, "ua", "127.0.0.1")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("expected refresh token rotation")
	}

	if _, err := svc.Refresh(ctx, first.RefreshToken, "ua", "127.0.0.1"); err == nil {
		t.Fatal("expected reused refresh token to be rejected")
	} else if appErr, ok := errs.As(err); !ok || appErr.Code != errs.CodeRefreshReuse {
		t.Fatalf("expected REFRESH_REUSE_DETECTED, got %v", err)
	}

	if _, err := svc.Refresh(ctx, second.RefreshToken, "ua", "127.0.0.1"); err == nil {
		t.Fatal("expected family to be revoked after reuse")
	}
}

func TestVerifyEmailRejectsWrongCode(t *testing.T) {
	svc, _, mail, _ := registerAndVerify(t)
	ctx := context.Background()

	// Register again to obtain a fresh unconsumed code.
	if _, err := svc.Register(ctx, dto.RegisterRequest{
		Email: "other@example.com", Password: "password123", Name: "Other User",
	}); err != nil {
		t.Fatal(err)
	}
	bad := "000000"
	if mail.lastCode == bad {
		bad = "111111"
	}
	if _, err := svc.VerifyEmail(ctx, dto.VerifyEmailRequest{Email: "other@example.com", Code: bad}, "ua", "127.0.0.1"); err == nil {
		t.Fatal("expected wrong OTP to be rejected")
	}
}
