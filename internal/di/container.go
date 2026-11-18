package di

import (
	"log/slog"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/khorihutama/hsi-go-api/internal/config"
	"github.com/khorihutama/hsi-go-api/internal/infra/mailer"
	"github.com/khorihutama/hsi-go-api/internal/infra/ratelimit"
	authHandler "github.com/khorihutama/hsi-go-api/internal/module/auth/handler"
	authRepository "github.com/khorihutama/hsi-go-api/internal/module/auth/repository"
	authService "github.com/khorihutama/hsi-go-api/internal/module/auth/service"
	authToken "github.com/khorihutama/hsi-go-api/internal/module/auth/token"
	healthHandler "github.com/khorihutama/hsi-go-api/internal/module/health/handler"
	healthService "github.com/khorihutama/hsi-go-api/internal/module/health/service"
	tripHandler "github.com/khorihutama/hsi-go-api/internal/module/trip/handler"
	tripRepository "github.com/khorihutama/hsi-go-api/internal/module/trip/repository"
	tripService "github.com/khorihutama/hsi-go-api/internal/module/trip/service"
)

type Container struct {
	Auth   *authHandler.Handler
	Trip   *tripHandler.Handler
	Health *healthHandler.Handler
	Tokens *authToken.Manager
}

func Build(cfg *config.Config, logger *slog.Logger, db *gorm.DB, rdb *goredis.Client) *Container {
	limiter := ratelimit.New(rdb)
	mockMailer := mailer.NewMock(logger)
	tokens := authToken.New(cfg.JWTSecret, cfg.AccessTokenTTL)

	authRepo := authRepository.New(db)
	authSvc := authService.New(authRepo, tokens, mockMailer, authService.Config{
		JWTSecret:       cfg.JWTSecret,
		RefreshTokenTTL: cfg.RefreshTokenTTL,
		OTPExpiry:       cfg.OTPExpiry,
	}, logger)
	authH := authHandler.New(authSvc, limiter, cfg.RefreshCookieName, cfg.IsProduction())

	tripRepo := tripRepository.New(db)
	tripSvc := tripService.New(tripRepo)
	tripH := tripHandler.New(tripSvc)

	healthSvc := healthService.New(db, rdb)
	healthH := healthHandler.New(healthSvc)

	return &Container{Auth: authH, Trip: tripH, Health: healthH, Tokens: tokens}
}
