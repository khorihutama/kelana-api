package handler

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/kelana-api/internal/api/middleware"
	"github.com/khorihutama/kelana-api/internal/common/dto"
	"github.com/khorihutama/kelana-api/internal/common/errs"
	"github.com/khorihutama/kelana-api/internal/infra/ratelimit"
	paymentdto "github.com/khorihutama/kelana-api/internal/module/payment/model/dto"
	"github.com/khorihutama/kelana-api/internal/module/payment/service"
)

const maxWebhookBytes int64 = 1 << 20

type Handler struct {
	service *service.Service
	limiter *ratelimit.Limiter
}

func New(svc *service.Service, limiter *ratelimit.Limiter) *Handler {
	return &Handler{service: svc, limiter: limiter}
}

func (h *Handler) RegisterPath(group *gin.RouterGroup, auth gin.HandlerFunc) {
	group.POST("/reservations/:id/payments", auth, middleware.RateLimitUser(h.limiter, "payment:create", 10, time.Minute), h.create)
	group.GET("/payments/:id", auth, h.get)
	group.POST("/payments/webhook/:provider", middleware.RateLimit(h.limiter, "payment:webhook", 120, time.Minute), h.webhook)
}

func (h *Handler) create(c *gin.Context) {
	var req paymentdto.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Fail(c, errs.Validation([]errs.FieldError{{Field: "body", Message: err.Error()}}))
		return
	}
	result, created, err := h.service.Create(c.Request.Context(),
		c.GetString(middleware.ContextUserID), c.Param("id"), req.Method)
	if err != nil {
		dto.Fail(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	dto.OK(c, status, result)
}

func (h *Handler) get(c *gin.Context) {
	result, err := h.service.Get(c.Request.Context(), c.GetString(middleware.ContextUserID), c.Param("id"))
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OK(c, http.StatusOK, result)
}

func (h *Handler) webhook(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBytes)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			dto.Fail(c, errs.New(errs.CodeRequestTooLarge, http.StatusRequestEntityTooLarge, "Payload webhook terlalu besar"))
			return
		}
		dto.Fail(c, errs.Internal(err))
		return
	}
	if err := h.service.HandleWebhook(c.Request.Context(), c.Param("provider"), c.Request.Header, raw); err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OK(c, http.StatusOK, paymentdto.WebhookResponse{Received: true})
}
