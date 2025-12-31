package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// CreateCampaignEmailTemplateParams represents parameters for creating a campaign email template
type CreateCampaignEmailTemplateParams struct {
	CampaignID        uuid.UUID
	Name              string
	Type              string
	Subject           string
	HTMLBody          string
	BlocksJSON        *JSONB
	Enabled           bool
	SendAutomatically bool
	VariantName       *string
	VariantWeight     *int
}

const sqlCreateCampaignEmailTemplate = `
INSERT INTO campaign_email_templates (campaign_id, name, type, subject, html_body, blocks_json, enabled, send_automatically, variant_name, variant_weight)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, campaign_id, name, type, subject, html_body, blocks_json, enabled, send_automatically, variant_name, variant_weight, created_at, updated_at, deleted_at
`

// CreateCampaignEmailTemplate creates a new campaign email template
func (s *Store) CreateCampaignEmailTemplate(ctx context.Context, params CreateCampaignEmailTemplateParams) (CampaignEmailTemplate, error) {
	var template CampaignEmailTemplate
	err := s.db.GetContext(ctx, &template, sqlCreateCampaignEmailTemplate,
		params.CampaignID,
		params.Name,
		params.Type,
		params.Subject,
		params.HTMLBody,
		params.BlocksJSON,
		params.Enabled,
		params.SendAutomatically,
		params.VariantName,
		params.VariantWeight)
	if err != nil {
		return CampaignEmailTemplate{}, fmt.Errorf("failed to create campaign email template: %w", err)
	}
	return template, nil
}

const sqlGetCampaignEmailTemplateByID = `
SELECT id, campaign_id, name, type, subject, html_body, blocks_json, enabled, send_automatically, variant_name, variant_weight, created_at, updated_at, deleted_at
FROM campaign_email_templates
WHERE id = $1 AND deleted_at IS NULL
`

// GetCampaignEmailTemplateByID retrieves a campaign email template by ID
func (s *Store) GetCampaignEmailTemplateByID(ctx context.Context, templateID uuid.UUID) (CampaignEmailTemplate, error) {
	var template CampaignEmailTemplate
	err := s.db.GetContext(ctx, &template, sqlGetCampaignEmailTemplateByID, templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignEmailTemplate{}, ErrNotFound
		}
		return CampaignEmailTemplate{}, fmt.Errorf("failed to get campaign email template: %w", err)
	}
	return template, nil
}

const sqlGetCampaignEmailTemplatesByCampaign = `
SELECT id, campaign_id, name, type, subject, html_body, blocks_json, enabled, send_automatically, variant_name, variant_weight, created_at, updated_at, deleted_at
FROM campaign_email_templates
WHERE campaign_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
`

// GetCampaignEmailTemplatesByCampaign retrieves all campaign email templates for a campaign
func (s *Store) GetCampaignEmailTemplatesByCampaign(ctx context.Context, campaignID uuid.UUID) ([]CampaignEmailTemplate, error) {
	var templates []CampaignEmailTemplate
	err := s.db.SelectContext(ctx, &templates, sqlGetCampaignEmailTemplatesByCampaign, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign email templates: %w", err)
	}
	return templates, nil
}

const sqlGetCampaignEmailTemplatesByAccount = `
SELECT et.id, et.campaign_id, et.name, et.type, et.subject, et.html_body, et.blocks_json, et.enabled, et.send_automatically, et.variant_name, et.variant_weight, et.created_at, et.updated_at, et.deleted_at
FROM campaign_email_templates et
JOIN campaigns c ON et.campaign_id = c.id
WHERE c.account_id = $1 AND et.deleted_at IS NULL AND c.deleted_at IS NULL
ORDER BY et.created_at DESC
`

// GetCampaignEmailTemplatesByAccount retrieves all campaign email templates for an account across all campaigns
func (s *Store) GetCampaignEmailTemplatesByAccount(ctx context.Context, accountID uuid.UUID) ([]CampaignEmailTemplate, error) {
	var templates []CampaignEmailTemplate
	err := s.db.SelectContext(ctx, &templates, sqlGetCampaignEmailTemplatesByAccount, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign email templates by account: %w", err)
	}
	return templates, nil
}

const sqlGetCampaignEmailTemplateByType = `
SELECT id, campaign_id, name, type, subject, html_body, blocks_json, enabled, send_automatically, variant_name, variant_weight, created_at, updated_at, deleted_at
FROM campaign_email_templates
WHERE campaign_id = $1 AND type = $2 AND enabled = TRUE AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 1
`

// GetCampaignEmailTemplateByType retrieves a campaign email template by campaign and type
func (s *Store) GetCampaignEmailTemplateByType(ctx context.Context, campaignID uuid.UUID, templateType string) (CampaignEmailTemplate, error) {
	var template CampaignEmailTemplate
	err := s.db.GetContext(ctx, &template, sqlGetCampaignEmailTemplateByType, campaignID, templateType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignEmailTemplate{}, ErrNotFound
		}
		return CampaignEmailTemplate{}, fmt.Errorf("failed to get campaign email template by type: %w", err)
	}
	return template, nil
}

const sqlUpdateCampaignEmailTemplate = `
UPDATE campaign_email_templates
SET name = COALESCE($2, name),
    subject = COALESCE($3, subject),
    html_body = COALESCE($4, html_body),
    blocks_json = COALESCE($5, blocks_json),
    enabled = COALESCE($6, enabled),
    send_automatically = COALESCE($7, send_automatically),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, campaign_id, name, type, subject, html_body, blocks_json, enabled, send_automatically, variant_name, variant_weight, created_at, updated_at, deleted_at
`

// UpdateCampaignEmailTemplateParams represents parameters for updating a campaign email template
type UpdateCampaignEmailTemplateParams struct {
	Name              *string
	Subject           *string
	HTMLBody          *string
	BlocksJSON        *JSONB
	Enabled           *bool
	SendAutomatically *bool
}

// UpdateCampaignEmailTemplate updates a campaign email template
func (s *Store) UpdateCampaignEmailTemplate(ctx context.Context, templateID uuid.UUID, params UpdateCampaignEmailTemplateParams) (CampaignEmailTemplate, error) {
	var template CampaignEmailTemplate
	err := s.db.GetContext(ctx, &template, sqlUpdateCampaignEmailTemplate,
		templateID,
		params.Name,
		params.Subject,
		params.HTMLBody,
		params.BlocksJSON,
		params.Enabled,
		params.SendAutomatically)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignEmailTemplate{}, ErrNotFound
		}
		return CampaignEmailTemplate{}, fmt.Errorf("failed to update campaign email template: %w", err)
	}
	return template, nil
}

const sqlDeleteCampaignEmailTemplate = `
UPDATE campaign_email_templates
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// DeleteCampaignEmailTemplate soft deletes a campaign email template
func (s *Store) DeleteCampaignEmailTemplate(ctx context.Context, templateID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, sqlDeleteCampaignEmailTemplate, templateID)
	if err != nil {
		return fmt.Errorf("failed to delete campaign email template: %w", err)
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
	CampaignID         uuid.UUID
	UserID             *uuid.UUID
	CampaignTemplateID *uuid.UUID
	BlastTemplateID    *uuid.UUID
	BlastID            *uuid.UUID
	RecipientEmail     string
	Subject            string
	Type               string
	ProviderMessageID  *string
}

const sqlCreateEmailLog = `
INSERT INTO email_logs (campaign_id, user_id, campaign_template_id, blast_template_id, blast_id, recipient_email, subject, type, provider_message_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, campaign_id, user_id, campaign_template_id, blast_template_id, blast_id, recipient_email, subject, type, status, provider_message_id, sent_at, delivered_at, opened_at, clicked_at, bounced_at, failed_at, error_message, bounce_reason, open_count, click_count, created_at, updated_at
`

// CreateEmailLog creates a new email log entry
func (s *Store) CreateEmailLog(ctx context.Context, params CreateEmailLogParams) (EmailLog, error) {
	var log EmailLog
	err := s.db.GetContext(ctx, &log, sqlCreateEmailLog,
		params.CampaignID,
		params.UserID,
		params.CampaignTemplateID,
		params.BlastTemplateID,
		params.BlastID,
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
SELECT id, campaign_id, user_id, campaign_template_id, blast_template_id, blast_id, recipient_email, subject, type, status, provider_message_id, sent_at, delivered_at, opened_at, clicked_at, bounced_at, failed_at, error_message, bounce_reason, open_count, click_count, created_at, updated_at
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
SELECT id, campaign_id, user_id, campaign_template_id, blast_template_id, blast_id, recipient_email, subject, type, status, provider_message_id, sent_at, delivered_at, opened_at, clicked_at, bounced_at, failed_at, error_message, bounce_reason, open_count, click_count, created_at, updated_at
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
SELECT id, campaign_id, user_id, campaign_template_id, blast_template_id, blast_id, recipient_email, subject, type, status, provider_message_id, sent_at, delivered_at, opened_at, clicked_at, bounced_at, failed_at, error_message, bounce_reason, open_count, click_count, created_at, updated_at
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
SELECT id, campaign_id, user_id, campaign_template_id, blast_template_id, blast_id, recipient_email, subject, type, status, provider_message_id, sent_at, delivered_at, opened_at, clicked_at, bounced_at, failed_at, error_message, bounce_reason, open_count, click_count, created_at, updated_at
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
