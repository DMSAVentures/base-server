package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"base-server/internal/webhooks/service"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
)

// WebhookProcessor handles webhook business logic
type WebhookProcessor struct {
	store          store.Store
	logger         *observability.Logger
	webhookService *service.WebhookService
}

// New creates a new WebhookProcessor
func New(store store.Store, logger *observability.Logger, webhookService *service.WebhookService) *WebhookProcessor {
	return &WebhookProcessor{
		store:          store,
		logger:         logger,
		webhookService: webhookService,
	}
}

// CreateWebhookParams represents parameters for creating a webhook
type CreateWebhookParams struct {
	AccountID    uuid.UUID
	CampaignID   *uuid.UUID
	URL          string
	Events       []string
	RetryEnabled bool
	MaxRetries   int
}

// CreateWebhook creates a new webhook
func (p *WebhookProcessor) CreateWebhook(ctx context.Context, params CreateWebhookParams) (store.Webhook, string, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: params.AccountID},
		observability.Field{Key: "url", Value: params.URL},
	)

	// Generate a secret for HMAC signing
	secret, err := p.generateSecret()
	if err != nil {
		p.logger.Error(ctx, "failed to generate secret", err)
		return store.Webhook{}, "", fmt.Errorf("failed to generate secret: %w", err)
	}

	// Validate events
	if len(params.Events) == 0 {
		return store.Webhook{}, "", fmt.Errorf("at least one event must be specified")
	}

	for _, event := range params.Events {
		if !p.isValidEvent(event) {
			return store.Webhook{}, "", fmt.Errorf("invalid event type: %s", event)
		}
	}

	// Set default values
	if params.MaxRetries == 0 {
		params.MaxRetries = 5
	}

	// Create webhook in database
	webhook, err := p.store.CreateWebhook(ctx, store.CreateWebhookParams{
		AccountID:    params.AccountID,
		CampaignID:   params.CampaignID,
		URL:          params.URL,
		Secret:       secret,
		Events:       params.Events,
		RetryEnabled: params.RetryEnabled,
		MaxRetries:   params.MaxRetries,
	})
	if err != nil {
		p.logger.Error(ctx, "failed to create webhook", err)
		return store.Webhook{}, "", fmt.Errorf("failed to create webhook: %w", err)
	}

	p.logger.Info(ctx, fmt.Sprintf("created webhook %s", webhook.ID))

	// Return webhook and secret (secret is only returned once)
	return webhook, secret, nil
}

// UpdateWebhookParams represents parameters for updating a webhook
type UpdateWebhookParams struct {
	URL          *string
	Events       []string
	Status       *string
	RetryEnabled *bool
	MaxRetries   *int
}

// UpdateWebhook updates an existing webhook
func (p *WebhookProcessor) UpdateWebhook(ctx context.Context, webhookID uuid.UUID, params UpdateWebhookParams) (store.Webhook, error) {
	ctx = observability.WithFields(ctx, observability.Field{Key: "webhook_id", Value: webhookID})

	// Validate events if provided
	if len(params.Events) > 0 {
		for _, event := range params.Events {
			if !p.isValidEvent(event) {
				return store.Webhook{}, fmt.Errorf("invalid event type: %s", event)
			}
		}
	}

	// Validate status if provided
	if params.Status != nil {
		if !p.isValidStatus(*params.Status) {
			return store.Webhook{}, fmt.Errorf("invalid status: %s", *params.Status)
		}
	}

	// Update webhook in database
	webhook, err := p.store.UpdateWebhook(ctx, webhookID, store.UpdateWebhookParams{
		URL:          params.URL,
		Events:       params.Events,
		Status:       params.Status,
		RetryEnabled: params.RetryEnabled,
		MaxRetries:   params.MaxRetries,
	})
	if err != nil {
		p.logger.Error(ctx, "failed to update webhook", err)
		return store.Webhook{}, fmt.Errorf("failed to update webhook: %w", err)
	}

	p.logger.Info(ctx, fmt.Sprintf("updated webhook %s", webhook.ID))

	return webhook, nil
}

// GetWebhook retrieves a webhook by ID
func (p *WebhookProcessor) GetWebhook(ctx context.Context, webhookID uuid.UUID) (store.Webhook, error) {
	webhook, err := p.store.GetWebhookByID(ctx, webhookID)
	if err != nil {
		p.logger.Error(ctx, "failed to get webhook", err)
		return store.Webhook{}, fmt.Errorf("failed to get webhook: %w", err)
	}
	return webhook, nil
}

// GetWebhooksByAccount retrieves all webhooks for an account
func (p *WebhookProcessor) GetWebhooksByAccount(ctx context.Context, accountID uuid.UUID) ([]store.Webhook, error) {
	webhooks, err := p.store.GetWebhooksByAccount(ctx, accountID)
	if err != nil {
		p.logger.Error(ctx, "failed to get webhooks", err)
		return nil, fmt.Errorf("failed to get webhooks: %w", err)
	}
	return webhooks, nil
}

// GetWebhooksByCampaign retrieves all webhooks for a campaign
func (p *WebhookProcessor) GetWebhooksByCampaign(ctx context.Context, campaignID uuid.UUID) ([]store.Webhook, error) {
	webhooks, err := p.store.GetWebhooksByCampaign(ctx, campaignID)
	if err != nil {
		p.logger.Error(ctx, "failed to get webhooks for campaign", err)
		return nil, fmt.Errorf("failed to get webhooks for campaign: %w", err)
	}
	return webhooks, nil
}

// DeleteWebhook deletes a webhook
func (p *WebhookProcessor) DeleteWebhook(ctx context.Context, webhookID uuid.UUID) error {
	ctx = observability.WithFields(ctx, observability.Field{Key: "webhook_id", Value: webhookID})

	err := p.store.DeleteWebhook(ctx, webhookID)
	if err != nil {
		p.logger.Error(ctx, "failed to delete webhook", err)
		return fmt.Errorf("failed to delete webhook: %w", err)
	}

	p.logger.Info(ctx, fmt.Sprintf("deleted webhook %s", webhookID))

	return nil
}

// GetWebhookDeliveries retrieves webhook delivery history
func (p *WebhookProcessor) GetWebhookDeliveries(ctx context.Context, webhookID uuid.UUID, limit, offset int) ([]store.WebhookDelivery, error) {
	deliveries, err := p.store.GetWebhookDeliveriesByWebhook(ctx, webhookID, limit, offset)
	if err != nil {
		p.logger.Error(ctx, "failed to get webhook deliveries", err)
		return nil, fmt.Errorf("failed to get webhook deliveries: %w", err)
	}
	return deliveries, nil
}

// TestWebhook sends a test event to a webhook
func (p *WebhookProcessor) TestWebhook(ctx context.Context, webhookID uuid.UUID) error {
	ctx = observability.WithFields(ctx, observability.Field{Key: "webhook_id", Value: webhookID})

	err := p.webhookService.TestWebhook(ctx, webhookID)
	if err != nil {
		p.logger.Error(ctx, "failed to test webhook", err)
		return fmt.Errorf("failed to test webhook: %w", err)
	}

	p.logger.Info(ctx, fmt.Sprintf("test webhook sent to %s", webhookID))

	return nil
}

// generateSecret generates a random secret for HMAC signing
func (p *WebhookProcessor) generateSecret() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// isValidEvent checks if an event type is valid
func (p *WebhookProcessor) isValidEvent(eventType string) bool {
	validEvents := []string{
		"user.created",
		"user.updated",
		"user.verified",
		"user.deleted",
		"user.position_changed",
		"user.converted",
		"referral.created",
		"referral.verified",
		"referral.converted",
		"reward.earned",
		"reward.delivered",
		"reward.redeemed",
		"campaign.milestone",
		"campaign.launched",
		"campaign.completed",
		"email.sent",
		"email.delivered",
		"email.opened",
		"email.clicked",
		"email.bounced",
	}

	for _, valid := range validEvents {
		if eventType == valid {
			return true
		}
	}
	return false
}

// isValidStatus checks if a status is valid
func (p *WebhookProcessor) isValidStatus(status string) bool {
	return status == "active" || status == "paused" || status == "failed"
}
