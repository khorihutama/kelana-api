package service

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Service struct {
	db  *gorm.DB
	rdb *goredis.Client
}

func New(db *gorm.DB, rdb *goredis.Client) *Service {
	return &Service{db: db, rdb: rdb}
}

func (s *Service) Ready(ctx context.Context) (bool, map[string]string) {
	checks := map[string]string{"postgres": "ok", "redis": "ok"}
	ready := true

	sqlDB, err := s.db.DB()
	if err != nil {
		checks["postgres"] = "error"
		ready = false
	} else {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := sqlDB.PingContext(pingCtx); err != nil {
			checks["postgres"] = "error"
			ready = false
		}
	}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.rdb.Ping(pingCtx).Err(); err != nil {
		checks["redis"] = "error"
		ready = false
	}
	return ready, checks
}
