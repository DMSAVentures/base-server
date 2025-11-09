package processor

import (
	"context"
	"fmt"

	"base-server/internal/observability"
	"base-server/internal/webhooks/service"
	"base-server/internal/workers"

	"github.com/google/uuid"
)

// WebhookEventProcessor implements the EventProcessor interface for webhook events.
// It dispatches events to subscribed webhooks via the WebhookService.
type WebhookEventProcessor struct {
	webhookService *service.WebhookService
	logger         *observability.Logger
}

// NewWebhookEventProcessor creates a new webhook event processor.
func NewWebhookEventProcessor(
	webhookService *service.WebhookService,
	logger *observability.Logger,
) workers.EventProcessor {
	return &WebhookEventProcessor{
		webhookService: webhookService,
		logger:         logger,
	}
}

// Name returns the processor name for logging and metrics.
func (p *WebhookEventProcessor) Name() string {
	return "webhook"
}

// Process handles a single webhook event from Kafka.
// It parses the event and dispatches it to all subscribed webhooks.
// Returns an error if processing fails, which prevents offset commit and enables replay.
func (p *WebhookEventProcessor) Process(ctx context.Context, event workers.EventMessage) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "event_id", Value: event.ID},
		observability.Field{Key: "event_type", Value: event.Type},
		observability.Field{Key: "account_id", Value: event.AccountID},
	)

	p.logger.Info(ctx, fmt.Sprintf("Processing webhook event %s", event.Type))

	// Parse account ID
	accountID, err := uuid.Parse(event.AccountID)
	if err != nil {
		p.logger.Error(ctx, "Invalid account_id in event", err)
		// Return error to prevent offset commit - event will be replayed
		return fmt.Errorf("invalid account_id: %w", err)
	}

	// Parse campaign ID if present
	var campaignID *uuid.UUID
	if event.CampaignID != nil {
		parsed, err := uuid.Parse(*event.CampaignID)
		if err != nil {
			p.logger.Error(ctx, "Invalid campaign_id in event", err)
			// Return error to prevent offset commit - event will be replayed
			return fmt.Errorf("invalid campaign_id: %w", err)
		}
		campaignID = &parsed
	}

	// Dispatch event to subscribed webhooks
	// This will:
	// 1. Find all webhooks subscribed to this event type for this account
	// 2. Create webhook delivery attempts for each
	// 3. Attempt immediate delivery (with retries on failure via webhook worker)
	err = p.webhookService.DispatchEvent(ctx, accountID, campaignID, event.Type, event.Data)
	if err != nil {
		p.logger.Error(ctx, "Failed to dispatch event to webhooks", err)
		// Return error to prevent offset commit - event will be replayed
		return fmt.Errorf("failed to dispatch event to webhooks: %w", err)
	}

	p.logger.Info(ctx, fmt.Sprintf("Successfully processed webhook event %s", event.Type))
	return nil
}
