package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/kelana-api/internal/common/dto"
	"github.com/khorihutama/kelana-api/internal/module/health/service"
)

type Handler struct {
	service *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{service: svc}
}

func (h *Handler) RegisterPath(group *gin.RouterGroup) {
	group.GET("/healthz", h.healthz)
	group.GET("/readyz", h.readyz)
}

func (h *Handler) healthz(c *gin.Context) {
	dto.OK(c, http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) readyz(c *gin.Context) {
	ready, checks := h.service.Ready(c.Request.Context())
	status := http.StatusOK
	state := "ok"
	if !ready {
		status = http.StatusServiceUnavailable
		state = "degraded"
	}
	c.JSON(status, dto.SuccessResponse{Data: gin.H{"status": state, "checks": checks}})
}
