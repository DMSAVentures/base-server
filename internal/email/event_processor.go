package email

import (
	"context"
	"encoding/json"
	"fmt"

	"base-server/internal/observability"
	"base-server/internal/store"
	"base-server/internal/workers"

	"github.com/google/uuid"
)

// EmailEventProcessor implements the EventProcessor interface for email events.
// It listens to user events and sends appropriate emails (verification, welcome, etc.).
type EmailEventProcessor struct {
	emailService *EmailService
	store        store.Store
	logger       *observability.Logger
}

// NewEmailEventProcessor creates a new email event processor.
func NewEmailEventProcessor(
	emailService *EmailService,
	store store.Store,
	logger *observability.Logger,
) workers.EventProcessor {
	return &EmailEventProcessor{
		emailService: emailService,
		store:        store,
		logger:       logger,
	}
}

// Name returns the processor name for logging and metrics.
func (p *EmailEventProcessor) Name() string {
	return "email"
}

// Process handles a single email event from Kafka.
// It routes the event to the appropriate email handler based on event type.
// Returns an error if processing fails, which prevents offset commit and enables replay.
func (p *EmailEventProcessor) Process(ctx context.Context, event workers.EventMessage) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "event_id", Value: event.ID},
		observability.Field{Key: "event_type", Value: event.Type},
		observability.Field{Key: "account_id", Value: event.AccountID},
	)

	p.logger.Info(ctx, fmt.Sprintf("Processing email event %s", event.Type))

	// Route to appropriate handler based on event type
	switch event.Type {
	case "user.created":
		return p.handleUserCreated(ctx, event)
	case "user.verified":
		return p.handleUserVerified(ctx, event)
	default:
		// Ignore events we don't handle - not an error, commit offset
		p.logger.Info(ctx, fmt.Sprintf("Ignoring unhandled event type %s", event.Type))
		return nil
	}
}

// handleUserCreated sends verification or welcome email for new waitlist signups.
// If verification is enabled: sends verification email
// If verification is disabled but send_welcome_email is enabled: sends welcome email immediately
func (p *EmailEventProcessor) handleUserCreated(ctx context.Context, event workers.EventMessage) error {
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
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("failed to get campaign: %w", err)
	}

	// Check email settings
	verificationEnabled := false
	sendWelcomeEmail := false
	if campaign.EmailSettings != nil {
		verificationEnabled = campaign.EmailSettings.VerificationRequired
		sendWelcomeEmail = campaign.EmailSettings.SendWelcomeEmail
	}

	// If neither setting is enabled, skip
	if !verificationEnabled && !sendWelcomeEmail {
		p.logger.Info(ctx, "No email settings enabled for campaign, skipping")
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

	referralLink := ""
	if rl, ok := eventData.User["referral_link"].(string); ok {
		referralLink = rl
	}

	campaignName := campaign.Name

	// Decide which email to send based on settings
	if verificationEnabled {
		// Send verification email
		verificationToken := ""
		if token, ok := eventData.User["verification_token"]; ok && token != nil {
			if tokenPtr, ok := token.(*string); ok && tokenPtr != nil {
				verificationToken = *tokenPtr
			} else if tokenStr, ok := token.(string); ok {
				verificationToken = tokenStr
			}
		}

		campaignSlug := campaign.Slug
		verificationLink := fmt.Sprintf("https://app.example.com/verify?token=%s&campaign=%s",
			verificationToken, campaignSlug)

		p.logger.Info(ctx, fmt.Sprintf("Sending verification email to %s for campaign %s",
			email, campaignName))

		err = p.emailService.SendWaitlistVerificationEmail(
			ctx, email, firstName, campaignName, verificationLink, referralLink, position)
		if err != nil {
			return fmt.Errorf("failed to send verification email: %w", err)
		}

		p.logger.Info(ctx, fmt.Sprintf("Successfully sent verification email to %s", email))
	} else if sendWelcomeEmail {
		// Send welcome email immediately (no verification required)
		p.logger.Info(ctx, fmt.Sprintf("Sending welcome email to %s for campaign %s (no verification required)",
			email, campaignName))

		referralCount := 0
		if rc, ok := eventData.User["referral_count"].(float64); ok {
			referralCount = int(rc)
		}

		err = p.emailService.SendWaitlistWelcomeEmail(
			ctx, email, firstName, campaignName, referralLink, position, referralCount)
		if err != nil {
			return fmt.Errorf("failed to send welcome email: %w", err)
		}

		p.logger.Info(ctx, fmt.Sprintf("Successfully sent welcome email to %s", email))
	}

	return nil
}

// handleUserVerified sends welcome email after email is verified.
func (p *EmailEventProcessor) handleUserVerified(ctx context.Context, event workers.EventMessage) error {
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
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
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
	p.logger.Info(ctx, fmt.Sprintf("Sending welcome email to %s for campaign %s",
		email, campaignName))

	err = p.emailService.SendWaitlistWelcomeEmail(
		ctx, email, firstName, campaignName, referralLink, position, referralCount)
	if err != nil {
		return fmt.Errorf("failed to send welcome email: %w", err)
	}

	p.logger.Info(ctx, fmt.Sprintf("Successfully sent welcome email to %s", email))
	return nil
}
