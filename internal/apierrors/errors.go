package apierrors

import "net/http"

// APIError represents an error that can be safely returned to API clients.
// It contains both user-facing information and internal error details for logging.
type APIError struct {
	// StatusCode is the HTTP status code to return to the client
	StatusCode int

	// Code is a machine-readable error code for client-side handling
	// (e.g., "CAMPAIGN_NOT_FOUND", "EMAIL_EXISTS")
	Code string

	// Message is a user-friendly error message safe to expose to clients
	Message string

	// Internal is the original error for logging purposes only.
	// This is NOT sent to the client and may contain sensitive details.
	Internal error
}

// Error implements the error interface, returning the user-friendly message
func (e *APIError) Error() string {
	return e.Message
}

// Unwrap allows errors.Is and errors.As to work with wrapped errors
func (e *APIError) Unwrap() error {
	return e.Internal
}

// Error code constants
const (
	CodeCampaignNotFound      = "CAMPAIGN_NOT_FOUND"
	CodeEmailExists           = "EMAIL_EXISTS"
	CodeEmailNotFound         = "EMAIL_NOT_FOUND"
	CodeUserNotFound          = "USER_NOT_FOUND"
	CodeInvalidInput          = "INVALID_INPUT"
	CodeUnauthorized          = "UNAUTHORIZED"
	CodeForbidden             = "FORBIDDEN"
	CodeInternalError         = "INTERNAL_ERROR"
	CodeSlugExists            = "SLUG_EXISTS"
	CodeInvalidStatus         = "INVALID_STATUS"
	CodeInvalidType           = "INVALID_TYPE"
	CodeInvalidReferral       = "INVALID_REFERRAL"
	CodeInvalidToken          = "INVALID_TOKEN"
	CodeAlreadyVerified       = "ALREADY_VERIFIED"
	CodeMaxSignupsReached     = "MAX_SIGNUPS_REACHED"
	CodeRewardNotFound        = "REWARD_NOT_FOUND"
	CodeUserRewardNotFound    = "USER_REWARD_NOT_FOUND"
	CodeInvalidRewardType     = "INVALID_REWARD_TYPE"
	CodeInvalidTriggerType    = "INVALID_TRIGGER_TYPE"
	CodeInvalidDeliveryMethod = "INVALID_DELIVERY_METHOD"
	CodeInvalidRewardStatus   = "INVALID_REWARD_STATUS"
	CodeRewardLimitReached    = "REWARD_LIMIT_REACHED"
	CodeUserLimitReached      = "USER_LIMIT_REACHED"
	CodeReferralNotFound      = "REFERRAL_NOT_FOUND"
	CodeReferralCodeRequired  = "REFERRAL_CODE_REQUIRED"
	CodeInvalidDateRange      = "INVALID_DATE_RANGE"
	CodeInvalidGranularity    = "INVALID_GRANULARITY"
	CodeNotFound              = "NOT_FOUND"
	CodeInvalidCredentials    = "INVALID_CREDENTIALS"
	CodePaymentProviderError  = "PAYMENT_PROVIDER_ERROR"
	CodeEmailServiceError     = "EMAIL_SERVICE_ERROR"
	CodeAIServiceError        = "AI_SERVICE_ERROR"
	CodeCaptchaRequired       = "CAPTCHA_REQUIRED"
	CodeCaptchaFailed         = "CAPTCHA_FAILED"
)

// NotFound creates a 404 Not Found error
func NotFound(code, message string) *APIError {
	return &APIError{
		StatusCode: http.StatusNotFound,
		Code:       code,
		Message:    message,
	}
}

// BadRequest creates a 400 Bad Request error
func BadRequest(code, message string) *APIError {
	return &APIError{
		StatusCode: http.StatusBadRequest,
		Code:       code,
		Message:    message,
	}
}

// Unauthorized creates a 401 Unauthorized error (missing/invalid auth token)
func Unauthorized(message string) *APIError {
	return &APIError{
		StatusCode: http.StatusUnauthorized,
		Code:       CodeUnauthorized,
		Message:    message,
	}
}

// Forbidden creates a 403 Forbidden error (authenticated but insufficient permissions)
func Forbidden(message string) *APIError {
	return &APIError{
		StatusCode: http.StatusForbidden,
		Code:       CodeForbidden,
		Message:    message,
	}
}

// Conflict creates a 409 Conflict error
func Conflict(code, message string) *APIError {
	return &APIError{
		StatusCode: http.StatusConflict,
		Code:       code,
		Message:    message,
	}
}

// InternalError creates a sanitized 500 Internal Server Error.
// The internal error is preserved for logging but not exposed to clients.
func InternalError(internalErr error) *APIError {
	return &APIError{
		StatusCode: http.StatusInternalServerError,
		Code:       CodeInternalError,
		Message:    "An internal error occurred. Please try again later.",
		Internal:   internalErr,
	}
}

// ServiceUnavailable creates a 503 Service Unavailable error for external service failures
func ServiceUnavailable(code, message string, internalErr error) *APIError {
	return &APIError{
		StatusCode: http.StatusServiceUnavailable,
		Code:       code,
		Message:    message,
		Internal:   internalErr,
	}
}

// WrapInternal creates an APIError with a custom message but preserves the internal error for logging
func WrapInternal(statusCode int, code, message string, internalErr error) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
		Internal:   internalErr,
	}
}
