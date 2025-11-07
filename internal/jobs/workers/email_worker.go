package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"base-server/internal/email"
	"base-server/internal/jobs"
	"base-server/internal/observability"
	"base-server/internal/store"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// EmailWorker handles email sending jobs
type EmailWorker struct {
	store         *store.Store
	emailService  email.Service
	logger        *observability.Logger
}

// NewEmailWorker creates a new email worker
func NewEmailWorker(store *store.Store, emailService email.Service, logger *observability.Logger) *EmailWorker {
	return &EmailWorker{
		store:        store,
		emailService: emailService,
		logger:       logger,
	}
}

// ProcessEmailJob processes an email job (for Kafka)
func (w *EmailWorker) ProcessEmailJob(ctx context.Context, payload jobs.EmailJobPayload) error {
	return w.processEmail(ctx, payload)
}

// ProcessEmailTask processes an email task (for Asynq)
func (w *EmailWorker) ProcessEmailTask(ctx context.Context, task *asynq.Task) error {
	var payload jobs.EmailJobPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		w.logger.Error(ctx, "failed to unmarshal email job payload", err)
		return fmt.Errorf("failed to unmarshal email job payload: %w", err)
	}
	return w.processEmail(ctx, payload)
}

// processEmail contains the core email sending logic
func (w *EmailWorker) processEmail(ctx context.Context, payload jobs.EmailJobPayload) error {
	// Get user
	user, err := w.store.GetWaitlistUserByID(ctx, payload.UserID)
	if err != nil {
		w.logger.Error(ctx, "failed to get user", err)
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Get campaign
	campaign, err := w.store.GetCampaignByID(ctx, payload.CampaignID)
	if err != nil {
		w.logger.Error(ctx, "failed to get campaign", err)
		return fmt.Errorf("failed to get campaign: %w", err)
	}

	// Get template if specified, otherwise use default
	var template *store.EmailTemplate
	if payload.TemplateID != nil {
		tmpl, err := w.store.GetEmailTemplateByID(ctx, *payload.TemplateID)
		if err != nil {
			w.logger.Error(ctx, "failed to get email template", err)
			return fmt.Errorf("failed to get email template: %w", err)
		}
		template = &tmpl
	} else {
		// Get default template for this type
		tmpl, err := w.store.GetEmailTemplateByType(ctx, payload.CampaignID, payload.Type)
		if err != nil {
			w.logger.Error(ctx, "failed to get default email template", err)
			return fmt.Errorf("failed to get default email template: %w", err)
		}
		template = &tmpl
	}

	// Create email log
	emailLog, err := w.store.CreateEmailLog(ctx, store.CreateEmailLogParams{
		CampaignID:     payload.CampaignID,
		UserID:         &payload.UserID,
		TemplateID:     &template.ID,
		RecipientEmail: user.Email,
		Subject:        template.Subject,
		Type:           payload.Type,
	})
	if err != nil {
		w.logger.Error(ctx, "failed to create email log", err)
		return fmt.Errorf("failed to create email log: %w", err)
	}

	// Prepare template data
	templateData := w.prepareTemplateData(user, campaign, payload.TemplateData)

	// Render email content
	htmlBody := w.renderTemplate(template.HTMLBody, templateData)
	textBody := w.renderTemplate(template.TextBody, templateData)
	subject := w.renderTemplate(template.Subject, templateData)

	// Get email config from campaign
	emailConfig, ok := campaign.EmailConfig.(map[string]interface{})
	if !ok {
		emailConfig = map[string]interface{}{
			"from_name":  "Waitlist",
			"from_email": "noreply@example.com",
		}
	}

	fromName := w.getString(emailConfig, "from_name", "Waitlist")
	fromEmail := w.getString(emailConfig, "from_email", "noreply@example.com")

	// Send email via email service
	err = w.emailService.Send(ctx, email.SendParams{
		From:     fmt.Sprintf("%s <%s>", fromName, fromEmail),
		To:       user.Email,
		Subject:  subject,
		HTMLBody: htmlBody,
		TextBody: textBody,
	})

	if err != nil {
		// Update email log as failed
		if updateErr := w.store.UpdateEmailLogFailed(ctx, emailLog.ID, err.Error()); updateErr != nil {
			w.logger.Error(ctx, "failed to update email log as failed", updateErr)
		}
		w.logger.Error(ctx, "failed to send email", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	// Update email log as sent
	// Note: In production, you'd get the provider message ID from the email service
	providerMessageID := uuid.New().String()
	if err := w.store.UpdateEmailLogSent(ctx, emailLog.ID, providerMessageID); err != nil {
		w.logger.Error(ctx, "failed to update email log as sent", err)
	}

	w.logger.Info(ctx, fmt.Sprintf("successfully sent %s email to %s", payload.Type, user.Email))
	return nil
}

// prepareTemplateData prepares template data with user and campaign information
func (w *EmailWorker) prepareTemplateData(user store.WaitlistUser, campaign store.Campaign, customData map[string]interface{}) map[string]interface{} {
	data := map[string]interface{}{
		"first_name":     w.getStringPtr(user.FirstName),
		"last_name":      w.getStringPtr(user.LastName),
		"email":          user.Email,
		"position":       user.Position,
		"referral_code":  user.ReferralCode,
		"referral_link":  fmt.Sprintf("https://example.com/join/%s", user.ReferralCode),
		"referral_count": user.ReferralCount,
		"campaign_name":  campaign.Name,
		"campaign_slug":  campaign.Slug,
	}

	// Merge custom data
	for k, v := range customData {
		data[k] = v
	}

	return data
}

// renderTemplate renders a template string with data
func (w *EmailWorker) renderTemplate(template string, data map[string]interface{}) string {
	result := template
	for key, value := range data {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}
	return result
}

// Helper functions
func (w *EmailWorker) getString(m map[string]interface{}, key, defaultValue string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func (w *EmailWorker) getStringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
