package handler

import (
	"errors"
	"net/http"
	"strconv"

	"base-server/internal/apierrors"
	"base-server/internal/observability"
	"base-server/internal/webhooks/processor"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler handles webhook HTTP requests
type Handler struct {
	processor *processor.WebhookProcessor
	logger    *observability.Logger
}

// New creates a new Handler
func New(processor *processor.WebhookProcessor, logger *observability.Logger) *Handler {
	return &Handler{
		processor: processor,
		logger:    logger,
	}
}

// handleError maps processor errors to appropriate API error responses
func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, processor.ErrWebhooksNotAvailable):
		apierrors.Forbidden(c, "FEATURE_NOT_AVAILABLE", "Webhooks are not available in your plan. Please upgrade to Team plan.")
	default:
		apierrors.InternalError(c, err)
	}
}

// CreateWebhookRequest represents a request to create a webhook
type CreateWebhookRequest struct {
	URL          string   `json:"url" binding:"required,url"`
	CampaignID   *string  `json:"campaign_id"`
	Events       []string `json:"events" binding:"required,min=1"`
	RetryEnabled bool     `json:"retry_enabled"`
	MaxRetries   int      `json:"max_retries"`
}

// CreateWebhookResponse represents the response for creating a webhook
type CreateWebhookResponse struct {
	Webhook interface{} `json:"webhook"`
	Secret  string      `json:"secret"`
}

// HandleCreateWebhook handles POST /api/v1/webhooks
func (h *Handler) HandleCreateWebhook(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context (set by auth middleware)
	accountID := c.MustGet("Account-ID")
	parsedAccountID := uuid.MustParse(accountID.(string))
	ctx = observability.WithFields(ctx, observability.Field{Key: "account_id", Value: parsedAccountID})

	var req CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	// Parse campaign ID if provided
	var campaignID *uuid.UUID
	if req.CampaignID != nil && *req.CampaignID != "" {
		parsed, err := uuid.Parse(*req.CampaignID)
		if err != nil {
			h.logger.Error(ctx, "failed to parse campaign_id", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign_id"})
			return
		}
		campaignID = &parsed
	}

	// Set default retry enabled to true if not specified
	if req.MaxRetries == 0 {
		req.RetryEnabled = true
	}

	// Create webhook
	webhook, secret, err := h.processor.CreateWebhook(ctx, processor.CreateWebhookParams{
		AccountID:    parsedAccountID,
		CampaignID:   campaignID,
		URL:          req.URL,
		Events:       req.Events,
		RetryEnabled: req.RetryEnabled,
		MaxRetries:   req.MaxRetries,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, CreateWebhookResponse{
		Webhook: webhook,
		Secret:  secret,
	})
}

// HandleListWebhooks handles GET /api/v1/webhooks
func (h *Handler) HandleListWebhooks(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountID := c.MustGet("Account-ID")
	parsedAccountID := uuid.MustParse(accountID.(string))
	ctx = observability.WithFields(ctx, observability.Field{Key: "account_id", Value: parsedAccountID})

	// Check if filtering by campaign
	campaignIDStr := c.Query("campaign_id")
	if campaignIDStr != "" {
		campaignID, err := uuid.Parse(campaignIDStr)
		if err != nil {
			h.logger.Error(ctx, "failed to parse campaign_id", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign_id"})
			return
		}

		webhooks, err := h.processor.GetWebhooksByCampaign(ctx, campaignID)
		if err != nil {
			h.handleError(c, err)
			return
		}

		c.JSON(http.StatusOK, webhooks)
		return
	}

	// Get all webhooks for account
	webhooks, err := h.processor.GetWebhooksByAccount(ctx, parsedAccountID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, webhooks)
}

// HandleGetWebhook handles GET /api/v1/webhooks/:webhook_id
func (h *Handler) HandleGetWebhook(c *gin.Context) {
	ctx := c.Request.Context()

	webhookID, err := uuid.Parse(c.Param("webhook_id"))
	if err != nil {
		h.logger.Error(ctx, "failed to parse webhook_id", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook_id"})
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "webhook_id", Value: webhookID})

	webhook, err := h.processor.GetWebhook(ctx, webhookID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, webhook)
}

// UpdateWebhookRequest represents a request to update a webhook
type UpdateWebhookRequest struct {
	URL          *string  `json:"url"`
	Events       []string `json:"events"`
	Status       *string  `json:"status"`
	RetryEnabled *bool    `json:"retry_enabled"`
	MaxRetries   *int     `json:"max_retries"`
}

// HandleUpdateWebhook handles PUT /api/v1/webhooks/:webhook_id
func (h *Handler) HandleUpdateWebhook(c *gin.Context) {
	ctx := c.Request.Context()

	webhookID, err := uuid.Parse(c.Param("webhook_id"))
	if err != nil {
		h.logger.Error(ctx, "failed to parse webhook_id", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook_id"})
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "webhook_id", Value: webhookID})

	var req UpdateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	webhook, err := h.processor.UpdateWebhook(ctx, webhookID, processor.UpdateWebhookParams{
		URL:          req.URL,
		Events:       req.Events,
		Status:       req.Status,
		RetryEnabled: req.RetryEnabled,
		MaxRetries:   req.MaxRetries,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, webhook)
}

// HandleDeleteWebhook handles DELETE /api/v1/webhooks/:webhook_id
func (h *Handler) HandleDeleteWebhook(c *gin.Context) {
	ctx := c.Request.Context()

	webhookID, err := uuid.Parse(c.Param("webhook_id"))
	if err != nil {
		h.logger.Error(ctx, "failed to parse webhook_id", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook_id"})
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "webhook_id", Value: webhookID})

	err = h.processor.DeleteWebhook(ctx, webhookID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// HandleListWebhookDeliveries handles GET /api/v1/webhooks/:webhook_id/deliveries
func (h *Handler) HandleListWebhookDeliveries(c *gin.Context) {
	ctx := c.Request.Context()

	webhookID, err := uuid.Parse(c.Param("webhook_id"))
	if err != nil {
		h.logger.Error(ctx, "failed to parse webhook_id", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook_id"})
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "webhook_id", Value: webhookID})

	// Parse pagination parameters
	limit := 20
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			offset = (page - 1) * limit
		}
	}

	deliveries, err := h.processor.GetWebhookDeliveries(ctx, webhookID, limit, offset)
	if err != nil {
		h.handleError(c, err)
		return
	}

	page := (offset / limit) + 1
	c.JSON(http.StatusOK, gin.H{
		"deliveries": deliveries,
		"pagination": gin.H{
			"page":   page,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// TestWebhookRequest represents a request to test a webhook
type TestWebhookRequest struct {
	EventType string `json:"event_type"`
}

// HandleTestWebhook handles POST /api/v1/webhooks/:webhook_id/test
func (h *Handler) HandleTestWebhook(c *gin.Context) {
	ctx := c.Request.Context()

	webhookID, err := uuid.Parse(c.Param("webhook_id"))
	if err != nil {
		h.logger.Error(ctx, "failed to parse webhook_id", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook_id"})
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "webhook_id", Value: webhookID})

	err = h.processor.TestWebhook(ctx, webhookID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Test webhook sent successfully"})
}
