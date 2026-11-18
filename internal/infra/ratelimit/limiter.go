package ratelimit

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const allowScript = `
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
local ttl = redis.call('PTTL', KEYS[1])
return {count, ttl}
`

var allow = goredis.NewScript(allowScript)

type Result struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Duration
}

type Limiter struct {
	client *goredis.Client
}

func New(client *goredis.Client) *Limiter {
	return &Limiter{client: client}
}

// Allow increments the Redis counter for key and reports whether the request
// is within limit. The counter is shared by every instance using the same Redis.
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (Result, error) {
	if limit <= 0 {
		return Result{Allowed: true, Limit: limit}, nil
	}
	if window <= 0 {
		window = time.Minute
	}
	raw, err := allow.Run(ctx, l.client, []string{"rl:" + key}, window.Milliseconds()).Slice()
	if err != nil {
		return Result{}, err
	}
	count, ok := raw[0].(int64)
	if !ok {
		return Result{}, fmt.Errorf("rate limit: unexpected count type %T", raw[0])
	}
	ttlMillis, _ := raw[1].(int64)
	retryAfter := time.Duration(ttlMillis) * time.Millisecond
	if retryAfter < 0 {
		retryAfter = window
	}
	remaining := limit - int(count)
	if remaining < 0 {
		remaining = 0
	}
	return Result{
		Allowed:    count <= int64(limit),
		Limit:      limit,
		Remaining:  remaining,
		RetryAfter: retryAfter,
	}, nil
}

// Reserve sets a cooldown key if it does not exist and reports whether it was acquired.
func (l *Limiter) Reserve(ctx context.Context, key string, window time.Duration) (bool, error) {
	return l.client.SetNX(ctx, "cd:"+key, 1, window).Result()
}
