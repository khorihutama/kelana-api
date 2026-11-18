package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/hsi-go-api/internal/common/dto"
	"github.com/khorihutama/hsi-go-api/internal/common/errs"
	"github.com/khorihutama/hsi-go-api/internal/module/trip/service"
)

type Handler struct {
	service *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{service: svc}
}

func (h *Handler) RegisterPath(group *gin.RouterGroup) {
	group.GET("/trips", h.list)
}

func (h *Handler) list(c *gin.Context) {
	page, err := parsePositiveInt(c.Query("page"), 1)
	if err != nil {
		dto.Fail(c, errs.Validation([]errs.FieldError{{Field: "page", Message: "page harus bilangan bulat >= 1"}}))
		return
	}
	limit, err := parsePositiveInt(c.Query("limit"), 10)
	if err != nil {
		dto.Fail(c, errs.Validation([]errs.FieldError{{Field: "limit", Message: "limit harus bilangan bulat >= 1"}}))
		return
	}

	cards, meta, err := h.service.List(c.Request.Context(), page, limit)
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OKPage(c, cards, meta)
}

func parsePositiveInt(raw string, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, errs.ErrInvalidQuery
	}
	return value, nil
}
