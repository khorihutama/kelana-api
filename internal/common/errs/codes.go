package errs

const (
	CodeValidation       = "VALIDATION_ERROR"
	CodeUnauthorized     = "UNAUTHORIZED"
	CodeForbidden        = "FORBIDDEN"
	CodeNotFound         = "NOT_FOUND"
	CodeInternal         = "INTERNAL_ERROR"
	CodeRateLimited      = "RATE_LIMITED"
	CodeEmailTaken       = "EMAIL_TAKEN"
	CodeInvalidCreds     = "INVALID_CREDENTIALS"
	CodeEmailNotVerified = "EMAIL_NOT_VERIFIED"
	CodeOTPInvalid       = "OTP_INVALID"
	CodeOTPExpired       = "OTP_EXPIRED"
	CodeOTPAttempts      = "OTP_ATTEMPTS_EXCEEDED"
	CodeResendCooldown   = "RESEND_COOLDOWN"
	CodeRefreshInvalid   = "REFRESH_INVALID"
	CodeRefreshReuse     = "REFRESH_REUSE_DETECTED"
	CodeUserNotFound     = "USER_NOT_FOUND"
)
