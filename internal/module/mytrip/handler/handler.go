package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/hsi-go-api/internal/api/middleware"
	"github.com/khorihutama/hsi-go-api/internal/common/dto"
	"github.com/khorihutama/hsi-go-api/internal/common/errs"
	"github.com/khorihutama/hsi-go-api/internal/common/util"
	"github.com/khorihutama/hsi-go-api/internal/module/mytrip/service"
)

type Handler struct {
	service *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{service: svc}
}

func (h *Handler) RegisterPath(group *gin.RouterGroup, auth gin.HandlerFunc) {
	group.GET("/mytrips", auth, h.list)
	group.GET("/mytrips/:id", auth, h.get)
	group.POST("/mytrips/:id/cancel", auth, h.cancel)
}

func (h *Handler) list(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		dto.Fail(c, errs.Validation([]errs.FieldError{{Field: "page", Message: "page harus angka"}}))
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil {
		dto.Fail(c, errs.Validation([]errs.FieldError{{Field: "limit", Message: "limit harus angka"}}))
		return
	}
	items, total, page, limit, err := h.service.List(c.Request.Context(),
		c.GetString(middleware.ContextUserID), c.Query("tab"), page, limit)
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OKPage(c, items, dto.Meta{
		Page: page, Limit: limit, TotalItems: total, TotalPages: util.TotalPages(total, limit),
	})
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
