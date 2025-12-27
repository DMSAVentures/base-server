package consumer

import (
	"context"
	"fmt"
	"time"

	"base-server/internal/integrations"
	"base-server/internal/integrations/service"
	"base-server/internal/observability"
	"base-server/internal/workers"

	"github.com/google/uuid"
)

// IntegrationEventProcessor implements the EventProcessor interface for integration events.
// It dispatches events to subscribed integration services (Zapier, Slack, etc.)
type IntegrationEventProcessor struct {
	integrationService *service.IntegrationService
	store              integrations.IntegrationStore
	logger             *observability.Logger
}

// NewIntegrationEventProcessor creates a new integration event processor.
func NewIntegrationEventProcessor(
	integrationService *service.IntegrationService,
	store integrations.IntegrationStore,
	logger *observability.Logger,
) workers.EventProcessor {
	return &IntegrationEventProcessor{
		integrationService: integrationService,
		store:              store,
		logger:             logger,
	}
}

// Name returns the processor name for logging and metrics.
func (p *IntegrationEventProcessor) Name() string {
	return "integrations"
}

// Process handles a single event from Kafka.
// It parses the event and dispatches it to all subscribed integrations.
func (p *IntegrationEventProcessor) Process(ctx context.Context, event workers.EventMessage) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "event_id", Value: event.ID},
		observability.Field{Key: "event_type", Value: event.Type},
		observability.Field{Key: "account_id", Value: event.AccountID},
		observability.Field{Key: "processor", Value: "integrations"},
	)

	p.logger.Info(ctx, fmt.Sprintf("Processing integration event %s", event.Type))

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

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, event.Timestamp)
	if err != nil {
		// Use current time if parsing fails
		timestamp = time.Now().UTC()
	}

	// Create integration event
	integrationEvent := integrations.Event{
		ID:         event.ID,
		Type:       event.Type,
		AccountID:  accountID,
		CampaignID: campaignID,
		Data:       event.Data,
		Timestamp:  timestamp,
	}

	// Dispatch event to all subscribed integrations
	// This will:
	// 1. Find all integration subscriptions for this event type and account
	// 2. Create delivery records for each
	// 3. Deliver to each integration (Zapier, Slack, etc.) asynchronously
	err = p.integrationService.DeliverEvent(ctx, integrationEvent)
	if err != nil {
		p.logger.Error(ctx, "Failed to deliver event to integrations", err)
		// We don't return error here to avoid blocking the queue
		// Individual delivery failures are tracked per subscription
		// This prevents one failing integration from blocking all others
	}

	p.logger.Info(ctx, fmt.Sprintf("Successfully processed integration event %s", event.Type))
	return nil
}
