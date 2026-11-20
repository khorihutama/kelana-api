package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/khorihutama/kelana-api/internal/api/middleware"
	"github.com/khorihutama/kelana-api/internal/common/dto"
	"github.com/khorihutama/kelana-api/internal/common/errs"
	"github.com/khorihutama/kelana-api/internal/infra/ratelimit"
	authdto "github.com/khorihutama/kelana-api/internal/module/auth/model/dto"
	"github.com/khorihutama/kelana-api/internal/module/auth/service"
)

type Handler struct {
	service    *service.Service
	limiter    *ratelimit.Limiter
	cookieName string
	secure     bool
}

func New(svc *service.Service, limiter *ratelimit.Limiter, cookieName string, secure bool) *Handler {
	return &Handler{service: svc, limiter: limiter, cookieName: cookieName, secure: secure}
}

func (h *Handler) RegisterPath(group *gin.RouterGroup, auth gin.HandlerFunc) {
	group.POST("/auth/register", middleware.RateLimitRegister(h.limiter), h.register)
	group.POST("/auth/verify-email", middleware.RateLimit(h.limiter, "auth-verify", 10, time.Minute), h.verifyEmail)
	group.POST("/auth/resend-verification", middleware.RateLimit(h.limiter, "auth-resend", 3, time.Hour), h.resendVerification)
	group.POST("/auth/login", middleware.RateLimit(h.limiter, "auth-login", 5, time.Minute), h.login)
	group.POST("/auth/refresh", middleware.RateLimit(h.limiter, "auth-refresh", 30, time.Minute), h.refresh)
	group.POST("/auth/logout", auth, h.logout)
	group.GET("/auth/me", auth, h.me)
}

func (h *Handler) register(c *gin.Context) {
	var req authdto.RegisterRequest
	if !bindJSON(c, &req) {
		return
	}
	result, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OK(c, http.StatusCreated, result)
}

func (h *Handler) verifyEmail(c *gin.Context) {
	var req authdto.VerifyEmailRequest
	if !bindJSON(c, &req) {
		return
	}
	result, err := h.service.VerifyEmail(c.Request.Context(), req, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		dto.Fail(c, err)
		return
	}
	h.setRefreshCookie(c, result.RefreshToken, result.RefreshExpiresAt)
	dto.OK(c, http.StatusOK, authdto.AuthResponse{
		AccessToken: result.AccessToken,
		ExpiresIn:   result.ExpiresIn,
		User:        result.User,
	})
}

func (h *Handler) resendVerification(c *gin.Context) {
	var req authdto.ResendVerificationRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx := c.Request.Context()
	ok, err := h.limiter.Reserve(ctx, "otp-resend:"+req.Email, 15*time.Second)
	if err != nil {
		dto.Fail(c, errs.Internal(err))
		return
	}
	if !ok {
		dto.Fail(c, errs.New(errs.CodeResendCooldown, http.StatusTooManyRequests, "Tunggu 15 detik sebelum meminta kode lagi"))
		return
	}
	if err := h.service.ResendVerification(ctx, req.Email); err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OK(c, http.StatusAccepted, authdto.MessageResponse{Message: "Kode OTP telah dikirim"})
}

func (h *Handler) login(c *gin.Context) {
	var req authdto.LoginRequest
	if !bindJSON(c, &req) {
		return
	}
	result, err := h.service.Login(c.Request.Context(), req, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		dto.Fail(c, err)
		return
	}
	h.setRefreshCookie(c, result.RefreshToken, result.RefreshExpiresAt)
	dto.OK(c, http.StatusOK, authdto.AuthResponse{
		AccessToken: result.AccessToken,
		ExpiresIn:   result.ExpiresIn,
		User:        result.User,
	})
}

func (h *Handler) refresh(c *gin.Context) {
	raw, err := c.Cookie(h.cookieName)
	if err != nil || raw == "" {
		dto.Fail(c, errs.New(errs.CodeRefreshInvalid, http.StatusUnauthorized, "Refresh token tidak ditemukan"))
		return
	}
	result, err := h.service.Refresh(c.Request.Context(), raw, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		dto.Fail(c, err)
		return
	}
	h.setRefreshCookie(c, result.RefreshToken, result.RefreshExpiresAt)
	dto.OK(c, http.StatusOK, authdto.AuthResponse{
		AccessToken: result.AccessToken,
		ExpiresIn:   result.ExpiresIn,
		User:        result.User,
	})
}

func (h *Handler) logout(c *gin.Context) {
	sessionID := c.GetString(middleware.ContextSessionID)
	if err := h.service.Logout(c.Request.Context(), sessionID); err != nil {
		dto.Fail(c, err)
		return
	}
	h.clearRefreshCookie(c)
	c.Status(http.StatusNoContent)
}

func (h *Handler) me(c *gin.Context) {
	user, err := h.service.Me(c.Request.Context(), c.GetString(middleware.ContextUserID))
	if err != nil {
		dto.Fail(c, err)
		return
	}
	dto.OK(c, http.StatusOK, user)
}

func (h *Handler) setRefreshCookie(c *gin.Context, value string, expires time.Time) {
	maxAge := int(time.Until(expires).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cookieName,
		Value:    value,
		Path:     "/api/v1/auth",
		MaxAge:   maxAge,
		Expires:  expires,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cookieName,
		Value:    "",
		Path:     "/api/v1/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		dto.Fail(c, errs.Validation([]errs.FieldError{{Field: "body", Message: err.Error()}}))
		return false
	}
	return true
}
