package position

import (
	"base-server/internal/observability"
	"base-server/internal/waitlist/processor"
	"base-server/internal/webhooks/events"
	"base-server/internal/workers"
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Processor handles position calculation events from Kafka
type Processor struct {
	positionCalculator *processor.PositionCalculator
	logger             *observability.Logger
}

// NewProcessor creates a new position calculation event processor
func NewProcessor(positionCalculator *processor.PositionCalculator, logger *observability.Logger) *Processor {
	return &Processor{
		positionCalculator: positionCalculator,
		logger:             logger,
	}
}

// Process handles user.created and user.verified events to trigger position calculation
// Uses formula-based calculation for single user (LOCK-FREE, single row UPDATE)
func (p *Processor) Process(ctx context.Context, event workers.EventMessage) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "event_id", Value: event.ID},
		observability.Field{Key: "event_type", Value: event.Type},
		observability.Field{Key: "account_id", Value: event.AccountID},
	)

	// Only process user.created and user.verified events
	if event.Type != events.EventUserCreated && event.Type != events.EventUserVerified {
		// Silently skip non-relevant events (not an error)
		return nil
	}

	// Extract campaign_id from event data
	campaignIDStr, ok := event.Data["campaign_id"].(string)
	if !ok || campaignIDStr == "" {
		p.logger.Error(ctx, "event missing campaign_id", fmt.Errorf("invalid or missing campaign_id"))
		// Skip this event - it's malformed
		return nil
	}

	_, err := uuid.Parse(campaignIDStr)
	if err != nil {
		p.logger.Error(ctx, "invalid campaign_id format", err)
		// Skip this event - it's malformed
		return nil
	}

	// Extract user_id from event data
	// The event.Data contains a "user" map with the user details
	userData, ok := event.Data["user"].(map[string]interface{})
	if !ok {
		p.logger.Error(ctx, "event missing user data", fmt.Errorf("invalid or missing user data"))
		// Skip this event - it's malformed
		return nil
	}

	userIDStr, ok := userData["id"].(string)
	if !ok || userIDStr == "" {
		p.logger.Error(ctx, "event missing user_id", fmt.Errorf("invalid or missing user_id"))
		// Skip this event - it's malformed
		return nil
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		p.logger.Error(ctx, "invalid user_id format", err)
		// Skip this event - it's malformed
		return nil
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_id", Value: campaignIDStr},
		observability.Field{Key: "user_id", Value: userID.String()},
	)

	p.logger.Info(ctx, fmt.Sprintf("processing %s event for position calculation", event.Type))

	// Calculate position for single user only (LOCK-FREE approach)
	// This updates only ONE row in the database, eliminating lock contention
	err = p.positionCalculator.CalculateUserPosition(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to calculate user position", err)
		return fmt.Errorf("failed to calculate position: %w", err)
	}

	p.logger.Info(ctx, "successfully processed event and calculated user position")
	return nil
}

// Name returns the processor name for logging
func (p *Processor) Name() string {
	return "position-calculator"
}
