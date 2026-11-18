package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/hsi-go-api/internal/common/dto"
	"github.com/khorihutama/hsi-go-api/internal/module/trip/model/query"
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
	group.GET("/trips/:id", h.detail)
	group.GET("/trips/:id/schedules", h.schedules)
	group.GET("/destinations", h.destinations)
	group.GET("/categories", h.categories)
}

func (h *Handler) list(c *gin.Context) {
	filter, appErr := query.Parse(c.Request.URL.Query())
	if appErr != nil {
		dto.Fail(c, appErr)
		return
	}
	cards, meta, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OKPage(c, cards, meta)
}

func (h *Handler) detail(c *gin.Context) {
	detail, err := h.service.Detail(c.Request.Context(), c.Param("id"))
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OK(c, http.StatusOK, detail)
}

func (h *Handler) schedules(c *gin.Context) {
	schedules, err := h.service.Schedules(c.Request.Context(), c.Param("id"))
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OK(c, http.StatusOK, schedules)
}

func (h *Handler) destinations(c *gin.Context) {
	rows, err := h.service.Destinations(c.Request.Context())
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OK(c, http.StatusOK, rows)
}

func (h *Handler) categories(c *gin.Context) {
	rows, err := h.service.Categories(c.Request.Context())
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OK(c, http.StatusOK, rows)
}
