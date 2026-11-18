package di

import (
	"log/slog"
	"path/filepath"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/khorihutama/hsi-go-api/internal/config"
	"github.com/khorihutama/hsi-go-api/internal/infra/mailer"
	"github.com/khorihutama/hsi-go-api/internal/infra/ratelimit"
	redisinfra "github.com/khorihutama/hsi-go-api/internal/infra/redis"
	"github.com/khorihutama/hsi-go-api/internal/infra/storage"
	authHandler "github.com/khorihutama/hsi-go-api/internal/module/auth/handler"
	authRepository "github.com/khorihutama/hsi-go-api/internal/module/auth/repository"
	authService "github.com/khorihutama/hsi-go-api/internal/module/auth/service"
	authToken "github.com/khorihutama/hsi-go-api/internal/module/auth/token"
	bookingHandler "github.com/khorihutama/hsi-go-api/internal/module/booking/handler"
	bookingRepository "github.com/khorihutama/hsi-go-api/internal/module/booking/repository"
	bookingService "github.com/khorihutama/hsi-go-api/internal/module/booking/service"
	documentHandler "github.com/khorihutama/hsi-go-api/internal/module/document/handler"
	documentRepository "github.com/khorihutama/hsi-go-api/internal/module/document/repository"
	documentService "github.com/khorihutama/hsi-go-api/internal/module/document/service"
	healthHandler "github.com/khorihutama/hsi-go-api/internal/module/health/handler"
	healthService "github.com/khorihutama/hsi-go-api/internal/module/health/service"
	tripHandler "github.com/khorihutama/hsi-go-api/internal/module/trip/handler"
	tripRepository "github.com/khorihutama/hsi-go-api/internal/module/trip/repository"
	tripService "github.com/khorihutama/hsi-go-api/internal/module/trip/service"
)

type Container struct {
	Auth       *authHandler.Handler
	Trip       *tripHandler.Handler
	Booking    *bookingHandler.Handler
	Document   *documentHandler.Handler
	Health     *healthHandler.Handler
	Tokens     *authToken.Manager
	BookingSvc *bookingService.Service
}

func Build(cfg *config.Config, logger *slog.Logger, db *gorm.DB, rdb *goredis.Client) (*Container, error) {
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

	seatStore := redisinfra.NewSeatHoldStore(rdb)
	bookingRepo := bookingRepository.New(db)
	bookingSvc := bookingService.New(bookingRepo, seatStore, bookingService.Config{
		SeatLockTTL:       cfg.SeatLockTTL,
		SeatHoldSafetyTTL: cfg.SeatHoldSafetyTTL,
		SeatSweepInterval: cfg.SeatSweepInterval,
	}, logger)
	bookingH := bookingHandler.New(bookingSvc)

	docStore, err := storage.NewLocal(filepath.Join(cfg.UploadsDir, "documents"), cfg.DocumentEncKey)
	if err != nil {
		return nil, err
	}
	docRepo := documentRepository.New(db)
	docSvc := documentService.New(docRepo, docStore, cfg.DocumentEncKey, documentService.Config{
		URLTTL: cfg.DocumentURLTTL,
	}, logger)
	docH := documentHandler.New(docSvc, limiter)

	return &Container{
		Auth: authH, Trip: tripH, Booking: bookingH, Document: docH, Health: healthH,
		Tokens: tokens, BookingSvc: bookingSvc,
	}, nil
}
