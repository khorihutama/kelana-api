package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/hsi-go-api/internal/common/dto"
	"github.com/khorihutama/hsi-go-api/internal/common/errs"
	"github.com/khorihutama/hsi-go-api/internal/infra/ratelimit"
)

const (
	registerIPLimit     = 30
	registerIPWindow    = time.Hour
	registerEmailLimit  = 5
	registerEmailWindow = 15 * time.Minute
)

// RateLimitRegister limits registration by IP and by normalized email + IP.
// The tighter email bucket keeps one address from exhausting a shared NAT,
// while the wider IP bucket still blocks bulk abuse.
func RateLimitRegister(limiter *ratelimit.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ipResult, err := limiter.Allow(ctx, "auth-register:ip:"+c.ClientIP(), registerIPLimit, registerIPWindow)
		if err != nil {
			dto.Fail(c, errs.Internal(err))
			return
		}

		emailResult := ratelimit.Result{
			Allowed:    true,
			Limit:      registerEmailLimit,
			Remaining:  registerEmailLimit,
			RetryAfter: registerEmailWindow,
		}
		emailKey := registerEmailKey(c)
		if emailKey != "" {
			emailResult, err = limiter.Allow(ctx, emailKey, registerEmailLimit, registerEmailWindow)
			if err != nil {
				dto.Fail(c, errs.Internal(err))
				return
			}
		}

		result := ipResult
		if emailKey != "" {
			result = emailResult
		}
		if !ipResult.Allowed {
			result = ipResult
			if !emailResult.Allowed && emailResult.RetryAfter > ipResult.RetryAfter {
				result.RetryAfter = emailResult.RetryAfter
			}
		}

		setRateLimitHeaders(c, result)
		if !ipResult.Allowed || !emailResult.Allowed {
			dto.Fail(c, errs.TooManyRequests("Terlalu banyak percobaan. Coba lagi nanti."))
			return
		}
		c.Next()
	}
}

// RateLimit limits requests per client IP for a named route.
func RateLimit(limiter *ratelimit.Limiter, name string, limit int, window time.Duration) gin.HandlerFunc {
	return rateLimit(limiter, name, limit, window, func(c *gin.Context) string {
		return "ip:" + c.ClientIP()
	})
}

// RateLimitUser limits requests per authenticated user, falling back to IP when
// the auth middleware did not set a user.
func RateLimitUser(limiter *ratelimit.Limiter, name string, limit int, window time.Duration) gin.HandlerFunc {
	return rateLimit(limiter, name, limit, window, func(c *gin.Context) string {
		if userID := c.GetString(ContextUserID); userID != "" {
			return "user:" + userID
		}
		return "ip:" + c.ClientIP()
	})
}

func rateLimit(limiter *ratelimit.Limiter, name string, limit int, window time.Duration, subject func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := limiter.Allow(c.Request.Context(), name+":"+subject(c), limit, window)
		if err != nil {
			dto.Fail(c, errs.Internal(err))
			return
		}
		setRateLimitHeaders(c, result)
		if !result.Allowed {
			dto.Fail(c, errs.TooManyRequests("Terlalu banyak percobaan. Coba lagi nanti."))
			return
		}
		c.Next()
	}
}

func setRateLimitHeaders(c *gin.Context, result ratelimit.Result) {
	c.Header("RateLimit-Limit", strconv.Itoa(result.Limit))
	c.Header("RateLimit-Remaining", strconv.Itoa(result.Remaining))
	reset := resetSeconds(result)
	c.Header("RateLimit-Reset", strconv.Itoa(reset))
	if !result.Allowed {
		c.Header("Retry-After", strconv.Itoa(reset))
	}
}

func resetSeconds(result ratelimit.Result) int {
	reset := int(math.Ceil(result.RetryAfter.Seconds()))
	if reset < 1 {
		reset = 1
	}
	return reset
}

// registerEmailKey reads the JSON body without consuming it and hashes the
// normalized email together with the client IP.
func registerEmailKey(c *gin.Context) string {
	raw, err := c.GetRawData()
	if err != nil || len(raw) == 0 {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))

	var payload struct {
		Email string `json:"email"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return ""
	}
	email := strings.ToLower(strings.TrimSpace(payload.Email))
	if email == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(email + "|" + c.ClientIP()))
	return "auth-register:email:" + hex.EncodeToString(sum[:])
}
