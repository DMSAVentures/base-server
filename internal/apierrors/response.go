package apierrors

import (
	"errors"

	"base-server/internal/observability"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Package-level logger that uses context for observability
var logger = observability.NewLogger()

// ErrorResponse is the JSON structure returned to API clients for errors
type ErrorResponse struct {
	Error string `json:"error"`          // User-friendly error message
	Code  string `json:"code,omitempty"` // Machine-readable error code
}

// RespondWithError handles error logging and sends a sanitized JSON response to the client.
// This is the primary function handlers should use for error responses.
//
// It performs the following:
// 1. Converts the error to an APIError (using MapError if necessary)
// 2. Logs the API response for correlation (processor already logged the detailed error)
// 3. Sends a sanitized error response to the client
//
// Example usage:
//
//	if err != nil {
//	    apierrors.RespondWithError(c, err)
//	    return
//	}
func RespondWithError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	ctx := c.Request.Context()

	// Convert to APIError
	apiErr := MapError(err)

	// Log API error response for correlation with processor logs
	// Processor has already logged the detailed error with full context
	// This log entry includes request_id for correlation
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "status_code", Value: apiErr.StatusCode},
		observability.Field{Key: "error_code", Value: apiErr.Code},
		observability.Field{Key: "error_message", Value: apiErr.Message},
	)
	logger.Info(ctx, "API error response")

	// Send sanitized response to client
	c.JSON(apiErr.StatusCode, ErrorResponse{
		Error: apiErr.Message,
		Code:  apiErr.Code,
	})
}

// RespondWithValidationError handles Gin binding/validation errors and returns
// structured validation error responses.
//
// This should be used when c.ShouldBindJSON or similar binding functions fail.
//
// Example usage:
//
//	var req SomeRequest
//	if err := c.ShouldBindJSON(&req); err != nil {
//	    apierrors.RespondWithValidationError(c, err)
//	    return
//	}
func RespondWithValidationError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	ctx := c.Request.Context()

	// Check if it's a validator error
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		// Create structured validation error
		apiErr := ValidationError(err)
		logger.Error(ctx, "Validation failed", err)

		c.JSON(apiErr.StatusCode, ErrorResponse{
			Error: apiErr.Message,
			Code:  apiErr.Code,
		})
		return
	}

	// Not a validation error - might be a JSON parsing error or other binding issue
	logger.Error(ctx, "Request binding failed", err)
	c.JSON(400, ErrorResponse{
		Error: "Invalid request format. Please check your JSON syntax.",
		Code:  CodeInvalidInput,
	})
}
