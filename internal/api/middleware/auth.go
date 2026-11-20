package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/kelana-api/internal/common/dto"
	"github.com/khorihutama/kelana-api/internal/common/errs"
	"github.com/khorihutama/kelana-api/internal/module/auth/token"
)

const (
	ContextUserID    = "user_id"
	ContextRole      = "role"
	ContextSessionID = "session_id"
)

func Auth(tokens *token.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			dto.Fail(c, errs.Unauthorized("Token akses tidak ditemukan"))
			return
		}
		raw := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		claims, err := tokens.Parse(raw)
		if err != nil {
			dto.Fail(c, errs.New(errs.CodeUnauthorized, http.StatusUnauthorized, "Token akses tidak valid"))
			return
		}
		c.Set(ContextUserID, claims.Subject)
		c.Set(ContextRole, claims.Role)
		c.Set(ContextSessionID, claims.SessionID)
		c.Next()
	}
}
