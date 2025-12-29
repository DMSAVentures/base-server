package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"base-server/internal/apierrors"
	"base-server/internal/observability"
	"base-server/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Available scopes for API keys
var ValidScopes = []string{"zapier", "webhooks", "read", "write", "all"}

// APIKeyStore defines the interface for API key database operations
type APIKeyStore interface {
	CreateAPIKey(ctx context.Context, params store.CreateAPIKeyParams) (store.APIKey, error)
	GetAPIKeysByAccount(ctx context.Context, accountID uuid.UUID) ([]store.APIKey, error)
	GetAPIKeyByID(ctx context.Context, keyID uuid.UUID) (store.APIKey, error)
	RevokeAPIKey(ctx context.Context, keyID uuid.UUID, revokedBy uuid.UUID) error
	UpdateAPIKeyName(ctx context.Context, keyID uuid.UUID, name string) error
}

// Handler handles API key HTTP requests
type Handler struct {
	store  APIKeyStore
	logger *observability.Logger
}

// New creates a new Handler
func New(store APIKeyStore, logger *observability.Logger) *Handler {
	return &Handler{
		store:  store,
		logger: logger,
	}
}

// handleError maps errors to appropriate API responses
func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		apierrors.NotFound(c, "API key not found")
	default:
		apierrors.InternalError(c, err)
	}
}

// CreateAPIKeyRequest represents a request to create an API key
type CreateAPIKeyRequest struct {
	Name      string   `json:"name" binding:"required,min=1,max=100"`
	Scopes    []string `json:"scopes" binding:"required,min=1"`
	ExpiresIn *int     `json:"expires_in_days"` // Optional: days until expiration
}

// CreateAPIKeyResponse represents the response for creating an API key
type CreateAPIKeyResponse struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Key       string   `json:"key"` // Only returned once on creation
	KeyPrefix string   `json:"key_prefix"`
	Scopes    []string `json:"scopes"`
	ExpiresAt *string  `json:"expires_at,omitempty"`
	CreatedAt string   `json:"created_at"`
}

// APIKeyResponse represents an API key in list responses (no secret)
type APIKeyResponse struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	KeyPrefix     string   `json:"key_prefix"`
	Scopes        []string `json:"scopes"`
	Status        string   `json:"status"`
	LastUsedAt    *string  `json:"last_used_at,omitempty"`
	TotalRequests int      `json:"total_requests"`
	ExpiresAt     *string  `json:"expires_at,omitempty"`
	CreatedAt     string   `json:"created_at"`
	RevokedAt     *string  `json:"revoked_at,omitempty"`
}

// UpdateAPIKeyRequest represents a request to update an API key
type UpdateAPIKeyRequest struct {
	Name string `json:"name" binding:"required,min=1,max=100"`
}

// HandleCreateAPIKey handles POST /api/protected/api-keys
func (h *Handler) HandleCreateAPIKey(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account and user ID from context (set by auth middleware)
	accountIDStr := c.GetString("Account-ID")
	userIDStr := c.GetString("User-ID")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	var userID *uuid.UUID
	if userIDStr != "" {
		parsed, err := uuid.Parse(userIDStr)
		if err == nil {
			userID = &parsed
		}
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "account_id", Value: accountID})

	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	// Validate scopes
	for _, scope := range req.Scopes {
		valid := false
		for _, validScope := range ValidScopes {
			if scope == validScope {
				valid = true
				break
			}
		}
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scope: " + scope})
			return
		}
	}

	// Generate API key
	rawKey, keyHash, keyPrefix, err := generateAPIKey()
	if err != nil {
		h.logger.Error(ctx, "failed to generate API key", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate API key"})
		return
	}

	// Calculate expiration if provided
	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		exp := time.Now().AddDate(0, 0, *req.ExpiresIn)
		expiresAt = &exp
	}

	// Create API key in database
	apiKey, err := h.store.CreateAPIKey(ctx, store.CreateAPIKeyParams{
		AccountID:     accountID,
		Name:          req.Name,
		KeyHash:       keyHash,
		KeyPrefix:     keyPrefix,
		Scopes:        req.Scopes,
		RateLimitTier: "standard",
		ExpiresAt:     expiresAt,
		CreatedBy:     userID,
	})
	if err != nil {
		h.logger.Error(ctx, "failed to create API key", err)
		h.handleError(c, err)
		return
	}

	h.logger.Info(ctx, "created API key")

	// Build response with the raw key (only time it's returned)
	var expiresAtStr *string
	if apiKey.ExpiresAt != nil {
		s := apiKey.ExpiresAt.Format(time.RFC3339)
		expiresAtStr = &s
	}

	c.JSON(http.StatusCreated, CreateAPIKeyResponse{
		ID:        apiKey.ID.String(),
		Name:      apiKey.Name,
		Key:       rawKey,
		KeyPrefix: apiKey.KeyPrefix,
		Scopes:    apiKey.Scopes,
		ExpiresAt: expiresAtStr,
		CreatedAt: apiKey.CreatedAt.Format(time.RFC3339),
	})
}

// HandleListAPIKeys handles GET /api/protected/api-keys
func (h *Handler) HandleListAPIKeys(c *gin.Context) {
	ctx := c.Request.Context()

	accountIDStr := c.GetString("Account-ID")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "account_id", Value: accountID})

	apiKeys, err := h.store.GetAPIKeysByAccount(ctx, accountID)
	if err != nil {
		h.logger.Error(ctx, "failed to list API keys", err)
		h.handleError(c, err)
		return
	}

	response := make([]APIKeyResponse, len(apiKeys))
	for i, key := range apiKeys {
		response[i] = toAPIKeyResponse(key)
	}

	c.JSON(http.StatusOK, response)
}

// HandleGetAPIKey handles GET /api/protected/api-keys/:id
func (h *Handler) HandleGetAPIKey(c *gin.Context) {
	ctx := c.Request.Context()

	accountIDStr := c.GetString("Account-ID")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid api key id"})
		return
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID},
		observability.Field{Key: "api_key_id", Value: keyID},
	)

	apiKey, err := h.store.GetAPIKeyByID(ctx, keyID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			apierrors.NotFound(c, "API key not found")
			return
		}
		h.logger.Error(ctx, "failed to get API key", err)
		h.handleError(c, err)
		return
	}

	// Verify the key belongs to this account
	if apiKey.AccountID != accountID {
		apierrors.NotFound(c, "API key not found")
		return
	}

	c.JSON(http.StatusOK, toAPIKeyResponse(apiKey))
}

// HandleUpdateAPIKey handles PUT /api/protected/api-keys/:id
func (h *Handler) HandleUpdateAPIKey(c *gin.Context) {
	ctx := c.Request.Context()

	accountIDStr := c.GetString("Account-ID")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid api key id"})
		return
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID},
		observability.Field{Key: "api_key_id", Value: keyID},
	)

	var req UpdateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	// Verify the key exists and belongs to this account
	apiKey, err := h.store.GetAPIKeyByID(ctx, keyID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			apierrors.NotFound(c, "API key not found")
			return
		}
		h.logger.Error(ctx, "failed to get API key", err)
		h.handleError(c, err)
		return
	}

	if apiKey.AccountID != accountID {
		apierrors.NotFound(c, "API key not found")
		return
	}

	// Update the name
	err = h.store.UpdateAPIKeyName(ctx, keyID, req.Name)
	if err != nil {
		h.logger.Error(ctx, "failed to update API key", err)
		h.handleError(c, err)
		return
	}

	h.logger.Info(ctx, "updated API key name")

	// Fetch the updated key
	apiKey, err = h.store.GetAPIKeyByID(ctx, keyID)
	if err != nil {
		h.logger.Error(ctx, "failed to get updated API key", err)
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, toAPIKeyResponse(apiKey))
}

// HandleRevokeAPIKey handles DELETE /api/protected/api-keys/:id
func (h *Handler) HandleRevokeAPIKey(c *gin.Context) {
	ctx := c.Request.Context()

	accountIDStr := c.GetString("Account-ID")
	userIDStr := c.GetString("User-ID")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid api key id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID},
		observability.Field{Key: "api_key_id", Value: keyID},
	)

	// Verify the key exists and belongs to this account
	apiKey, err := h.store.GetAPIKeyByID(ctx, keyID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			apierrors.NotFound(c, "API key not found")
			return
		}
		h.logger.Error(ctx, "failed to get API key", err)
		h.handleError(c, err)
		return
	}

	if apiKey.AccountID != accountID {
		apierrors.NotFound(c, "API key not found")
		return
	}

	// Revoke the key
	err = h.store.RevokeAPIKey(ctx, keyID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			apierrors.NotFound(c, "API key not found or already revoked")
			return
		}
		h.logger.Error(ctx, "failed to revoke API key", err)
		h.handleError(c, err)
		return
	}

	h.logger.Info(ctx, "revoked API key")

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// HandleGetScopes handles GET /api/protected/api-keys/scopes
// Returns available scopes for API keys
func (h *Handler) HandleGetScopes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"scopes": ValidScopes,
	})
}

// generateAPIKey generates a new API key with hash and prefix
func generateAPIKey() (rawKey, keyHash, keyPrefix string, err error) {
	// Generate 32 random bytes
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", "", err
	}

	// Create the raw key with prefix
	rawKey = "sk_" + hex.EncodeToString(bytes)

	// Create hash for storage
	hash := sha256.Sum256([]byte(rawKey))
	keyHash = hex.EncodeToString(hash[:])

	// Create prefix for display (first 8 chars after sk_)
	keyPrefix = rawKey[:11] + "..."

	return rawKey, keyHash, keyPrefix, nil
}

// toAPIKeyResponse converts a store.APIKey to APIKeyResponse
func toAPIKeyResponse(key store.APIKey) APIKeyResponse {
	var lastUsedAt *string
	if key.LastUsedAt != nil {
		s := key.LastUsedAt.Format(time.RFC3339)
		lastUsedAt = &s
	}

	var expiresAt *string
	if key.ExpiresAt != nil {
		s := key.ExpiresAt.Format(time.RFC3339)
		expiresAt = &s
	}

	var revokedAt *string
	if key.RevokedAt != nil {
		s := key.RevokedAt.Format(time.RFC3339)
		revokedAt = &s
	}

	return APIKeyResponse{
		ID:            key.ID.String(),
		Name:          key.Name,
		KeyPrefix:     key.KeyPrefix,
		Scopes:        key.Scopes,
		Status:        key.Status,
		LastUsedAt:    lastUsedAt,
		TotalRequests: key.TotalRequests,
		ExpiresAt:     expiresAt,
		CreatedAt:     key.CreatedAt.Format(time.RFC3339),
		RevokedAt:     revokedAt,
	}
}
