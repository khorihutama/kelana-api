package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/kelana-api/internal/api/middleware"
	"github.com/khorihutama/kelana-api/internal/common/dto"
	"github.com/khorihutama/kelana-api/internal/common/errs"
	"github.com/khorihutama/kelana-api/internal/infra/ratelimit"
	"github.com/khorihutama/kelana-api/internal/module/document/service"
)

const (
	maxDocumentBytes  int64 = 3 * 1024 * 1024
	maxMultipartBytes       = maxDocumentBytes + 1024*1024
)

type Handler struct {
	service *service.Service
	limiter *ratelimit.Limiter
}

func New(svc *service.Service, limiter *ratelimit.Limiter) *Handler {
	return &Handler{service: svc, limiter: limiter}
}

func (h *Handler) RegisterPath(group *gin.RouterGroup, auth gin.HandlerFunc) {
	group.POST("/documents", auth, middleware.RateLimitUser(h.limiter, "document:upload", 20, time.Minute), h.upload)
	group.GET("/documents/:id/url", auth, h.signedURL)
	group.GET("/documents/:id/content", h.content)
	group.DELETE("/documents/:id", auth, h.delete)
	group.GET("/reservations/:id/documents/completeness", auth, h.completeness)
}

func (h *Handler) upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMultipartBytes)
	if err := c.Request.ParseMultipartForm(maxMultipartBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) || strings.Contains(err.Error(), "request body too large") {
			dto.Fail(c, errs.New(errs.CodeFileTooLarge, http.StatusRequestEntityTooLarge, "Ukuran dokumen maksimal 3 MB"))
			return
		}
		dto.Fail(c, errs.Validation([]errs.FieldError{{Field: "body", Message: "multipart tidak valid"}}))
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		dto.Fail(c, errs.Validation([]errs.FieldError{{Field: "file", Message: "file wajib diunggah"}}))
		return
	}
	defer file.Close()

	if header.Size > maxDocumentBytes {
		dto.Fail(c, errs.New(errs.CodeFileTooLarge, http.StatusRequestEntityTooLarge, "Ukuran dokumen maksimal 3 MB"))
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxDocumentBytes+1))
	if err != nil {
		dto.Fail(c, errs.Internal(err))
		return
	}

	result, err := h.service.Upload(c.Request.Context(),
		c.GetString(middleware.ContextUserID),
		c.PostForm("participant_id"),
		c.PostForm("type"),
		data,
	)
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OK(c, http.StatusCreated, result)
}

func (h *Handler) signedURL(c *gin.Context) {
	result, err := h.service.SignedURL(c.Request.Context(), c.GetString(middleware.ContextUserID), c.Param("id"))
	if err != nil {
		dto.Fail(c, err)
		return
	}
	result.DownloadURL = absoluteURL(c, result.DownloadURL)
	dto.OK(c, http.StatusOK, result)
}

func (h *Handler) content(c *gin.Context) {
	exp, err := strconv.ParseInt(c.Query("exp"), 10, 64)
	if err != nil {
		dto.Fail(c, errs.New(errs.CodeSignatureInvalid, http.StatusForbidden, "Tanda tangan URL tidak valid"))
		return
	}
	doc, data, err := h.service.Download(c.Request.Context(), c.Param("id"), exp, c.Query("sig"))
	if err != nil {
		dto.Fail(c, err)
		return
	}
	c.Header("Content-Type", doc.MimeType)
	c.Header("Content-Length", strconv.FormatInt(int64(len(data)), 10))
	c.Header("Content-Disposition", service.ContentDisposition(doc))
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, doc.MimeType, data)
}

func (h *Handler) delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.GetString(middleware.ContextUserID), c.Param("id")); err != nil {
		dto.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) completeness(c *gin.Context) {
	result, err := h.service.Completeness(c.Request.Context(), c.GetString(middleware.ContextUserID), c.Param("id"))
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OK(c, http.StatusOK, result)
}

func absoluteURL(c *gin.Context, path string) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); forwarded != "" {
		scheme = forwarded
	}
	return scheme + "://" + c.Request.Host + path
}
