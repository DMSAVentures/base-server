package leaderboard

import (
	"base-server/internal/apierrors"
	"base-server/internal/observability"
	"base-server/internal/store"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler handles HTTP requests for the leaderboard API
type Handler struct {
	processor *Processor
	store     store.Store
	logger    *observability.Logger
}

// NewHandler creates a new leaderboard handler
func NewHandler(processor *Processor, store store.Store, logger *observability.Logger) *Handler {
	return &Handler{
		processor: processor,
		store:     store,
		logger:    logger,
	}
}

// HandleGetUserRank handles GET /api/v1/leaderboard/rank
func (h *Handler) HandleGetUserRank(c *gin.Context) {
	ctx := c.Request.Context()
	startTime := time.Now()

	// Get account from context (set by auth middleware)
	accountVal, exists := c.Get("account")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized", "code": "UNAUTHORIZED"})
		return
	}
	account := accountVal.(store.Account)

	// Get API key ID for usage tracking
	apiKeyIDVal, _ := c.Get("api_key_id")
	var apiKeyID *uuid.UUID
	if apiKeyIDVal != nil {
		id := apiKeyIDVal.(uuid.UUID)
		apiKeyID = &id
	}

	// Parse request
	var req GetUserRankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	// Track usage (async)
	go h.trackUsage(ctx, account.ID, &req.CampaignID, apiKeyID, store.UsageOperationGetRank, http.StatusOK, time.Since(startTime))

	// Process request
	response, err := h.processor.GetUserRank(ctx, account, req)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleGetTopUsers handles GET /api/v1/leaderboard/top
func (h *Handler) HandleGetTopUsers(c *gin.Context) {
	ctx := c.Request.Context()
	startTime := time.Now()

	// Get account from context
	accountVal, exists := c.Get("account")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized", "code": "UNAUTHORIZED"})
		return
	}
	account := accountVal.(store.Account)

	// Get API key ID for usage tracking
	apiKeyIDVal, _ := c.Get("api_key_id")
	var apiKeyID *uuid.UUID
	if apiKeyIDVal != nil {
		id := apiKeyIDVal.(uuid.UUID)
		apiKeyID = &id
	}

	// Parse request
	var req GetTopUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	// Track usage (async)
	go h.trackUsage(ctx, account.ID, &req.CampaignID, apiKeyID, store.UsageOperationGetTopN, http.StatusOK, time.Since(startTime))

	// Process request
	response, err := h.processor.GetTopUsers(ctx, account, req)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleGetUsersAround handles GET /api/v1/leaderboard/around
func (h *Handler) HandleGetUsersAround(c *gin.Context) {
	ctx := c.Request.Context()
	startTime := time.Now()

	// Get account from context
	accountVal, exists := c.Get("account")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized", "code": "UNAUTHORIZED"})
		return
	}
	account := accountVal.(store.Account)

	// Get API key ID for usage tracking
	apiKeyIDVal, _ := c.Get("api_key_id")
	var apiKeyID *uuid.UUID
	if apiKeyIDVal != nil {
		id := apiKeyIDVal.(uuid.UUID)
		apiKeyID = &id
	}

	// Parse request
	var req GetUsersAroundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	// Track usage (async)
	go h.trackUsage(ctx, account.ID, &req.CampaignID, apiKeyID, store.UsageOperationGetUsersAround, http.StatusOK, time.Since(startTime))

	// Process request
	response, err := h.processor.GetUsersAround(ctx, account, req)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleUpdateUserScore handles POST /api/v1/leaderboard/update
func (h *Handler) HandleUpdateUserScore(c *gin.Context) {
	ctx := c.Request.Context()
	startTime := time.Now()

	// Get account from context
	accountVal, exists := c.Get("account")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized", "code": "UNAUTHORIZED"})
		return
	}
	account := accountVal.(store.Account)

	// Get API key ID for usage tracking
	apiKeyIDVal, _ := c.Get("api_key_id")
	var apiKeyID *uuid.UUID
	if apiKeyIDVal != nil {
		id := apiKeyIDVal.(uuid.UUID)
		apiKeyID = &id
	}

	// Parse request
	var req UpdateUserScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	// Track usage (async)
	go h.trackUsage(ctx, account.ID, &req.CampaignID, apiKeyID, store.UsageOperationUpdateScore, http.StatusOK, time.Since(startTime))

	// Process request
	response, err := h.processor.UpdateUserScore(ctx, account, req)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleSyncToRedis handles POST /api/v1/leaderboard/sync
func (h *Handler) HandleSyncToRedis(c *gin.Context) {
	ctx := c.Request.Context()
	startTime := time.Now()

	// Get account from context
	accountVal, exists := c.Get("account")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized", "code": "UNAUTHORIZED"})
		return
	}
	account := accountVal.(store.Account)

	// Get API key ID for usage tracking
	apiKeyIDVal, _ := c.Get("api_key_id")
	var apiKeyID *uuid.UUID
	if apiKeyIDVal != nil {
		id := apiKeyIDVal.(uuid.UUID)
		apiKeyID = &id
	}

	// Parse request
	var req SyncToRedisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	// Track usage (async)
	go h.trackUsage(ctx, account.ID, &req.CampaignID, apiKeyID, store.UsageOperationSyncDatabase, http.StatusOK, time.Since(startTime))

	// Process request
	response, err := h.processor.SyncToRedis(ctx, account, req)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleHealthCheck handles GET /api/v1/leaderboard/health
func (h *Handler) HandleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "leaderboard-api",
		"timestamp": time.Now(),
	})
}

// trackUsage creates a usage event for billing
func (h *Handler) trackUsage(ctx context.Context, accountID uuid.UUID, campaignID, apiKeyID *uuid.UUID, operation string, statusCode int, responseTime time.Duration) {
	// Get request ID from context
	requestIDVal := ctx.Value("request_id")
	var requestID *uuid.UUID
	if requestIDVal != nil {
		if id, ok := requestIDVal.(string); ok {
			parsed, err := uuid.Parse(id)
			if err == nil {
				requestID = &parsed
			}
		}
	}

	// Create usage event
	responseTimeMs := int(responseTime.Milliseconds())
	err := h.store.CreateUsageEvent(ctx, store.CreateUsageEventParams{
		AccountID:     accountID,
		CampaignID:     campaignID,
		APIKeyID:       apiKeyID,
		Operation:      operation,
		Count:          1,
		RequestID:      requestID,
		ResponseTimeMs: &responseTimeMs,
		StatusCode:     &statusCode,
	})

	if err != nil {
		h.logger.Error(ctx, "failed to track usage", err,
			observability.Field{Key: "operation", Value: operation},
		)
	}
}
