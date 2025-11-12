package ratelimit

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CustomerContextKey is the key used to store customer in request context
type contextKey string

const CustomerContextKey contextKey = "customer"

// Middleware creates a Gin middleware for rate limiting
func (s *Service) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Get customer from context (should be set by auth middleware)
		customerVal := c.Request.Context().Value(CustomerContextKey)
		if customerVal == nil {
			// No customer in context, skip rate limiting (e.g., public endpoints)
			c.Next()
			return
		}

		customer, ok := customerVal.(store.Customer)
		if !ok {
			s.logger.Error(ctx, "invalid customer in context", fmt.Errorf("type assertion failed"))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error", "code": "INTERNAL_ERROR"})
			c.Abort()
			return
		}

		ctx = observability.WithFields(ctx,
			observability.Field{Key: "customer_id", Value: customer.ID.String()},
			observability.Field{Key: "rate_limit_rpm", Value: customer.RateLimitRPM},
		)

		// Check rate limit
		result, err := s.CheckRateLimit(ctx, customer.ID, customer.RateLimitRPM)
		if err != nil {
			s.logger.Error(ctx, "rate limit check failed", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error", "code": "INTERNAL_ERROR"})
			c.Abort()
			return
		}

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", result.Limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", result.ResetAt.Unix()))

		// Check if rate limit exceeded
		if !result.Allowed {
			c.Header("Retry-After", fmt.Sprintf("%d", result.RetryAfterMs/1000))
			s.logger.Warn(ctx, "rate limit exceeded",
				observability.Field{Key: "limit", Value: result.Limit},
				observability.Field{Key: "retry_after_ms", Value: result.RetryAfterMs},
			)

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":         "Rate limit exceeded",
				"code":          "RATE_LIMIT_EXCEEDED",
				"limit":         result.Limit,
				"retry_after":   result.RetryAfterMs / 1000, // Convert to seconds
				"reset_at":      result.ResetAt.Unix(),
			})
			c.Abort()
			return
		}

		// Rate limit check passed, continue
		c.Next()
	}
}

// SetCustomerContext is a helper to set customer in request context
func SetCustomerContext(ctx context.Context, customer store.Customer) context.Context {
	return context.WithValue(ctx, CustomerContextKey, customer)
}

// GetCustomerFromContext retrieves customer from request context
func GetCustomerFromContext(ctx context.Context) (store.Customer, bool) {
	customer, ok := ctx.Value(CustomerContextKey).(store.Customer)
	return customer, ok
}

// GetCustomerIDFromContext retrieves customer ID from request context
func GetCustomerIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	customer, ok := GetCustomerFromContext(ctx)
	if !ok {
		return uuid.Nil, false
	}
	return customer.ID, true
}
