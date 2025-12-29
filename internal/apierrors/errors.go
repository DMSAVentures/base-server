package apierrors

import (
	"net/http"

	"base-server/internal/observability"

	"github.com/gin-gonic/gin"
)

var logger = observability.NewLogger()

// ErrorResponse is the JSON structure returned to API clients
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// respond writes the error response and logs correlation info
func respond(c *gin.Context, statusCode int, code, message string) {
	ctx := c.Request.Context()
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "status_code", Value: statusCode},
		observability.Field{Key: "error_code", Value: code},
		observability.Field{Key: "error_message", Value: message},
	)
	logger.Info(ctx, "API error response")

	c.JSON(statusCode, ErrorResponse{
		Error: message,
		Code:  code,
	})
}

// NotFound sends a 404 response
func NotFound(c *gin.Context, message string) {
	respond(c, http.StatusNotFound, "NOT_FOUND", message)
}

// BadRequest sends a 400 response
func BadRequest(c *gin.Context, code, message string) {
	respond(c, http.StatusBadRequest, code, message)
}

// Unauthorized sends a 401 response
func Unauthorized(c *gin.Context, message string) {
	respond(c, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

// Forbidden sends a 403 response
func Forbidden(c *gin.Context, code, message string) {
	respond(c, http.StatusForbidden, code, message)
}

// Conflict sends a 409 response
func Conflict(c *gin.Context, code, message string) {
	respond(c, http.StatusConflict, code, message)
}

// ServiceUnavailable sends a 503 response and logs the internal error
func ServiceUnavailable(c *gin.Context, code, message string, internalErr error) {
	ctx := c.Request.Context()
	logger.Error(ctx, "service unavailable", internalErr)
	respond(c, http.StatusServiceUnavailable, code, message)
}

// InternalError sends a sanitized 500 response - never exposes internal details
func InternalError(c *gin.Context, internalErr error) {
	ctx := c.Request.Context()
	logger.Error(ctx, "internal error", internalErr)
	respond(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred. Please try again later.")
}
