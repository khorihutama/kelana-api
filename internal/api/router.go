package api

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/hsi-go-api/internal/api/middleware"
	"github.com/khorihutama/hsi-go-api/internal/config"
	"github.com/khorihutama/hsi-go-api/internal/di"
)

func NewRouter(cfg *config.Config, logger *slog.Logger, container *di.Container) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(middleware.RequestID())
	router.Use(middleware.Recovery(logger))
	router.Use(middleware.CORS(cfg.CORSOrigins))
	router.Use(requestLogger(logger))

	container.Health.RegisterPath(router.Group(""))

	v1 := router.Group("/api/v1")
	container.Health.RegisterPath(v1)
	container.Auth.RegisterPath(v1, middleware.Auth(container.Tokens))
	container.Trip.RegisterPath(v1)

	return router
}

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("http request",
			"request_id", c.GetString("request_id"),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}
