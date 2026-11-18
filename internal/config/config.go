package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv            string
	RestPort          string
	DBHost            string
	DBPort            string
	DBName            string
	DBUser            string
	DBPass            string
	DBSchema          string
	RedisAddr         string
	JWTSecret         string
	RefreshCookieName string
	CORSOrigins       []string
	MailDriver        string
	PaymentProvider   string
	StorageDriver     string
	UploadsDir        string
	DocumentEncKey    []byte
	SeedEnabled       bool

	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	OTPExpiry         time.Duration
	SeatLockTTL       time.Duration
	SeatHoldSafetyTTL time.Duration
	SeatSweepInterval time.Duration
	DocumentURLTTL    time.Duration
}

func Load() (*Config, error) {
	appEnv := getenv("APP_ENV", "development")
	if !strings.EqualFold(appEnv, "production") {
		_ = godotenv.Load()
	}

	cfg := &Config{
		AppEnv:            appEnv,
		RestPort:          getenv("REST_PORT", "8080"),
		DBHost:            getenv("DB_HOST", "localhost"),
		DBPort:            getenv("DB_PORT", "5432"),
		DBName:            getenv("DB_DATABASE", "hsi_go_api"),
		DBUser:            getenv("DB_USERNAME", "hsi"),
		DBPass:            getenv("DB_PASSWORD", "hsi"),
		DBSchema:          getenv("DB_SCHEMA", "public"),
		RedisAddr:         getenv("REDIS_ADDR", "localhost:6379"),
		JWTSecret:         os.Getenv("JWT_SECRET"),
		RefreshCookieName: getenv("REFRESH_COOKIE_NAME", "hsi_refresh"),
		CORSOrigins:       splitCSV(getenv("CORS_ORIGINS", "http://localhost:3000,http://localhost:5173")),
		MailDriver:        getenv("MAIL_DRIVER", "mock"),
		PaymentProvider:   getenv("PAYMENT_PROVIDER", "mock"),
		StorageDriver:     getenv("STORAGE_DRIVER", "local"),
		UploadsDir:        getenv("UPLOADS_DIR", "./uploads"),
		SeedEnabled:       strings.EqualFold(getenv("SEED_ENABLED", "false"), "true"),
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   30 * 24 * time.Hour,
		OTPExpiry:         10 * time.Minute,
		SeatLockTTL:       time.Duration(envInt("SEAT_LOCK_TTL_SECONDS", 900)) * time.Second,
		SeatHoldSafetyTTL: time.Duration(envInt("SEAT_HOLD_SAFETY_TTL_SECONDS", 1800)) * time.Second,
		SeatSweepInterval: time.Duration(envInt("SEAT_SWEEP_INTERVAL_SECONDS", 30)) * time.Second,
		DocumentURLTTL:    time.Duration(envInt("DOCUMENT_URL_TTL_SECONDS", 300)) * time.Second,
	}

	encKey, err := decodeKey(getenvAny("DOCUMENT_ENC_KEY", "DOC_ENCRYPTION_KEY"))
	if err != nil {
		return nil, err
	}
	cfg.DocumentEncKey = encKey

	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if cfg.RefreshCookieName == "" {
		return nil, fmt.Errorf("REFRESH_COOKIE_NAME is required")
	}
	if cfg.SeatLockTTL <= 0 || cfg.SeatSweepInterval <= 0 {
		return nil, fmt.Errorf("SEAT_LOCK_TTL_SECONDS and SEAT_SWEEP_INTERVAL_SECONDS must be positive")
	}
	if cfg.SeatHoldSafetyTTL <= cfg.SeatLockTTL {
		return nil, fmt.Errorf("SEAT_HOLD_SAFETY_TTL_SECONDS must be greater than SEAT_LOCK_TTL_SECONDS")
	}
	if cfg.DocumentURLTTL <= 0 {
		return nil, fmt.Errorf("DOCUMENT_URL_TTL_SECONDS must be positive")
	}
	if !strings.EqualFold(cfg.StorageDriver, "local") {
		return nil, fmt.Errorf("STORAGE_DRIVER %q is not implemented; use local", cfg.StorageDriver)
	}
	return cfg, nil
}

func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable search_path=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPass, c.DBName, c.DBSchema,
	)
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getenvAny(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

func decodeKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, fmt.Errorf("DOCUMENT_ENC_KEY is required (base64-encoded 32 bytes)")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("DOCUMENT_ENC_KEY must be valid base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("DOCUMENT_ENC_KEY must decode to 32 bytes, got %d", len(key))
	}
	return key, nil
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}
