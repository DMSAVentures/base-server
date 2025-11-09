package apierrors

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
)

// ValidationErrorDetail represents a single field validation error
type ValidationErrorDetail struct {
	Field   string `json:"field"`           // Field name that failed validation
	Message string `json:"message"`         // Human-readable error message
	Tag     string `json:"tag,omitempty"`   // Validation tag that failed (e.g., "required", "email")
	Value   string `json:"value,omitempty"` // The value that failed (omitted for security on password fields)
}

// ValidationError creates a structured validation error with field details
func ValidationError(err error) *APIError {
	var details []ValidationErrorDetail

	// Check if it's a validator.ValidationErrors type
	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		for _, fieldErr := range validationErrs {
			detail := ValidationErrorDetail{
				Field:   fieldErr.Field(),
				Tag:     fieldErr.Tag(),
				Message: getValidationMessage(fieldErr),
			}

			// Don't include value for sensitive fields
			if !isSensitiveField(fieldErr.Field()) {
				detail.Value = fmt.Sprintf("%v", fieldErr.Value())
			}

			details = append(details, detail)
		}
	}

	// If we couldn't parse the validation errors, return a generic message
	if len(details) == 0 {
		return &APIError{
			StatusCode: http.StatusBadRequest,
			Code:       CodeInvalidInput,
			Message:    "Invalid request. Please check your input.",
			Internal:   err,
		}
	}

	// Create a user-friendly error message
	message := buildValidationMessage(details)

	return &APIError{
		StatusCode: http.StatusBadRequest,
		Code:       CodeInvalidInput,
		Message:    message,
		Internal:   err,
	}
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

// buildValidationMessage creates a user-friendly message from validation error details
func buildValidationMessage(details []ValidationErrorDetail) string {
	if len(details) == 0 {
		return "Invalid request"
	}

	if len(details) == 1 {
		return details[0].Message
	}

	// Multiple validation errors - list them
	var messages []string
	for _, detail := range details {
		messages = append(messages, detail.Message)
	}

	return "Validation failed: " + strings.Join(messages, "; ")
}

// isSensitiveField checks if a field name indicates sensitive data
func isSensitiveField(fieldName string) bool {
	lowerField := strings.ToLower(fieldName)
	sensitiveFields := []string{
		"password",
		"token",
		"secret",
		"key",
		"apikey",
		"api_key",
		"creditcard",
		"credit_card",
		"ssn",
		"cvv",
	}

	for _, sensitive := range sensitiveFields {
		if strings.Contains(lowerField, sensitive) {
			return true
		}
	}

	return false
}
