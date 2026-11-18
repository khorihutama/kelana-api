package middleware

import (
	"math"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/hsi-go-api/internal/common/dto"
	"github.com/khorihutama/hsi-go-api/internal/common/errs"
	"github.com/khorihutama/hsi-go-api/internal/infra/ratelimit"
)

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
		c.Header("RateLimit-Limit", strconv.Itoa(result.Limit))
		c.Header("RateLimit-Remaining", strconv.Itoa(result.Remaining))
		reset := int(math.Ceil(result.RetryAfter.Seconds()))
		if reset < 1 {
			reset = 1
		}
		c.Header("RateLimit-Reset", strconv.Itoa(reset))
		if !result.Allowed {
			c.Header("Retry-After", strconv.Itoa(reset))
			dto.Fail(c, errs.TooManyRequests("Terlalu banyak percobaan. Coba lagi nanti."))
			return
		}
		c.Next()
	}
}
