package config

import (
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
	SeedEnabled       bool

	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	OTPExpiry         time.Duration
	SeatLockTTL       time.Duration
	SeatSweepInterval time.Duration
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
		SeatSweepInterval: time.Duration(envInt("SEAT_SWEEP_INTERVAL_SECONDS", 30)) * time.Second,
	}

	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if cfg.RefreshCookieName == "" {
		return nil, fmt.Errorf("REFRESH_COOKIE_NAME is required")
	}
	if cfg.SeatLockTTL <= 0 || cfg.SeatSweepInterval <= 0 {
		return nil, fmt.Errorf("SEAT_LOCK_TTL_SECONDS and SEAT_SWEEP_INTERVAL_SECONDS must be positive")
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
