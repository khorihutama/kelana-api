package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/hsi-go-api/internal/common/dto"
	"github.com/khorihutama/hsi-go-api/internal/common/errs"
)

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Header("X-Permitted-Cross-Domain-Policies", "none")
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// BodyLimit rejects oversized declared bodies and caps the reader for chunked
// bodies. Handlers still enforce their own tighter multipart limits.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes > 0 {
			if c.Request.ContentLength > maxBytes {
				dto.Fail(c, errs.New(errs.CodeRequestTooLarge, http.StatusRequestEntityTooLarge,
					"Ukuran request terlalu besar (maks "+strconv.FormatInt(maxBytes, 10)+" byte)"))
				return
			}
			if c.Request.Body != nil {
				c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
			}
		}
		c.Next()
	}
}
