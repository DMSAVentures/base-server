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

// AccountContextKey is the key used to store account in request context
type contextKey string

const AccountContextKey contextKey = "account"

// Middleware creates a Gin middleware for rate limiting
func (s *Service) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Get account from context (should be set by auth middleware)
		accountVal := c.Request.Context().Value(AccountContextKey)
		if accountVal == nil {
			// No account in context, skip rate limiting (e.g., public endpoints)
			c.Next()
			return
		}

		account, ok := accountVal.(store.Account)
		if !ok {
			s.logger.Error(ctx, "invalid account in context", fmt.Errorf("type assertion failed"))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error", "code": "INTERNAL_ERROR"})
			c.Abort()
			return
		}

		ctx = observability.WithFields(ctx,
			observability.Field{Key: "account_id", Value: account.ID.String()},
			observability.Field{Key: "rate_limit_rpm", Value: account.RateLimitRPM},
		)

		// Check rate limit
		result, err := s.CheckRateLimit(ctx, account.ID, account.RateLimitRPM)
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

// SetAccountContext is a helper to set account in request context
func SetAccountContext(ctx context.Context, account store.Account) context.Context {
	return context.WithValue(ctx, AccountContextKey, account)
}

// GetAccountFromContext retrieves account from request context
func GetAccountFromContext(ctx context.Context) (store.Account, bool) {
	account, ok := ctx.Value(AccountContextKey).(store.Account)
	return account, ok
}

// GetAccountIDFromContext retrieves account ID from request context
func GetAccountIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	account, ok := GetAccountFromContext(ctx)
	if !ok {
		return uuid.Nil, false
	}
	return account.ID, true
}
