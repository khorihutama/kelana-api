//go:build integration

package ratelimit

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"github.com/khorihutama/hsi-go-api/internal/config"
	redisinfra "github.com/khorihutama/hsi-go-api/internal/infra/redis"
)

func TestIntegrationLimiterSharedAcrossInstances(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	_ = godotenv.Load(filepath.Join(root, ".env"))
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	rdb, err := redisinfra.Connect(context.Background(), cfg.RedisAddr)
	if err != nil {
		t.Fatalf("redis: %v", err)
	}
	key := "test:shared:" + time.Now().UTC().Format("20060102150405.000000000")
	t.Cleanup(func() { rdb.Del(context.Background(), "rl:"+key) })

	first := New(rdb)
	second := New(rdb)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		result, err := first.Allow(ctx, key, 2, time.Minute)
		if err != nil || !result.Allowed {
			t.Fatalf("instance 1 request %d = %+v, %v", i+1, result, err)
		}
	}
	blocked, err := second.Allow(ctx, key, 2, time.Minute)
	if err != nil {
		t.Fatalf("instance 2: %v", err)
	}
	if blocked.Allowed || blocked.RetryAfter <= 0 || blocked.Remaining != 0 {
		t.Fatalf("instance 2 result = %+v, want blocked with retry", blocked)
	}
	count, err := rdb.Get(ctx, "rl:"+key).Int()
	if err != nil || count != 3 {
		t.Fatalf("redis count = %d, %v", count, err)
	}
	if ttl := rdb.TTL(ctx, "rl:"+key).Val(); ttl <= 0 {
		t.Fatalf("redis ttl = %v", ttl)
	}
}
