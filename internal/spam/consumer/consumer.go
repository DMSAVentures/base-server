package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"base-server/internal/observability"
	spamProcessor "base-server/internal/spam/processor"
	"base-server/internal/store"
	"base-server/internal/workers"

	"github.com/google/uuid"
)

// SpamEventProcessor implements the EventProcessor interface for spam detection.
// It listens to user.created events and runs spam analysis asynchronously.
type SpamEventProcessor struct {
	processor *spamProcessor.Processor
	store     store.Store
	logger    *observability.Logger
}

// NewSpamEventProcessor creates a new spam event processor.
func NewSpamEventProcessor(
	processor *spamProcessor.Processor,
	store store.Store,
	logger *observability.Logger,
) workers.EventProcessor {
	return &SpamEventProcessor{
		processor: processor,
		store:     store,
		logger:    logger,
	}
}

// Name returns the processor name for logging and metrics.
func (p *SpamEventProcessor) Name() string {
	return "spam"
}

// Process handles a single event from Kafka.
// It routes the event to spam analysis if it's a user.created event.
func (p *SpamEventProcessor) Process(ctx context.Context, event workers.EventMessage) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "event_id", Value: event.ID},
		observability.Field{Key: "event_type", Value: event.Type},
		observability.Field{Key: "account_id", Value: event.AccountID},
	)

	// Only process user.created events
	if event.Type != "user.created" {
		return nil
	}

	p.logger.Info(ctx, "Processing spam check for user.created event")

	return p.handleUserCreated(ctx, event)
}

// handleUserCreated runs spam analysis for new waitlist signups.
func (p *SpamEventProcessor) handleUserCreated(ctx context.Context, event workers.EventMessage) error {
	// Parse event data
	var eventData struct {
		CampaignID string                 `json:"campaign_id"`
		User       map[string]interface{} `json:"user"`
	}

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &eventData); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	// Extract user ID from event data
	userIDStr, ok := eventData.User["id"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid user id in event data")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return fmt.Errorf("invalid user_id format: %w", err)
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "user_id", Value: userID},
		observability.Field{Key: "campaign_id", Value: eventData.CampaignID},
	)

	// Fetch the full user from the database
	user, err := p.store.GetWaitlistUserByID(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get waitlist user for spam analysis", err)
		return fmt.Errorf("failed to get waitlist user: %w", err)
	}

	// Skip if user is already blocked
	if user.Status == store.WaitlistUserStatusBlocked {
		p.logger.Info(ctx, "Skipping spam analysis - user already blocked")
		return nil
	}

	// Run spam analysis
	if err := p.processor.AnalyzeSignup(ctx, user); err != nil {
		p.logger.Error(ctx, "spam analysis failed", err)
		return fmt.Errorf("spam analysis failed: %w", err)
	}

	return nil
}
