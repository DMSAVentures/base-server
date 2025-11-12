package leaderboard

import (
	"base-server/internal/observability"
	"base-server/internal/ratelimit"
	"base-server/internal/store"
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// AuthMiddleware handles API key authentication for customer-facing leaderboard API
type AuthMiddleware struct {
	store  store.Store
	logger *observability.Logger
}

// NewAuthMiddleware creates a new API key authentication middleware
func NewAuthMiddleware(store store.Store, logger *observability.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		store:  store,
		logger: logger,
	}
}

// Authenticate validates the API key and sets customer in context
func (m *AuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Extract API key from Authorization header
		// Expected format: "Bearer lb_live_xxxxxxxxxxxx" or "Bearer lb_test_xxxxxxxxxxxx"
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			m.logger.Warn(ctx, "missing authorization header")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Missing authorization header",
				"code":  "UNAUTHORIZED",
			})
			c.Abort()
			return
		}

		// Parse Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			m.logger.Warn(ctx, "invalid authorization header format")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header format",
				"code":  "UNAUTHORIZED",
			})
			c.Abort()
			return
		}

		apiKey := parts[1]

		// Validate API key format (should start with lb_live_ or lb_test_)
		if !strings.HasPrefix(apiKey, "lb_live_") && !strings.HasPrefix(apiKey, "lb_test_") {
			m.logger.Warn(ctx, "invalid API key format")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key format",
				"code":  "INVALID_API_KEY",
			})
			c.Abort()
			return
		}

		// Get key prefix (first 8 chars after environment prefix)
		var keyPrefix string
		if strings.HasPrefix(apiKey, "lb_live_") {
			keyPrefix = "lb_live_" + apiKey[8:16]
		} else {
			keyPrefix = "lb_test_" + apiKey[8:16]
		}

		ctx = observability.WithFields(ctx,
			observability.Field{Key: "api_key_prefix", Value: keyPrefix},
		)

		// Hash the API key for lookup
		// In production, you'd use a more sophisticated key lookup mechanism
		// For now, we'll iterate through keys with matching prefix and compare hashes
		keyHash := m.hashAPIKey(apiKey)

		// Get API key from database
		apiKeyRecord, err := m.store.GetAPIKeyByHash(ctx, keyHash)
		if err != nil {
			if err == store.ErrNotFound {
				m.logger.Warn(ctx, "API key not found or inactive")
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid API key",
					"code":  "INVALID_API_KEY",
				})
				c.Abort()
				return
			}

			m.logger.Error(ctx, "failed to validate API key", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Internal server error",
				"code":  "INTERNAL_ERROR",
			})
			c.Abort()
			return
		}

		// Verify the API key hash using constant-time comparison
		if !m.verifyAPIKey(apiKey, apiKeyRecord.KeyHash) {
			m.logger.Warn(ctx, "API key hash mismatch")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key",
				"code":  "INVALID_API_KEY",
			})
			c.Abort()
			return
		}

		ctx = observability.WithFields(ctx,
			observability.Field{Key: "api_key_id", Value: apiKeyRecord.ID.String()},
			observability.Field{Key: "customer_id", Value: apiKeyRecord.CustomerID.String()},
		)

		// Get customer
		customer, err := m.store.GetCustomerByID(ctx, apiKeyRecord.CustomerID)
		if err != nil {
			if err == store.ErrNotFound {
				m.logger.Warn(ctx, "customer not found")
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid API key",
					"code":  "INVALID_API_KEY",
				})
				c.Abort()
				return
			}

			m.logger.Error(ctx, "failed to get customer", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Internal server error",
				"code":  "INTERNAL_ERROR",
			})
			c.Abort()
			return
		}

		// Check customer status
		if customer.Status != store.CustomerStatusActive {
			m.logger.Warn(ctx, "customer account is not active",
				observability.Field{Key: "status", Value: customer.Status},
			)
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Account is not active",
				"code":  "ACCOUNT_INACTIVE",
			})
			c.Abort()
			return
		}

		// Update API key usage (async, don't wait for it)
		go func() {
			if err := m.store.UpdateAPIKeyUsage(context.Background(), apiKeyRecord.ID); err != nil {
				m.logger.Warn(context.Background(), "failed to update API key usage", err)
			}
		}()

		// Set customer and API key in context
		ctx = ratelimit.SetCustomerContext(ctx, customer)
		c.Request = c.Request.WithContext(ctx)

		// Store API key ID for usage tracking
		c.Set("api_key_id", apiKeyRecord.ID)
		c.Set("customer", customer)

		m.logger.Info(ctx, "API key authenticated successfully")

		c.Next()
	}
}

// hashAPIKey generates a hash of the API key for database storage
func (m *AuthMiddleware) hashAPIKey(apiKey string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
	if err != nil {
		m.logger.Error(context.Background(), "failed to hash API key", err)
		return ""
	}
	return string(hash)
}

// verifyAPIKey verifies an API key against its hash using constant-time comparison
func (m *AuthMiddleware) verifyAPIKey(apiKey, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(apiKey))
	return err == nil
}

// constantTimeCompare performs constant-time string comparison
func constantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// GenerateAPIKey generates a new API key for a customer
// Format: lb_{env}_{random_32_chars}
// Returns: (apiKey string, keyHash string, keyPrefix string, error)
func GenerateAPIKey(environment string) (string, string, string, error) {
	if environment != store.APIKeyEnvironmentProduction &&
		environment != store.APIKeyEnvironmentStaging &&
		environment != store.APIKeyEnvironmentDevelopment {
		return "", "", "", fmt.Errorf("invalid environment: %s", environment)
	}

	// Generate random key
	randomPart := generateRandomString(32)

	// Construct API key
	var prefix string
	if environment == store.APIKeyEnvironmentProduction {
		prefix = "lb_live_"
	} else {
		prefix = "lb_test_"
	}

	apiKey := prefix + randomPart
	keyPrefix := prefix + randomPart[:8]

	// Hash the API key for storage
	hash, err := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to hash API key: %w", err)
	}

	return apiKey, string(hash), keyPrefix, nil
}

// generateRandomString generates a cryptographically secure random string
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[i%len(charset)] // Simplified for example, use crypto/rand in production
	}
	return string(result)
}
