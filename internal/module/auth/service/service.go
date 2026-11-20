package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/khorihutama/kelana-api/internal/common/errs"
	"github.com/khorihutama/kelana-api/internal/infra/mailer"
	"github.com/khorihutama/kelana-api/internal/module/auth/model/dto"
	"github.com/khorihutama/kelana-api/internal/module/auth/model/entity"
	"github.com/khorihutama/kelana-api/internal/module/auth/repository"
	"github.com/khorihutama/kelana-api/internal/module/auth/token"
)

type Config struct {
	JWTSecret       string
	RefreshTokenTTL time.Duration
	OTPExpiry       time.Duration
}

type Service struct {
	repo   repository.Repository
	tokens *token.Manager
	mailer mailer.Mailer
	cfg    Config
	logger *slog.Logger
	now    func() time.Time
	random io.Reader
}

func New(repo repository.Repository, tokens *token.Manager, mail mailer.Mailer, cfg Config, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		tokens: tokens,
		mailer: mail,
		cfg:    cfg,
		logger: logger,
		now:    func() time.Time { return time.Now().UTC() },
		random: rand.Reader,
	}
}

func (s *Service) Register(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	existing, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if existing != nil {
		return nil, errs.Conflict(errs.CodeEmailTaken, "Email sudah terdaftar")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, errs.Internal(err)
	}

	user := &entity.User{
		ID:           uuid.Must(uuid.NewV7()),
		Email:        email,
		PasswordHash: string(hash),
		Name:         strings.TrimSpace(req.Name),
		Phone:        req.Phone,
		Role:         "CUSTOMER",
		Status:       "ACTIVE",
	}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, errs.Internal(err)
	}
	if err := s.issueVerification(ctx, user); err != nil {
		return nil, err
	}

	return &dto.RegisterResponse{
		ID:     user.ID.String(),
		Email:  user.Email,
		Name:   user.Name,
		Status: "PENDING_VERIFICATION",
	}, nil
}

func (s *Service) ResendVerification(ctx context.Context, email string) error {
	user, err := s.repo.FindUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return errs.Internal(err)
	}
	if user == nil {
		return errs.NotFound(errs.CodeUserNotFound, "Akun tidak ditemukan")
	}
	if user.EmailVerifiedAt != nil {
		return errs.Conflict("EMAIL_ALREADY_VERIFIED", "Email sudah terverifikasi")
	}
	return s.issueVerification(ctx, user)
}

func (s *Service) VerifyEmail(ctx context.Context, req dto.VerifyEmailRequest, userAgent, ip string) (*dto.LoginResult, error) {
	user, err := s.repo.FindUserByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		return nil, errs.Internal(err)
	}
	if user == nil {
		return nil, errs.NotFound(errs.CodeUserNotFound, "Akun tidak ditemukan")
	}

	verification, err := s.repo.FindActiveVerification(ctx, user.ID)
	if err != nil {
		return nil, errs.Internal(err)
	}
	now := s.now()
	if verification == nil || verification.ExpiresAt.Before(now) {
		return nil, errs.New(errs.CodeOTPExpired, 400, "Kode OTP sudah kedaluwarsa")
	}
	if verification.Attempts >= 5 {
		return nil, errs.New(errs.CodeOTPAttempts, 400, "Terlalu banyak percobaan OTP")
	}
	if err := s.repo.UpdateVerificationAttempts(ctx, verification.ID, verification.Attempts+1); err != nil {
		return nil, errs.Internal(err)
	}
	if !hmac.Equal([]byte(verification.CodeHash), []byte(s.hashCode(req.Code))) {
		return nil, errs.New(errs.CodeOTPInvalid, 400, "Kode OTP salah")
	}
	if err := s.repo.ConsumeVerification(ctx, verification.ID, now); err != nil {
		return nil, errs.Internal(err)
	}
	if err := s.repo.SetEmailVerified(ctx, user.ID, now); err != nil {
		return nil, errs.Internal(err)
	}
	user.EmailVerifiedAt = &now

	result, _, err := s.issueSession(ctx, user, userAgent, ip, nil)
	return result, err
}

func (s *Service) Login(ctx context.Context, req dto.LoginRequest, userAgent, ip string) (*dto.LoginResult, error) {
	user, err := s.repo.FindUserByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		return nil, errs.Internal(err)
	}
	if user == nil {
		return nil, errs.New(errs.CodeInvalidCreds, 401, "Email atau kata sandi salah")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return nil, errs.New(errs.CodeInvalidCreds, 401, "Email atau kata sandi salah")
	}
	if user.EmailVerifiedAt == nil {
		return nil, errs.Forbidden(errs.CodeEmailNotVerified, "Email belum terverifikasi")
	}
	if user.Status == "BLOCKED" {
		return nil, errs.Forbidden(errs.CodeForbidden, "Akun diblokir")
	}
	result, _, err := s.issueSession(ctx, user, userAgent, ip, nil)
	return result, err
}

func (s *Service) Refresh(ctx context.Context, rawToken, userAgent, ip string) (*dto.LoginResult, error) {
	hash := s.hashToken(rawToken)
	session, err := s.repo.FindSessionByHash(ctx, hash)
	if err != nil {
		return nil, errs.Internal(err)
	}
	now := s.now()
	if session == nil {
		return nil, errs.New(errs.CodeRefreshInvalid, 401, "Refresh token tidak valid")
	}
	if session.RevokedAt != nil {
		_ = s.repo.RevokeFamily(ctx, session.FamilyID, now)
		return nil, errs.New(errs.CodeRefreshReuse, 401, "Refresh token sudah dipakai; sesi dicabut")
	}
	if session.ExpiresAt.Before(now) {
		return nil, errs.New(errs.CodeRefreshInvalid, 401, "Refresh token kedaluwarsa")
	}

	user, err := s.repo.FindUserByID(ctx, session.UserID)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if user == nil {
		return nil, errs.New(errs.CodeRefreshInvalid, 401, "Refresh token tidak valid")
	}

	result, newSessionID, err := s.issueSession(ctx, user, userAgent, ip, &session.FamilyID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.RevokeSessionReplaced(ctx, session.ID, now, newSessionID); err != nil {
		return nil, errs.Internal(err)
	}
	return result, nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	id, err := uuid.Parse(sessionID)
	if err != nil {
		return errs.Unauthorized("Sesi tidak valid")
	}
	return s.repo.RevokeSession(ctx, id, s.now())
}

func (s *Service) Me(ctx context.Context, userID string) (*dto.UserResponse, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, errs.Unauthorized("Sesi tidak valid")
	}
	user, err := s.repo.FindUserByID(ctx, id)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if user == nil {
		return nil, errs.NotFound(errs.CodeUserNotFound, "Akun tidak ditemukan")
	}
	out := toUserResponse(user)
	return &out, nil
}

func (s *Service) issueVerification(ctx context.Context, user *entity.User) error {
	code, err := randomCode(s.random)
	if err != nil {
		return errs.Internal(err)
	}
	now := s.now()
	verification := &entity.EmailVerification{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    user.ID,
		CodeHash:  s.hashCode(code),
		ExpiresAt: now.Add(s.cfg.OTPExpiry),
	}
	if err := s.repo.CreateVerification(ctx, verification); err != nil {
		return errs.Internal(err)
	}
	if err := s.mailer.SendVerificationCode(ctx, user.Email, code); err != nil {
		return errs.Internal(err)
	}
	return nil
}

func (s *Service) issueSession(ctx context.Context, user *entity.User, userAgent, ip string, familyID *uuid.UUID) (*dto.LoginResult, uuid.UUID, error) {
	now := s.now()
	sessionID := uuid.Must(uuid.NewV7())
	family := uuid.Must(uuid.NewV7())
	if familyID != nil {
		family = *familyID
	}
	refreshToken, err := randomToken(s.random)
	if err != nil {
		return nil, uuid.Nil, errs.Internal(err)
	}

	session := &entity.RefreshSession{
		ID:        sessionID,
		UserID:    user.ID,
		TokenHash: s.hashToken(refreshToken),
		FamilyID:  family,
		UserAgent: stringPtr(userAgent),
		IP:        stringPtr(ip),
		ExpiresAt: now.Add(s.cfg.RefreshTokenTTL),
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, uuid.Nil, errs.Internal(err)
	}

	accessToken, expiresIn, err := s.tokens.Generate(user.ID, user.Role, sessionID)
	if err != nil {
		return nil, uuid.Nil, errs.Internal(err)
	}

	return &dto.LoginResult{
		AccessToken:      accessToken,
		ExpiresIn:        expiresIn,
		User:             toUserResponse(user),
		RefreshToken:     refreshToken,
		RefreshExpiresAt: session.ExpiresAt,
	}, sessionID, nil
}

func (s *Service) hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *Service) hashCode(code string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.JWTSecret))
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func randomCode(r io.Reader) (string, error) {
	n, err := rand.Int(r, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func randomToken(r io.Reader) (string, error) {
	buf := make([]byte, 32)
	if _, err := r.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func toUserResponse(user *entity.User) dto.UserResponse {
	return dto.UserResponse{
		ID:              user.ID.String(),
		Email:           user.Email,
		Name:            user.Name,
		Phone:           user.Phone,
		Role:            user.Role,
		EmailVerifiedAt: user.EmailVerifiedAt,
	}
}

func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
