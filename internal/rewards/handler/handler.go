package handler

import (
	"base-server/internal/observability"
	"base-server/internal/rewards/processor"
	"base-server/internal/store"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	processor processor.RewardProcessor
	logger    *observability.Logger
}

func New(processor processor.RewardProcessor, logger *observability.Logger) Handler {
	return Handler{
		processor: processor,
		logger:    logger,
	}
}

// CreateRewardRequest represents the HTTP request for creating a reward
type CreateRewardRequest struct {
	Name           string       `json:"name" binding:"required,max=255"`
	Description    *string      `json:"description,omitempty"`
	Type           string       `json:"type" binding:"required,oneof=early_access discount premium_feature merchandise custom"`
	Config         *store.JSONB `json:"config,omitempty"`
	TriggerType    string       `json:"trigger_type" binding:"required,oneof=referral_count position milestone manual"`
	TriggerConfig  *store.JSONB `json:"trigger_config,omitempty"`
	DeliveryMethod string       `json:"delivery_method" binding:"required,oneof=email webhook manual"`
	DeliveryConfig *store.JSONB `json:"delivery_config,omitempty"`
	TotalAvailable *int         `json:"total_available,omitempty"`
	UserLimit      *int         `json:"user_limit,omitempty"`
	StartsAt       *string      `json:"starts_at,omitempty"`
	ExpiresAt      *string      `json:"expires_at,omitempty"`
}

// UpdateRewardRequest represents the HTTP request for updating a reward
type UpdateRewardRequest struct {
	Name           *string      `json:"name,omitempty"`
	Description    *string      `json:"description,omitempty"`
	Config         *store.JSONB `json:"config,omitempty"`
	TriggerConfig  *store.JSONB `json:"trigger_config,omitempty"`
	DeliveryConfig *store.JSONB `json:"delivery_config,omitempty"`
	Status         *string      `json:"status,omitempty" binding:"omitempty,oneof=active paused expired"`
	ExpiresAt      *string      `json:"expires_at,omitempty"`
}

// GrantRewardRequest represents the HTTP request for granting a reward
type GrantRewardRequest struct {
	RewardID uuid.UUID `json:"reward_id" binding:"required"`
	Reason   *string   `json:"reason,omitempty"`
}

// HandleCreateReward creates a new reward
func (h *Handler) HandleCreateReward(c *gin.Context) {
	ctx := c.Request.Context()

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	var req CreateRewardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Set default empty JSONB if not provided
	if req.Config == nil {
		emptyJSON := store.JSONB{}
		req.Config = &emptyJSON
	}
	if req.TriggerConfig == nil {
		emptyJSON := store.JSONB{}
		req.TriggerConfig = &emptyJSON
	}
	if req.DeliveryConfig == nil {
		emptyJSON := store.JSONB{}
		req.DeliveryConfig = &emptyJSON
	}

	// Parse timestamps if provided
	var startsAt *time.Time
	if req.StartsAt != nil {
		parsed, err := time.Parse(time.RFC3339, *req.StartsAt)
		if err != nil {
			h.logger.Error(ctx, "failed to parse starts_at", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid starts_at format, expected RFC3339"})
			return
		}
		startsAt = &parsed
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		parsed, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			h.logger.Error(ctx, "failed to parse expires_at", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expires_at format, expected RFC3339"})
			return
		}
		expiresAt = &parsed
	}

	// Default user limit to 1 if not provided
	userLimit := 1
	if req.UserLimit != nil {
		userLimit = *req.UserLimit
	}

	processorReq := processor.CreateRewardRequest{
		Name:           req.Name,
		Description:    req.Description,
		Type:           req.Type,
		Config:         *req.Config,
		TriggerType:    req.TriggerType,
		TriggerConfig:  *req.TriggerConfig,
		DeliveryMethod: req.DeliveryMethod,
		DeliveryConfig: *req.DeliveryConfig,
		TotalAvailable: req.TotalAvailable,
		UserLimit:      userLimit,
		StartsAt:       startsAt,
		ExpiresAt:      expiresAt,
	}

	reward, err := h.processor.CreateReward(ctx, campaignID, processorReq)
	if err != nil {
		h.logger.Error(ctx, "failed to create reward", err)
		if errors.Is(err, processor.ErrInvalidRewardType) ||
			errors.Is(err, processor.ErrInvalidTriggerType) ||
			errors.Is(err, processor.ErrInvalidDeliveryMethod) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, reward)
}

// HandleListRewards lists all rewards for a campaign
func (h *Handler) HandleListRewards(c *gin.Context) {
	ctx := c.Request.Context()

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	rewards, err := h.processor.ListRewards(ctx, campaignID)
	if err != nil {
		h.logger.Error(ctx, "failed to list rewards", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rewards)
}

// HandleGetReward retrieves a reward by ID
func (h *Handler) HandleGetReward(c *gin.Context) {
	ctx := c.Request.Context()

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get reward ID from path
	rewardIDStr := c.Param("reward_id")
	rewardID, err := uuid.Parse(rewardIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse reward ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reward id"})
		return
	}

	reward, err := h.processor.GetReward(ctx, campaignID, rewardID)
	if err != nil {
		h.logger.Error(ctx, "failed to get reward", err)
		if errors.Is(err, processor.ErrRewardNotFound) || errors.Is(err, processor.ErrUnauthorized) {
			c.JSON(http.StatusNotFound, gin.H{"error": "reward not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reward)
}

// HandleUpdateReward updates a reward
func (h *Handler) HandleUpdateReward(c *gin.Context) {
	ctx := c.Request.Context()

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get reward ID from path
	rewardIDStr := c.Param("reward_id")
	rewardID, err := uuid.Parse(rewardIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse reward ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reward id"})
		return
	}

	var req UpdateRewardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	processorReq := processor.UpdateRewardRequest{
		Name:           req.Name,
		Description:    req.Description,
		Status:         req.Status,
	}

	if req.Config != nil {
		processorReq.Config = *req.Config
	}
	if req.TriggerConfig != nil {
		processorReq.TriggerConfig = *req.TriggerConfig
	}
	if req.DeliveryConfig != nil {
		processorReq.DeliveryConfig = *req.DeliveryConfig
	}

	reward, err := h.processor.UpdateReward(ctx, campaignID, rewardID, processorReq)
	if err != nil {
		h.logger.Error(ctx, "failed to update reward", err)
		if errors.Is(err, processor.ErrRewardNotFound) || errors.Is(err, processor.ErrUnauthorized) {
			c.JSON(http.StatusNotFound, gin.H{"error": "reward not found"})
			return
		}
		if errors.Is(err, processor.ErrInvalidRewardStatus) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reward status"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reward)
}

// HandleDeleteReward deletes a reward
func (h *Handler) HandleDeleteReward(c *gin.Context) {
	ctx := c.Request.Context()

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get reward ID from path
	rewardIDStr := c.Param("reward_id")
	rewardID, err := uuid.Parse(rewardIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse reward ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reward id"})
		return
	}

	err = h.processor.DeleteReward(ctx, campaignID, rewardID)
	if err != nil {
		h.logger.Error(ctx, "failed to delete reward", err)
		if errors.Is(err, processor.ErrRewardNotFound) || errors.Is(err, processor.ErrUnauthorized) {
			c.JSON(http.StatusNotFound, gin.H{"error": "reward not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// HandleGrantReward manually grants a reward to a user
func (h *Handler) HandleGrantReward(c *gin.Context) {
	ctx := c.Request.Context()

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get user ID from path
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse user ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req GrantRewardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	processorReq := processor.GrantRewardRequest{
		RewardID: req.RewardID,
		Reason:   req.Reason,
	}

	userReward, err := h.processor.GrantReward(ctx, campaignID, userID, processorReq)
	if err != nil {
		h.logger.Error(ctx, "failed to grant reward", err)
		if errors.Is(err, processor.ErrRewardNotFound) || errors.Is(err, processor.ErrUnauthorized) {
			c.JSON(http.StatusNotFound, gin.H{"error": "reward not found"})
			return
		}
		if errors.Is(err, processor.ErrInvalidRewardStatus) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "reward is not active"})
			return
		}
		if errors.Is(err, processor.ErrRewardLimitReached) {
			c.JSON(http.StatusConflict, gin.H{"error": "reward limit reached"})
			return
		}
		if errors.Is(err, processor.ErrUserLimitReached) {
			c.JSON(http.StatusConflict, gin.H{"error": "user has already claimed maximum rewards"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, userReward)
}

// HandleGetUserRewards retrieves all rewards earned by a user
func (h *Handler) HandleGetUserRewards(c *gin.Context) {
	ctx := c.Request.Context()

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get user ID from path
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse user ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	rewards, err := h.processor.GetUserRewards(ctx, campaignID, userID)
	if err != nil {
		h.logger.Error(ctx, "failed to get user rewards", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return empty array instead of null
	if rewards == nil {
		rewards = []store.UserReward{}
	}

	c.JSON(http.StatusOK, rewards)
}
