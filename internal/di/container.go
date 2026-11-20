package di

import (
	"log/slog"
	"path/filepath"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/khorihutama/kelana-api/internal/config"
	"github.com/khorihutama/kelana-api/internal/infra/mailer"
	paymentinfra "github.com/khorihutama/kelana-api/internal/infra/payment"
	"github.com/khorihutama/kelana-api/internal/infra/ratelimit"
	redisinfra "github.com/khorihutama/kelana-api/internal/infra/redis"
	"github.com/khorihutama/kelana-api/internal/infra/storage"
	authHandler "github.com/khorihutama/kelana-api/internal/module/auth/handler"
	authRepository "github.com/khorihutama/kelana-api/internal/module/auth/repository"
	authService "github.com/khorihutama/kelana-api/internal/module/auth/service"
	authToken "github.com/khorihutama/kelana-api/internal/module/auth/token"
	bookingHandler "github.com/khorihutama/kelana-api/internal/module/booking/handler"
	bookingRepository "github.com/khorihutama/kelana-api/internal/module/booking/repository"
	bookingService "github.com/khorihutama/kelana-api/internal/module/booking/service"
	documentHandler "github.com/khorihutama/kelana-api/internal/module/document/handler"
	documentRepository "github.com/khorihutama/kelana-api/internal/module/document/repository"
	documentService "github.com/khorihutama/kelana-api/internal/module/document/service"
	healthHandler "github.com/khorihutama/kelana-api/internal/module/health/handler"
	healthService "github.com/khorihutama/kelana-api/internal/module/health/service"
	mytripHandler "github.com/khorihutama/kelana-api/internal/module/mytrip/handler"
	mytripRepository "github.com/khorihutama/kelana-api/internal/module/mytrip/repository"
	mytripService "github.com/khorihutama/kelana-api/internal/module/mytrip/service"
	paymentHandler "github.com/khorihutama/kelana-api/internal/module/payment/handler"
	paymentPort "github.com/khorihutama/kelana-api/internal/module/payment/port"
	paymentRepository "github.com/khorihutama/kelana-api/internal/module/payment/repository"
	paymentService "github.com/khorihutama/kelana-api/internal/module/payment/service"
	tripHandler "github.com/khorihutama/kelana-api/internal/module/trip/handler"
	tripRepository "github.com/khorihutama/kelana-api/internal/module/trip/repository"
	tripService "github.com/khorihutama/kelana-api/internal/module/trip/service"
)

type Container struct {
	Auth       *authHandler.Handler
	Trip       *tripHandler.Handler
	Booking    *bookingHandler.Handler
	Document   *documentHandler.Handler
	Payment    *paymentHandler.Handler
	MyTrip     *mytripHandler.Handler
	Health     *healthHandler.Handler
	Tokens     *authToken.Manager
	BookingSvc *bookingService.Service
	PaymentSvc *paymentService.Service
}

func Build(cfg *config.Config, logger *slog.Logger, db *gorm.DB, rdb *goredis.Client) (*Container, error) {
	limiter := ratelimit.New(rdb)
	mockMailer := mailer.NewMock(logger)
	tokens := authToken.New(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.JWTIssuer, cfg.JWTAudience)

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
		CodePrefix:        cfg.BrandCodePrefix,
	}, logger)
	bookingH := bookingHandler.New(bookingSvc, limiter)

	docStore, err := storage.NewLocal(filepath.Join(cfg.UploadsDir, "documents"), cfg.DocumentEncKey)
	if err != nil {
		return nil, err
	}
	docRepo := documentRepository.New(db)
	docSvc := documentService.New(docRepo, docStore, cfg.DocumentEncKey, documentService.Config{
		URLTTL: cfg.DocumentURLTTL,
	}, logger)
	docH := documentHandler.New(docSvc, limiter)

	paymentRepo := paymentRepository.New(db)
	paymentSvc := paymentService.New(paymentRepo, seatStore, docSvc,
		[]paymentPort.Provider{paymentinfra.NewMock(cfg.MockWebhookSecret, cfg.BrandMerchantName)},
		paymentService.Config{
			Provider:       cfg.PaymentProvider,
			PaymentTTL:     cfg.PaymentTTL,
			ExpiryInterval: cfg.PaymentExpiryInterval,
			BrandName:      cfg.BrandName,
		},
		logger,
	)
	paymentH := paymentHandler.New(paymentSvc, limiter)

	mytripRepo := mytripRepository.New(db)
	mytripSvc := mytripService.New(mytripRepo, bookingSvc, mytripService.Config{WAGroupURL: cfg.WAGroupURL}, logger)
	mytripH := mytripHandler.New(mytripSvc)

	return &Container{
		Auth: authH, Trip: tripH, Booking: bookingH, Document: docH, Payment: paymentH,
		MyTrip: mytripH, Health: healthH,
		Tokens: tokens, BookingSvc: bookingSvc, PaymentSvc: paymentSvc,
	}, nil
}
