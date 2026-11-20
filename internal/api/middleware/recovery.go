package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/kelana-api/internal/common/dto"
	"github.com/khorihutama/kelana-api/internal/common/errs"
)

func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic recovered",
					"request_id", dto.RequestID(c),
					"error", r,
					"stack", string(debug.Stack()),
				)
				dto.Fail(c, errs.New(errs.CodeInternal, http.StatusInternalServerError, "Terjadi kesalahan pada server"))
			}
		}()
		c.Next()
	}
}
