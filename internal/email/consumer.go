package email

import (
	"base-server/internal/clients/kafka"
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// EventConsumer consumes events and sends emails
type EventConsumer struct {
	consumer     *kafka.Consumer
	emailService *EmailService
	store        store.Store
	logger       *observability.Logger
	workerCount  int
}

// NewEventConsumer creates a new email event consumer
func NewEventConsumer(consumer *kafka.Consumer, emailService *EmailService, store store.Store, logger *observability.Logger, workerCount int) *EventConsumer {
	if workerCount == 0 {
		workerCount = 5 // Default to 5 workers
	}

	return &EventConsumer{
		consumer:     consumer,
		emailService: emailService,
		store:        store,
		logger:       logger,
		workerCount:  workerCount,
	}
}

// Start begins consuming events and sending emails
func (c *EventConsumer) Start(ctx context.Context) error {
	c.logger.Info(ctx, fmt.Sprintf("Starting email event consumer with %d workers", c.workerCount))

	// Create a channel for events
	eventChan := make(chan kafka.EventMessage, 100)
	errorChan := make(chan error, 1)

	// Start consumer in a goroutine
	go func() {
		err := c.consumer.ConsumeEvents(ctx, func(msgCtx context.Context, event kafka.EventMessage) error {
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
		c.logger.Info(ctx, "Email consumer context cancelled")
		return ctx.Err()
	}

	return nil
}

// worker processes events from the event channel
func (c *EventConsumer) worker(ctx context.Context, workerID int, eventChan <-chan kafka.EventMessage) {
	workerCtx := observability.WithFields(ctx, observability.Field{Key: "worker_id", Value: workerID})
	c.logger.Info(workerCtx, fmt.Sprintf("Email worker %d started", workerID))

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				c.logger.Info(workerCtx, fmt.Sprintf("Email worker %d stopping - channel closed", workerID))
				return
			}

			eventCtx := observability.WithFields(workerCtx,
				observability.Field{Key: "event_type", Value: event.Type},
				observability.Field{Key: "event_id", Value: event.ID},
				observability.Field{Key: "account_id", Value: event.AccountID},
			)

			if err := c.handleEvent(eventCtx, event); err != nil {
				c.logger.Error(eventCtx, "failed to handle event", err)
			}

		case <-ctx.Done():
			c.logger.Info(workerCtx, fmt.Sprintf("Email worker %d stopping - context cancelled", workerID))
			return
		}
	}
}

// Stop gracefully stops the consumer
func (c *EventConsumer) Stop() {
	c.logger.Info(context.Background(), "Email consumer stopping...")
}

// handleEvent processes a single event
func (c *EventConsumer) handleEvent(ctx context.Context, event kafka.EventMessage) error {
	switch event.Type {
	case "user.created":
		return c.handleUserCreated(ctx, event)
	case "user.verified":
		return c.handleUserVerified(ctx, event)
	default:
		// Ignore events we don't care about
		return nil
	}
}

// handleUserCreated sends verification email for new waitlist signups
func (c *EventConsumer) handleUserCreated(ctx context.Context, event kafka.EventMessage) error {
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

	// Parse campaign ID
	campaignID, err := uuid.Parse(eventData.CampaignID)
	if err != nil {
		return fmt.Errorf("invalid campaign_id: %w", err)
	}

	// Get campaign to check email config
	campaign, err := c.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("failed to get campaign: %w", err)
	}

	// Check if email verification is enabled
	if campaign.EmailSettings == nil || !campaign.EmailSettings.VerificationRequired {
		// Email verification not enabled, skip
		c.logger.Info(ctx, "Email verification not enabled for campaign, skipping")
		return nil
	}

	// Extract user data
	email, ok := eventData.User["email"].(string)
	if !ok || email == "" {
		return fmt.Errorf("missing or invalid email in event data")
	}

	// Extract optional fields
	firstName := ""
	if fn, ok := eventData.User["first_name"]; ok && fn != nil {
		firstName = fn.(string)
	}

	position := 0
	if pos, ok := eventData.User["position"].(float64); ok {
		position = int(pos)
	}

	verificationToken := ""
	if token, ok := eventData.User["verification_token"]; ok && token != nil {
		if tokenPtr, ok := token.(*string); ok && tokenPtr != nil {
			verificationToken = *tokenPtr
		} else if tokenStr, ok := token.(string); ok {
			verificationToken = tokenStr
		}
	}

	referralLink := ""
	if rl, ok := eventData.User["referral_link"].(string); ok {
		referralLink = rl
	}

	campaignName := campaign.Name
	campaignSlug := campaign.Slug

	// Build verification link
	verificationLink := fmt.Sprintf("https://app.example.com/verify?token=%s&campaign=%s", verificationToken, campaignSlug)

	// Send verification email
	c.logger.Info(ctx, fmt.Sprintf("Sending verification email to %s for campaign %s", email, campaignName))

	err = c.emailService.SendWaitlistVerificationEmail(ctx, email, firstName, campaignName, verificationLink, referralLink, position)
	if err != nil {
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("Successfully sent verification email to %s", email))
	return nil
}

// handleUserVerified sends welcome email after email is verified
func (c *EventConsumer) handleUserVerified(ctx context.Context, event kafka.EventMessage) error {
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

	// Parse campaign ID
	campaignID, err := uuid.Parse(eventData.CampaignID)
	if err != nil {
		return fmt.Errorf("invalid campaign_id: %w", err)
	}

	// Get campaign
	campaign, err := c.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("failed to get campaign: %w", err)
	}

	// Extract user data
	email, ok := eventData.User["email"].(string)
	if !ok || email == "" {
		return fmt.Errorf("missing or invalid email in event data")
	}

	firstName := ""
	if fn, ok := eventData.User["first_name"]; ok && fn != nil {
		firstName = fn.(string)
	}

	position := 0
	if pos, ok := eventData.User["position"].(float64); ok {
		position = int(pos)
	}

	referralCount := 0
	if rc, ok := eventData.User["referral_count"].(float64); ok {
		referralCount = int(rc)
	}

	referralLink := ""
	if rl, ok := eventData.User["referral_link"].(string); ok {
		referralLink = rl
	}

	campaignName := campaign.Name

	// Send welcome email
	c.logger.Info(ctx, fmt.Sprintf("Sending welcome email to %s for campaign %s", email, campaignName))

	err = c.emailService.SendWaitlistWelcomeEmail(ctx, email, firstName, campaignName, referralLink, position, referralCount)
	if err != nil {
		return fmt.Errorf("failed to send welcome email: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("Successfully sent welcome email to %s", email))
	return nil
}
