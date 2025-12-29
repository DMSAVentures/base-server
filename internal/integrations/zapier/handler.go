package zapier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"base-server/internal/apierrors"
	"base-server/internal/integrations"
	"base-server/internal/observability"
	"base-server/internal/store"
	"base-server/internal/tiers"
	webhookEvents "base-server/internal/webhooks/events"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CampaignStore defines the interface for campaign-related database operations
type CampaignStore interface {
	ListCampaigns(ctx context.Context, params store.ListCampaignsParams) (store.ListCampaignsResult, error)
}

// APIKeyStore defines the interface for API key operations
type APIKeyStore interface {
	GetAPIKeyByHash(ctx context.Context, keyHash string) (store.APIKey, error)
	UpdateAPIKeyUsage(ctx context.Context, keyID uuid.UUID) error
}

// ZapierStore combines integration, campaign, and API key store interfaces
type ZapierStore interface {
	integrations.IntegrationStore
	CampaignStore
	APIKeyStore
}

// Handler handles Zapier-specific REST Hook endpoints
type Handler struct {
	store       ZapierStore
	tierService *tiers.TierService
	logger      *observability.Logger
}

// NewHandler creates a new Zapier handler
func NewHandler(store ZapierStore, tierService *tiers.TierService, logger *observability.Logger) *Handler {
	return &Handler{
		store:       store,
		tierService: tierService,
		logger:      logger,
	}
}

// handleError maps errors to appropriate API responses
func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		apierrors.NotFound(c, "Subscription not found")
	default:
		apierrors.InternalError(c, err)
	}
}

// APIKeyMiddleware validates API key authentication for Zapier endpoints
func (h *Handler) APIKeyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing API key",
			})
			return
		}

		// Hash the API key to look it up
		hash := sha256.Sum256([]byte(apiKey))
		keyHash := hex.EncodeToString(hash[:])

		// Look up the API key
		key, err := h.store.GetAPIKeyByHash(ctx, keyHash)
		if err != nil {
			h.logger.InfoWithError(ctx, "invalid API key", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid API key",
			})
			return
		}

		// Check if key has zapier scope
		hasZapierScope := false
		for _, scope := range key.Scopes {
			if scope == "zapier" || scope == "*" || scope == "all" {
				hasZapierScope = true
				break
			}
		}

		if !hasZapierScope {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "API key does not have Zapier access",
			})
			return
		}

		// Update usage stats (fire and forget)
		go func() {
			_ = h.store.UpdateAPIKeyUsage(context.Background(), key.ID)
		}()

		// Set account info in context
		c.Set("Account-ID", key.AccountID.String())
		c.Set("API-Key-ID", key.ID.String())

		ctx = observability.WithFields(ctx,
			observability.Field{Key: "account_id", Value: key.AccountID.String()},
			observability.Field{Key: "api_key_id", Value: key.ID.String()},
		)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// hashAPIKey creates a SHA256 hash of the API key
func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(apiKey)))
	return hex.EncodeToString(hash[:])
}

// SubscribeRequest represents a Zapier REST Hook subscription request
type SubscribeRequest struct {
	HookURL    string  `json:"hookUrl" binding:"required,url"`
	Event      string  `json:"event" binding:"required"`
	CampaignID *string `json:"campaign_id,omitempty"`
}

// SubscribeResponse represents the subscription response
type SubscribeResponse struct {
	ID string `json:"id"`
}

// AccountInfo represents the /me response
type AccountInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// HandleMe handles GET /api/v1/zapier/me
// This is used by Zapier to test the authentication
func (h *Handler) HandleMe(c *gin.Context) {
	accountIDStr, _ := c.Get("Account-ID")
	accountID := accountIDStr.(string)

	// Return basic account info for Zapier to verify auth
	c.JSON(http.StatusOK, AccountInfo{
		ID:   accountID,
		Name: "Connected Account",
	})
}

// HandleSubscribe handles POST /api/v1/zapier/subscribe
// Zapier calls this to subscribe to events (REST Hook pattern)
func (h *Handler) HandleSubscribe(c *gin.Context) {
	ctx := c.Request.Context()

	accountIDStr, _ := c.Get("Account-ID")
	accountID, _ := uuid.Parse(accountIDStr.(string))

	// Check if account has Zapier feature
	hasFeature, err := h.tierService.HasFeatureByAccountID(ctx, accountID, "webhooks_zapier")
	if err != nil {
		h.logger.Error(ctx, "failed to check Zapier feature", err)
		h.handleError(c, err)
		return
	}
	if !hasFeature {
		apierrors.Forbidden(c, "FEATURE_NOT_AVAILABLE", "Zapier integration is not available in your plan. Please upgrade to Team plan.")
		return
	}

	var req SubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	apiKeyIDStr, _ := c.Get("API-Key-ID")
	apiKeyID, _ := uuid.Parse(apiKeyIDStr.(string))

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "event_type", Value: req.Event},
		observability.Field{Key: "target_url", Value: req.HookURL},
	)

	// Validate event type
	if !isValidEventType(req.Event) {
		h.logger.Warn(ctx, "invalid event type for subscription")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event type"})
		return
	}

	// Parse campaign ID if provided
	var campaignID *uuid.UUID
	if req.CampaignID != nil && *req.CampaignID != "" {
		parsed, err := uuid.Parse(*req.CampaignID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign_id"})
			return
		}
		campaignID = &parsed
	}

	// Create subscription
	subscription, err := h.store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
		AccountID:       accountID,
		APIKeyID:        &apiKeyID,
		IntegrationType: integrations.IntegrationZapier,
		TargetURL:       req.HookURL,
		EventType:       req.Event,
		CampaignID:      campaignID,
	})
	if err != nil {
		h.logger.Error(ctx, "failed to create subscription", err)
		h.handleError(c, err)
		return
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "subscription_id", Value: subscription.ID.String()})
	h.logger.Info(ctx, "created Zapier subscription")

	c.JSON(http.StatusCreated, SubscribeResponse{
		ID: subscription.ID.String(),
	})
}

// HandleUnsubscribe handles DELETE /api/v1/zapier/subscribe/:id
// Zapier calls this to unsubscribe from events
func (h *Handler) HandleUnsubscribe(c *gin.Context) {
	ctx := c.Request.Context()

	subscriptionIDStr := c.Param("id")
	subscriptionID, err := uuid.Parse(subscriptionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription id"})
		return
	}

	accountIDStr, _ := c.Get("Account-ID")
	accountID, _ := uuid.Parse(accountIDStr.(string))

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "subscription_id", Value: subscriptionID.String()},
	)

	// Verify subscription belongs to account
	subscription, err := h.store.GetIntegrationSubscriptionByID(ctx, subscriptionID)
	if err != nil {
		h.logger.Error(ctx, "failed to get subscription", err)
		h.handleError(c, err)
		return
	}

	if subscription.AccountID != accountID {
		c.JSON(http.StatusForbidden, gin.H{"error": "subscription not found"})
		return
	}

	// Delete subscription
	err = h.store.DeleteIntegrationSubscription(ctx, subscriptionID)
	if err != nil {
		h.logger.Error(ctx, "failed to delete subscription", err)
		h.handleError(c, err)
		return
	}

	h.logger.Info(ctx, "deleted Zapier subscription")

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// HandleSampleData handles GET /api/v1/zapier/sample/:event
// Returns sample data for Zapier to use when setting up Zaps
func (h *Handler) HandleSampleData(c *gin.Context) {
	eventType := c.Param("event")

	sample := getSampleDataForEvent(eventType)
	if sample == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown event type"})
		return
	}

	// Zapier expects an array of sample objects
	c.JSON(http.StatusOK, []map[string]interface{}{sample})
}

// CampaignListItem represents a campaign item for Zapier dropdown
type CampaignListItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// HandleListCampaigns handles GET /api/v1/zapier/campaigns
// Returns campaigns for dropdown selection in Zapier
func (h *Handler) HandleListCampaigns(c *gin.Context) {
	ctx := c.Request.Context()

	accountIDStr, _ := c.Get("Account-ID")
	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
	)

	// List all active campaigns for this account
	result, err := h.store.ListCampaigns(ctx, store.ListCampaignsParams{
		AccountID: accountID,
		Page:      1,
		Limit:     100, // Reasonable limit for dropdown
	})
	if err != nil {
		h.logger.Error(ctx, "failed to list campaigns for Zapier", err)
		h.handleError(c, err)
		return
	}

	// Map to simplified format for Zapier dropdown
	campaigns := make([]CampaignListItem, len(result.Campaigns))
	for i, campaign := range result.Campaigns {
		campaigns[i] = CampaignListItem{
			ID:   campaign.ID.String(),
			Name: campaign.Name,
		}
	}

	c.JSON(http.StatusOK, campaigns)
}

// isValidEventType checks if an event type is valid for subscription
func isValidEventType(eventType string) bool {
	validEvents := []string{
		webhookEvents.EventUserCreated,
		webhookEvents.EventUserUpdated,
		webhookEvents.EventUserVerified,
		webhookEvents.EventUserDeleted,
		webhookEvents.EventUserPositionChanged,
		webhookEvents.EventUserConverted,
		webhookEvents.EventReferralCreated,
		webhookEvents.EventReferralVerified,
		webhookEvents.EventReferralConverted,
		webhookEvents.EventRewardEarned,
		webhookEvents.EventRewardDelivered,
		webhookEvents.EventRewardRedeemed,
		webhookEvents.EventCampaignMilestone,
		webhookEvents.EventCampaignLaunched,
		webhookEvents.EventCampaignCompleted,
		webhookEvents.EventEmailSent,
		webhookEvents.EventEmailDelivered,
		webhookEvents.EventEmailOpened,
		webhookEvents.EventEmailClicked,
		webhookEvents.EventEmailBounced,
	}

	for _, valid := range validEvents {
		if eventType == valid {
			return true
		}
	}
	return false
}

// ===== Management Endpoints (JWT authenticated, for the UI) =====

// StatusResponse represents the Zapier connection status
type StatusResponse struct {
	Connected           bool `json:"connected"`
	ActiveSubscriptions int  `json:"active_subscriptions"`
}

// SubscriptionResponse represents a subscription for the UI
type SubscriptionResponse struct {
	ID              string  `json:"id"`
	EventType       string  `json:"event_type"`
	CampaignID      *string `json:"campaign_id,omitempty"`
	Status          string  `json:"status"`
	TriggerCount    int     `json:"trigger_count"`
	LastTriggeredAt *string `json:"last_triggered_at,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

// HandleStatus returns the Zapier connection status for the account
// GET /api/protected/integrations/zapier/status
func (h *Handler) HandleStatus(c *gin.Context) {
	ctx := c.Request.Context()

	accountIDStr := c.GetString("Account-ID")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
	)

	// Get Zapier subscriptions for the account
	intType := integrations.IntegrationZapier
	subscriptions, err := h.store.GetIntegrationSubscriptionsByAccount(ctx, accountID, &intType)
	if err != nil {
		h.logger.Error(ctx, "failed to get subscriptions", err)
		h.handleError(c, err)
		return
	}

	// Account is "connected" if there are any active Zapier subscriptions
	connected := len(subscriptions) > 0

	c.JSON(http.StatusOK, StatusResponse{
		Connected:           connected,
		ActiveSubscriptions: len(subscriptions),
	})
}

// HandleSubscriptions returns the list of Zapier subscriptions for the account
// GET /api/protected/integrations/zapier/subscriptions
func (h *Handler) HandleSubscriptions(c *gin.Context) {
	ctx := c.Request.Context()

	accountIDStr := c.GetString("Account-ID")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
	)

	intType := integrations.IntegrationZapier
	subscriptions, err := h.store.GetIntegrationSubscriptionsByAccount(ctx, accountID, &intType)
	if err != nil {
		h.logger.Error(ctx, "failed to get subscriptions", err)
		h.handleError(c, err)
		return
	}

	result := make([]SubscriptionResponse, len(subscriptions))
	for i, sub := range subscriptions {
		var campaignID *string
		if sub.CampaignID != nil {
			id := sub.CampaignID.String()
			campaignID = &id
		}

		var lastTriggeredAt *string
		if sub.LastTriggeredAt != nil {
			t := sub.LastTriggeredAt.Format(time.RFC3339)
			lastTriggeredAt = &t
		}

		result[i] = SubscriptionResponse{
			ID:              sub.ID.String(),
			EventType:       sub.EventType,
			CampaignID:      campaignID,
			Status:          sub.Status,
			TriggerCount:    sub.TriggerCount,
			LastTriggeredAt: lastTriggeredAt,
			CreatedAt:       sub.CreatedAt.Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, result)
}

// HandleDisconnect deletes all Zapier subscriptions for the account
// POST /api/protected/integrations/zapier/disconnect
func (h *Handler) HandleDisconnect(c *gin.Context) {
	ctx := c.Request.Context()

	accountIDStr := c.GetString("Account-ID")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
	)

	// Get all Zapier subscriptions for this account
	intType := integrations.IntegrationZapier
	subscriptions, err := h.store.GetIntegrationSubscriptionsByAccount(ctx, accountID, &intType)
	if err != nil {
		h.logger.Error(ctx, "failed to get subscriptions", err)
		h.handleError(c, err)
		return
	}

	// Delete each subscription
	for _, sub := range subscriptions {
		if err := h.store.DeleteIntegrationSubscription(ctx, sub.ID); err != nil {
			h.logger.Error(ctx, "failed to delete subscription", err)
			// Continue with other subscriptions
		}
	}

	h.logger.Info(ctx, "disconnected Zapier integration")

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// getSampleDataForEvent returns sample data for a given event type
// Returns nil for unknown event types
func getSampleDataForEvent(eventType string) map[string]interface{} {
	// Check if event type is valid first
	if !isValidEventType(eventType) {
		return nil
	}

	basePayload := map[string]interface{}{
		"id":          "evt_" + uuid.New().String()[:8],
		"event":       eventType,
		"occurred_at": time.Now().UTC().Format(time.RFC3339),
		"account_id":  "acc_" + uuid.New().String()[:8],
		"campaign_id": "cmp_" + uuid.New().String()[:8],
	}

	switch eventType {
	case webhookEvents.EventUserCreated, webhookEvents.EventUserVerified, webhookEvents.EventUserUpdated:
		basePayload["data"] = map[string]interface{}{
			"user": map[string]interface{}{
				"id":             "usr_" + uuid.New().String()[:8],
				"email":          "john.doe@example.com",
				"first_name":     "John",
				"last_name":      "Doe",
				"position":       42,
				"referral_code":  "JOHN123",
				"referral_count": 5,
				"points":         150,
				"status":         "verified",
				"created_at":     time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339),
			},
		}

	case webhookEvents.EventUserPositionChanged:
		basePayload["data"] = map[string]interface{}{
			"user": map[string]interface{}{
				"id":       "usr_" + uuid.New().String()[:8],
				"email":    "john.doe@example.com",
				"position": 35,
			},
			"old_position": 42,
			"new_position": 35,
		}

	case webhookEvents.EventReferralCreated, webhookEvents.EventReferralVerified:
		basePayload["data"] = map[string]interface{}{
			"referral": map[string]interface{}{
				"id":           "ref_" + uuid.New().String()[:8],
				"referrer_id":  "usr_" + uuid.New().String()[:8],
				"referred_id":  "usr_" + uuid.New().String()[:8],
				"status":       "verified",
				"points_award": 25,
				"created_at":   time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339),
			},
		}

	case webhookEvents.EventRewardEarned, webhookEvents.EventRewardDelivered:
		basePayload["data"] = map[string]interface{}{
			"reward": map[string]interface{}{
				"id":          "rwd_" + uuid.New().String()[:8],
				"name":        "Early Access",
				"description": "Get early access to the product",
				"type":        "access",
			},
			"user": map[string]interface{}{
				"id":    "usr_" + uuid.New().String()[:8],
				"email": "john.doe@example.com",
			},
		}

	case webhookEvents.EventCampaignMilestone:
		basePayload["data"] = map[string]interface{}{
			"milestone":     1000,
			"total_signups": 1000,
		}

	case webhookEvents.EventEmailSent, webhookEvents.EventEmailOpened, webhookEvents.EventEmailClicked:
		basePayload["data"] = map[string]interface{}{
			"email": map[string]interface{}{
				"id":        "eml_" + uuid.New().String()[:8],
				"to":        "john.doe@example.com",
				"subject":   "Welcome to the waitlist!",
				"template":  "welcome",
				"opened_at": time.Now().UTC().Format(time.RFC3339),
			},
		}

	default:
		// For valid events without specific sample data, return generic data
		basePayload["data"] = map[string]interface{}{}
	}

	return basePayload
}
