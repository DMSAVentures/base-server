package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// CreateEmailTemplateParams represents parameters for creating an email template
type CreateEmailTemplateParams struct {
	CampaignID        uuid.UUID
	Name              string
	Type              string
	Subject           string
	HTMLBody          string
	TextBody          string
	Enabled           bool
	SendAutomatically bool
	VariantName       *string
	VariantWeight     *int
}

const sqlCreateEmailTemplate = `
INSERT INTO email_templates (campaign_id, name, type, subject, html_body, text_body, enabled, send_automatically, variant_name, variant_weight)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, campaign_id, name, type, subject, html_body, text_body, enabled, send_automatically, variant_name, variant_weight, created_at, updated_at, deleted_at
`

// CreateEmailTemplate creates a new email template
func (s *Store) CreateEmailTemplate(ctx context.Context, params CreateEmailTemplateParams) (EmailTemplate, error) {
	var template EmailTemplate
	err := s.db.GetContext(ctx, &template, sqlCreateEmailTemplate,
		params.CampaignID,
		params.Name,
		params.Type,
		params.Subject,
		params.HTMLBody,
		params.TextBody,
		params.Enabled,
		params.SendAutomatically,
		params.VariantName,
		params.VariantWeight)
	if err != nil {
		return EmailTemplate{}, fmt.Errorf("failed to create email template: %w", err)
	}
	return template, nil
}

const sqlGetEmailTemplateByID = `
SELECT id, campaign_id, name, type, subject, html_body, text_body, enabled, send_automatically, variant_name, variant_weight, created_at, updated_at, deleted_at
FROM email_templates
WHERE id = $1 AND deleted_at IS NULL
`

// GetEmailTemplateByID retrieves an email template by ID
func (s *Store) GetEmailTemplateByID(ctx context.Context, templateID uuid.UUID) (EmailTemplate, error) {
	var template EmailTemplate
	err := s.db.GetContext(ctx, &template, sqlGetEmailTemplateByID, templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EmailTemplate{}, ErrNotFound
		}
		return EmailTemplate{}, fmt.Errorf("failed to get email template: %w", err)
	}
	return template, nil
}

const sqlGetEmailTemplatesByCampaign = `
SELECT id, campaign_id, name, type, subject, html_body, text_body, enabled, send_automatically, variant_name, variant_weight, created_at, updated_at, deleted_at
FROM email_templates
WHERE campaign_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
`

// GetEmailTemplatesByCampaign retrieves all email templates for a campaign
func (s *Store) GetEmailTemplatesByCampaign(ctx context.Context, campaignID uuid.UUID) ([]EmailTemplate, error) {
	var templates []EmailTemplate
	err := s.db.SelectContext(ctx, &templates, sqlGetEmailTemplatesByCampaign, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to get email templates: %w", err)
	}
	return templates, nil
}

const sqlGetEmailTemplateByType = `
SELECT id, campaign_id, name, type, subject, html_body, text_body, enabled, send_automatically, variant_name, variant_weight, created_at, updated_at, deleted_at
FROM email_templates
WHERE campaign_id = $1 AND type = $2 AND enabled = TRUE AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 1
`

// GetEmailTemplateByType retrieves an email template by campaign and type
func (s *Store) GetEmailTemplateByType(ctx context.Context, campaignID uuid.UUID, templateType string) (EmailTemplate, error) {
	var template EmailTemplate
	err := s.db.GetContext(ctx, &template, sqlGetEmailTemplateByType, campaignID, templateType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EmailTemplate{}, ErrNotFound
		}
		return EmailTemplate{}, fmt.Errorf("failed to get email template by type: %w", err)
	}
	return template, nil
}

const sqlUpdateEmailTemplate = `
UPDATE email_templates
SET name = COALESCE($2, name),
    subject = COALESCE($3, subject),
    html_body = COALESCE($4, html_body),
    text_body = COALESCE($5, text_body),
    enabled = COALESCE($6, enabled),
    send_automatically = COALESCE($7, send_automatically),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, campaign_id, name, type, subject, html_body, text_body, enabled, send_automatically, variant_name, variant_weight, created_at, updated_at, deleted_at
`

// UpdateEmailTemplateParams represents parameters for updating an email template
type UpdateEmailTemplateParams struct {
	Name              *string
	Subject           *string
	HTMLBody          *string
	TextBody          *string
	Enabled           *bool
	SendAutomatically *bool
}

// UpdateEmailTemplate updates an email template
func (s *Store) UpdateEmailTemplate(ctx context.Context, templateID uuid.UUID, params UpdateEmailTemplateParams) (EmailTemplate, error) {
	var template EmailTemplate
	err := s.db.GetContext(ctx, &template, sqlUpdateEmailTemplate,
		templateID,
		params.Name,
		params.Subject,
		params.HTMLBody,
		params.TextBody,
		params.Enabled,
		params.SendAutomatically)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EmailTemplate{}, ErrNotFound
		}
		return EmailTemplate{}, fmt.Errorf("failed to update email template: %w", err)
	}
	return template, nil
}

const sqlDeleteEmailTemplate = `
UPDATE email_templates
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// DeleteEmailTemplate soft deletes an email template
func (s *Store) DeleteEmailTemplate(ctx context.Context, templateID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, sqlDeleteEmailTemplate, templateID)
	if err != nil {
		return fmt.Errorf("failed to delete email template: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// Email Log operations

// CreateEmailLogParams represents parameters for creating an email log
type CreateEmailLogParams struct {
	CampaignID        uuid.UUID
	UserID            *uuid.UUID
	TemplateID        *uuid.UUID
	RecipientEmail    string
	Subject           string
	Type              string
	ProviderMessageID *string
}

const sqlCreateEmailLog = `
INSERT INTO email_logs (campaign_id, user_id, template_id, recipient_email, subject, type, provider_message_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, campaign_id, user_id, template_id, recipient_email, subject, type, status, provider_message_id, sent_at, delivered_at, opened_at, clicked_at, bounced_at, failed_at, error_message, bounce_reason, open_count, click_count, created_at, updated_at
`

// CreateEmailLog creates a new email log entry
func (s *Store) CreateEmailLog(ctx context.Context, params CreateEmailLogParams) (EmailLog, error) {
	var log EmailLog
	err := s.db.GetContext(ctx, &log, sqlCreateEmailLog,
		params.CampaignID,
		params.UserID,
		params.TemplateID,
		params.RecipientEmail,
		params.Subject,
		params.Type,
		params.ProviderMessageID)
	if err != nil {
		return EmailLog{}, fmt.Errorf("failed to create email log: %w", err)
	}
	return log, nil
}

const sqlGetEmailLogByID = `
SELECT id, campaign_id, user_id, template_id, recipient_email, subject, type, status, provider_message_id, sent_at, delivered_at, opened_at, clicked_at, bounced_at, failed_at, error_message, bounce_reason, open_count, click_count, created_at, updated_at
FROM email_logs
WHERE id = $1
`

// GetEmailLogByID retrieves an email log by ID
func (s *Store) GetEmailLogByID(ctx context.Context, logID uuid.UUID) (EmailLog, error) {
	var log EmailLog
	err := s.db.GetContext(ctx, &log, sqlGetEmailLogByID, logID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EmailLog{}, ErrNotFound
		}
		return EmailLog{}, fmt.Errorf("failed to get email log: %w", err)
	}
	return log, nil
}

const sqlGetEmailLogsByUser = `
SELECT id, campaign_id, user_id, template_id, recipient_email, subject, type, status, provider_message_id, sent_at, delivered_at, opened_at, clicked_at, bounced_at, failed_at, error_message, bounce_reason, open_count, click_count, created_at, updated_at
FROM email_logs
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
`

// GetEmailLogsByUser retrieves email logs for a user with pagination
func (s *Store) GetEmailLogsByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]EmailLog, error) {
	var logs []EmailLog
	err := s.db.SelectContext(ctx, &logs, sqlGetEmailLogsByUser, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get email logs by user: %w", err)
	}
	return logs, nil
}

const sqlGetEmailLogsByCampaign = `
SELECT id, campaign_id, user_id, template_id, recipient_email, subject, type, status, provider_message_id, sent_at, delivered_at, opened_at, clicked_at, bounced_at, failed_at, error_message, bounce_reason, open_count, click_count, created_at, updated_at
FROM email_logs
WHERE campaign_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
`

// GetEmailLogsByCampaign retrieves email logs for a campaign with pagination
func (s *Store) GetEmailLogsByCampaign(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]EmailLog, error) {
	var logs []EmailLog
	err := s.db.SelectContext(ctx, &logs, sqlGetEmailLogsByCampaign, campaignID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get email logs by campaign: %w", err)
	}
	return logs, nil
}

const sqlUpdateEmailLogStatus = `
UPDATE email_logs
SET status = $2,
    sent_at = CASE WHEN $2 = 'sent' THEN COALESCE(sent_at, CURRENT_TIMESTAMP) ELSE sent_at END,
    delivered_at = CASE WHEN $2 = 'delivered' THEN CURRENT_TIMESTAMP ELSE delivered_at END,
    opened_at = CASE WHEN $2 = 'opened' THEN COALESCE(opened_at, CURRENT_TIMESTAMP) ELSE opened_at END,
    clicked_at = CASE WHEN $2 = 'clicked' THEN COALESCE(clicked_at, CURRENT_TIMESTAMP) ELSE clicked_at END,
    bounced_at = CASE WHEN $2 = 'bounced' THEN CURRENT_TIMESTAMP ELSE bounced_at END,
    failed_at = CASE WHEN $2 = 'failed' THEN CURRENT_TIMESTAMP ELSE failed_at END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// UpdateEmailLogStatus updates the status of an email log
func (s *Store) UpdateEmailLogStatus(ctx context.Context, logID uuid.UUID, status string) error {
	res, err := s.db.ExecContext(ctx, sqlUpdateEmailLogStatus, logID, status)
	if err != nil {
		return fmt.Errorf("failed to update email log status: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

const sqlIncrementEmailOpenCount = `
UPDATE email_logs
SET open_count = open_count + 1,
    opened_at = COALESCE(opened_at, CURRENT_TIMESTAMP),
    status = CASE WHEN status = 'sent' OR status = 'delivered' THEN 'opened' ELSE status END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// IncrementEmailOpenCount increments the open count for an email
func (s *Store) IncrementEmailOpenCount(ctx context.Context, logID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlIncrementEmailOpenCount, logID)
	if err != nil {
		return fmt.Errorf("failed to increment email open count: %w", err)
	}
	return nil
}

const sqlIncrementEmailClickCount = `
UPDATE email_logs
SET click_count = click_count + 1,
    clicked_at = COALESCE(clicked_at, CURRENT_TIMESTAMP),
    status = CASE WHEN status IN ('sent', 'delivered', 'opened') THEN 'clicked' ELSE status END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// IncrementEmailClickCount increments the click count for an email
func (s *Store) IncrementEmailClickCount(ctx context.Context, logID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlIncrementEmailClickCount, logID)
	if err != nil {
		return fmt.Errorf("failed to increment email click count: %w", err)
	}
	return nil
}

const sqlGetEmailLogByProviderMessageID = `
SELECT id, campaign_id, user_id, template_id, recipient_email, subject, type, status, provider_message_id, sent_at, delivered_at, opened_at, clicked_at, bounced_at, failed_at, error_message, bounce_reason, open_count, click_count, created_at, updated_at
FROM email_logs
WHERE provider_message_id = $1
`

// GetEmailLogByProviderMessageID retrieves an email log by provider message ID
func (s *Store) GetEmailLogByProviderMessageID(ctx context.Context, providerMessageID string) (EmailLog, error) {
	var log EmailLog
	err := s.db.GetContext(ctx, &log, sqlGetEmailLogByProviderMessageID, providerMessageID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EmailLog{}, ErrNotFound
		}
		return EmailLog{}, fmt.Errorf("failed to get email log by provider message id: %w", err)
	}
	return log, nil
}
