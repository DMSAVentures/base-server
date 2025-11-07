package consumers

import (
	"base-server/internal/clients/kafka"
	"base-server/internal/jobs"
	"base-server/internal/jobs/workers"
	"base-server/internal/observability"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// EmailConsumer handles consuming domain events and sending emails
type EmailConsumer struct {
	kafkaConsumer *kafka.Consumer
	emailWorker   *workers.EmailWorker
	logger        *observability.Logger
	workerCount   int
}

// NewEmailConsumer creates a new EmailConsumer
func NewEmailConsumer(
	kafkaConsumer *kafka.Consumer,
	emailWorker *workers.EmailWorker,
	logger *observability.Logger,
	workerCount int,
) *EmailConsumer {
	if workerCount == 0 {
		workerCount = 10 // Default to 10 workers
	}

	return &EmailConsumer{
		kafkaConsumer: kafkaConsumer,
		emailWorker:   emailWorker,
		logger:        logger,
		workerCount:   workerCount,
	}
}

// Start starts consuming events from Kafka with multiple workers
func (c *EmailConsumer) Start(ctx context.Context) error {
	c.logger.Info(ctx, fmt.Sprintf("Starting email consumer with %d workers", c.workerCount))

	// Create a channel for events
	eventChan := make(chan kafka.EventMessage, 100)
	errorChan := make(chan error, 1)

	// Start consumer in a goroutine
	go func() {
		err := c.kafkaConsumer.ConsumeEvents(ctx, func(msgCtx context.Context, event kafka.EventMessage) error {
			// Send event to worker pool
			select {
			case eventChan <- event:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if err != nil {
			errorChan <- err
		}
		close(eventChan)
	}()

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < c.workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			c.worker(ctx, workerID, eventChan)
		}(i)
	}

	// Wait for completion or error
	go func() {
		wg.Wait()
		close(errorChan)
	}()

	select {
	case err := <-errorChan:
		if err != nil {
			c.logger.Error(ctx, "consumer error", err)
			return err
		}
	case <-ctx.Done():
		c.logger.Info(ctx, "Consumer context cancelled")
		return ctx.Err()
	}

	return nil
}

// worker processes events from the event channel
func (c *EmailConsumer) worker(ctx context.Context, workerID int, eventChan <-chan kafka.EventMessage) {
	workerCtx := observability.WithFields(ctx, observability.Field{Key: "worker_id", Value: workerID})
	c.logger.Info(workerCtx, fmt.Sprintf("Email worker %d started", workerID))

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				c.logger.Info(workerCtx, fmt.Sprintf("Email worker %d stopping - channel closed", workerID))
				return
			}

			// Process event
			err := c.processEvent(workerCtx, event)
			if err != nil {
				c.logger.Error(workerCtx, fmt.Sprintf("Email worker %d failed to process event", workerID), err)
			}

		case <-ctx.Done():
			c.logger.Info(workerCtx, fmt.Sprintf("Email worker %d stopping - context cancelled", workerID))
			return
		}
	}
}

// processEvent processes a single event by routing to the appropriate email type
func (c *EmailConsumer) processEvent(ctx context.Context, event kafka.EventMessage) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "event_id", Value: event.ID},
		observability.Field{Key: "event_type", Value: event.Type},
		observability.Field{Key: "account_id", Value: event.AccountID},
	)

	c.logger.Info(ctx, fmt.Sprintf("Processing event %s for email", event.Type))

	// Route based on event type
	var err error
	switch event.Type {
	case "user.signed_up":
		err = c.sendWelcomeEmail(ctx, event)
	case "user.verified":
		err = c.sendVerificationConfirmationEmail(ctx, event)
	case "referral.verified":
		err = c.sendPositionUpdateEmail(ctx, event)
	case "reward.earned":
		err = c.sendRewardEarnedEmail(ctx, event)
	case "campaign.milestone_reached":
		err = c.sendMilestoneEmail(ctx, event)
	default:
		c.logger.Info(ctx, fmt.Sprintf("Email consumer ignoring event type: %s", event.Type))
		return nil
	}

	if err != nil {
		c.logger.Error(ctx, fmt.Sprintf("Failed to send email for %s", event.Type), err)
		return err
	}

	c.logger.Info(ctx, fmt.Sprintf("Successfully sent email for event %s", event.Type))
	return nil
}

// sendWelcomeEmail sends a welcome email when user signs up
func (c *EmailConsumer) sendWelcomeEmail(ctx context.Context, event kafka.EventMessage) error {
	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	userID, _ := uuid.Parse(data["user_id"].(string))
	campaignID, _ := uuid.Parse(data["campaign_id"].(string))

	payload := jobs.EmailJobPayload{
		Type:       "welcome",
		CampaignID: campaignID,
		UserID:     userID,
		Priority:   1,
	}

	return c.emailWorker.ProcessEmailJob(ctx, payload)
}

// sendVerificationConfirmationEmail sends confirmation email when user is verified
func (c *EmailConsumer) sendVerificationConfirmationEmail(ctx context.Context, event kafka.EventMessage) error {
	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	userID, _ := uuid.Parse(data["user_id"].(string))
	campaignID, _ := uuid.Parse(data["campaign_id"].(string))

	payload := jobs.EmailJobPayload{
		Type:       "verification",
		CampaignID: campaignID,
		UserID:     userID,
		Priority:   1,
	}

	return c.emailWorker.ProcessEmailJob(ctx, payload)
}

// sendPositionUpdateEmail sends position update email when referral is verified
func (c *EmailConsumer) sendPositionUpdateEmail(ctx context.Context, event kafka.EventMessage) error {
	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	referrerID, _ := uuid.Parse(data["referrer_id"].(string))
	campaignID, _ := uuid.Parse(data["campaign_id"].(string))

	payload := jobs.EmailJobPayload{
		Type:       "position_update",
		CampaignID: campaignID,
		UserID:     referrerID, // Send to the referrer
		Priority:   2,
	}

	return c.emailWorker.ProcessEmailJob(ctx, payload)
}

// sendRewardEarnedEmail sends reward notification email
func (c *EmailConsumer) sendRewardEarnedEmail(ctx context.Context, event kafka.EventMessage) error {
	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	userID, _ := uuid.Parse(data["user_id"].(string))
	campaignID, _ := uuid.Parse(data["campaign_id"].(string))

	payload := jobs.EmailJobPayload{
		Type:       "reward_earned",
		CampaignID: campaignID,
		UserID:     userID,
		Priority:   1,
	}

	return c.emailWorker.ProcessEmailJob(ctx, payload)
}

// sendMilestoneEmail sends milestone notification email
func (c *EmailConsumer) sendMilestoneEmail(ctx context.Context, event kafka.EventMessage) error {
	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	campaignID, _ := uuid.Parse(data["campaign_id"].(string))

	payload := jobs.EmailJobPayload{
		Type:       "milestone",
		CampaignID: campaignID,
		UserID:     uuid.Nil, // Will need to send to all users in campaign
		Priority:   3,
	}

	return c.emailWorker.ProcessEmailJob(ctx, payload)
}

// Stop stops the consumer
func (c *EmailConsumer) Stop() error {
	c.logger.Info(context.Background(), "Stopping email consumer")
	return c.kafkaConsumer.Close()
}
