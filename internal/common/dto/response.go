package dto

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/hsi-go-api/internal/common/errs"
)

type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

type SuccessResponse struct {
	Data any   `json:"data"`
	Meta *Meta `json:"meta,omitempty"`
}

type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details []errs.FieldError `json:"details,omitempty"`
}

type ErrorResponse struct {
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"request_id"`
}

func OK(c *gin.Context, status int, data any) {
	c.JSON(status, SuccessResponse{Data: data})
}

func OKPage(c *gin.Context, data any, meta Meta) {
	c.JSON(http.StatusOK, SuccessResponse{Data: data, Meta: &meta})
}

func RequestID(c *gin.Context) string {
	if v, ok := c.Get("request_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func Fail(c *gin.Context, err error) {
	appErr, ok := errs.As(err)
	if !ok {
		appErr = errs.Internal(err)
	}
	c.Set("error_code", appErr.Code)
	c.AbortWithStatusJSON(appErr.Status, ErrorResponse{
		Error: ErrorBody{
			Code:    appErr.Code,
			Message: appErr.Message,
			Details: appErr.Details,
		},
		RequestID: RequestID(c),
	})
}
