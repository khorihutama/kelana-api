package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/hsi-go-api/internal/common/dto"
	"github.com/khorihutama/hsi-go-api/internal/common/errs"
	"github.com/khorihutama/hsi-go-api/internal/infra/ratelimit"
)

// RateLimit limits requests per client IP for a named route.
func RateLimit(limiter *ratelimit.Limiter, name string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ok, err := limiter.Allow(c.Request.Context(), name+":"+c.ClientIP(), limit, window)
		if err != nil {
			dto.Fail(c, errs.Internal(err))
			return
		}
		if !ok {
			dto.Fail(c, errs.TooManyRequests("Terlalu banyak percobaan. Coba lagi nanti."))
			return
		}
		c.Next()
	}
}
