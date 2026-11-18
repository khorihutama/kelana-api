package ratelimit

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Limiter struct {
	client *goredis.Client
}

func New(client *goredis.Client) *Limiter {
	return &Limiter{client: client}
}

// Allow increments the counter for key and reports whether the request is within limit.
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	pipe := l.client.TxPipeline()
	incr := pipe.Incr(ctx, "rl:"+key)
	pipe.ExpireNX(ctx, "rl:"+key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	return incr.Val() <= int64(limit), nil
}

// Reserve sets a cooldown key if it does not exist and reports whether it was acquired.
func (l *Limiter) Reserve(ctx context.Context, key string, window time.Duration) (bool, error) {
	return l.client.SetNX(ctx, "cd:"+key, 1, window).Result()
}
