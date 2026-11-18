package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/hsi-go-api/internal/api/middleware"
	"github.com/khorihutama/hsi-go-api/internal/common/dto"
	"github.com/khorihutama/hsi-go-api/internal/common/errs"
	"github.com/khorihutama/hsi-go-api/internal/infra/ratelimit"
	bookingdto "github.com/khorihutama/hsi-go-api/internal/module/booking/model/dto"
	"github.com/khorihutama/hsi-go-api/internal/module/booking/service"
)

type Handler struct {
	service *service.Service
	limiter *ratelimit.Limiter
}

func New(svc *service.Service, limiter *ratelimit.Limiter) *Handler {
	return &Handler{service: svc, limiter: limiter}
}

func (h *Handler) RegisterPath(group *gin.RouterGroup, auth gin.HandlerFunc) {
	group.POST("/reservations/lock", auth, middleware.RateLimitUser(h.limiter, "booking:lock", 10, time.Minute), h.lock)
	group.GET("/reservations/:id", auth, h.get)
	group.POST("/reservations/:id/cancel", auth, h.cancel)
	group.POST("/promos/validate", auth, h.validatePromo)
}

func (h *Handler) lock(c *gin.Context) {
	var req bookingdto.LockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Fail(c, errs.Validation([]errs.FieldError{{Field: "body", Message: err.Error()}}))
		return
	}
	result, err := h.service.Lock(c.Request.Context(), c.GetString(middleware.ContextUserID), req)
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OK(c, http.StatusCreated, result)
}

func (h *Handler) get(c *gin.Context) {
	result, err := h.service.Get(c.Request.Context(), c.GetString(middleware.ContextUserID), c.Param("id"))
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OK(c, http.StatusOK, result)
}

func (h *Handler) cancel(c *gin.Context) {
	result, err := h.service.Cancel(c.Request.Context(), c.GetString(middleware.ContextUserID), c.Param("id"))
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OK(c, http.StatusOK, result)
}

func (h *Handler) validatePromo(c *gin.Context) {
	var req bookingdto.PromoValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Fail(c, errs.Validation([]errs.FieldError{{Field: "body", Message: err.Error()}}))
		return
	}
	result, err := h.service.ValidatePromo(c.Request.Context(), req.Code, req.SubtotalIDR)
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OK(c, http.StatusOK, result)
}
