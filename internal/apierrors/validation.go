package apierrors

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// ValidationError sends a 400 response for validation errors
func ValidationError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	ctx := c.Request.Context()

	// Check if it's a validator error
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		message := buildValidationMessage(validationErrs)
		logger.Error(ctx, "validation failed", err)
		respond(c, http.StatusBadRequest, "INVALID_INPUT", message)
		return
	}

	// Not a validation error - might be a JSON parsing error or other binding issue
	logger.Error(ctx, "request binding failed", err)
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error: "Invalid request format. Please check your JSON syntax.",
		Code:  "INVALID_INPUT",
	})
}

// buildValidationMessage creates a user-friendly message from validation errors
func buildValidationMessage(validationErrs validator.ValidationErrors) string {
	if len(validationErrs) == 0 {
		return "Invalid request"
	}

	if len(validationErrs) == 1 {
		return getValidationMessage(validationErrs[0])
	}

	var messages []string
	for _, fieldErr := range validationErrs {
		messages = append(messages, getValidationMessage(fieldErr))
	}
	return "Validation failed: " + strings.Join(messages, "; ")
}

// getValidationMessage returns a human-readable message for a validation error
func getValidationMessage(fieldErr validator.FieldError) string {
	field := fieldErr.Field()
	tag := fieldErr.Tag()

	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, fieldErr.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, fieldErr.Param())
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", field, fieldErr.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, fieldErr.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, fieldErr.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, fieldErr.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, fieldErr.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, fieldErr.Param())
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID", field)
	case "url":
		return fmt.Sprintf("%s must be a valid URL", field)
	case "alphanum":
		return fmt.Sprintf("%s must contain only alphanumeric characters", field)
	default:
		return fmt.Sprintf("%s failed validation (%s)", field, tag)
	}
}
