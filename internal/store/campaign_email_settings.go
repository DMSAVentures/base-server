package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// CreateCampaignEmailSettingsParams represents parameters for creating email settings
type CreateCampaignEmailSettingsParams struct {
	CampaignID           uuid.UUID
	FromName             *string
	FromEmail            *string
	ReplyTo              *string
	VerificationRequired bool
	SendWelcomeEmail     bool
}

// UpdateCampaignEmailSettingsParams represents parameters for updating email settings
type UpdateCampaignEmailSettingsParams struct {
	FromName             *string
	FromEmail            *string
	ReplyTo              *string
	VerificationRequired *bool
	SendWelcomeEmail     *bool
}

const sqlCreateCampaignEmailSettings = `
INSERT INTO campaign_email_settings (campaign_id, from_name, from_email, reply_to, verification_required, send_welcome_email)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, campaign_id, from_name, from_email, reply_to, verification_required, send_welcome_email, created_at, updated_at
`

// CreateCampaignEmailSettings creates email settings for a campaign
func (s *Store) CreateCampaignEmailSettings(ctx context.Context, params CreateCampaignEmailSettingsParams) (CampaignEmailSettings, error) {
	var settings CampaignEmailSettings
	err := s.db.GetContext(ctx, &settings, sqlCreateCampaignEmailSettings,
		params.CampaignID,
		params.FromName,
		params.FromEmail,
		params.ReplyTo,
		params.VerificationRequired,
		params.SendWelcomeEmail)
	if err != nil {
		s.logger.Error(ctx, "failed to create campaign email settings", err)
		return CampaignEmailSettings{}, fmt.Errorf("failed to create campaign email settings: %w", err)
	}
	return settings, nil
}

const sqlGetCampaignEmailSettings = `
SELECT id, campaign_id, from_name, from_email, reply_to, verification_required, send_welcome_email, created_at, updated_at
FROM campaign_email_settings
WHERE campaign_id = $1
`

// GetCampaignEmailSettings retrieves email settings for a campaign
func (s *Store) GetCampaignEmailSettings(ctx context.Context, campaignID uuid.UUID) (CampaignEmailSettings, error) {
	var settings CampaignEmailSettings
	err := s.db.GetContext(ctx, &settings, sqlGetCampaignEmailSettings, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignEmailSettings{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get campaign email settings", err)
		return CampaignEmailSettings{}, fmt.Errorf("failed to get campaign email settings: %w", err)
	}
	return settings, nil
}

const sqlUpdateCampaignEmailSettings = `
UPDATE campaign_email_settings
SET from_name = COALESCE($2, from_name),
    from_email = COALESCE($3, from_email),
    reply_to = COALESCE($4, reply_to),
    verification_required = COALESCE($5, verification_required),
    send_welcome_email = COALESCE($6, send_welcome_email),
    updated_at = CURRENT_TIMESTAMP
WHERE campaign_id = $1
RETURNING id, campaign_id, from_name, from_email, reply_to, verification_required, send_welcome_email, created_at, updated_at
`

// UpdateCampaignEmailSettings updates email settings for a campaign
func (s *Store) UpdateCampaignEmailSettings(ctx context.Context, campaignID uuid.UUID, params UpdateCampaignEmailSettingsParams) (CampaignEmailSettings, error) {
	var settings CampaignEmailSettings
	err := s.db.GetContext(ctx, &settings, sqlUpdateCampaignEmailSettings,
		campaignID,
		params.FromName,
		params.FromEmail,
		params.ReplyTo,
		params.VerificationRequired,
		params.SendWelcomeEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignEmailSettings{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to update campaign email settings", err)
		return CampaignEmailSettings{}, fmt.Errorf("failed to update campaign email settings: %w", err)
	}
	return settings, nil
}

const sqlDeleteCampaignEmailSettings = `
DELETE FROM campaign_email_settings WHERE campaign_id = $1
`

// DeleteCampaignEmailSettings deletes email settings for a campaign
func (s *Store) DeleteCampaignEmailSettings(ctx context.Context, campaignID uuid.UUID) error {
	result, err := s.db.ExecContext(ctx, sqlDeleteCampaignEmailSettings, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to delete campaign email settings", err)
		return fmt.Errorf("failed to delete campaign email settings: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		s.logger.Error(ctx, "failed to get rows affected", err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// UpsertCampaignEmailSettings creates or updates email settings for a campaign
func (s *Store) UpsertCampaignEmailSettings(ctx context.Context, params CreateCampaignEmailSettingsParams) (CampaignEmailSettings, error) {
	// Try to get existing settings
	existing, err := s.GetCampaignEmailSettings(ctx, params.CampaignID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Create new settings
			return s.CreateCampaignEmailSettings(ctx, params)
		}
		return CampaignEmailSettings{}, err
	}

	// Update existing settings
	updateParams := UpdateCampaignEmailSettingsParams{
		FromName:             params.FromName,
		FromEmail:            params.FromEmail,
		ReplyTo:              params.ReplyTo,
		VerificationRequired: &params.VerificationRequired,
		SendWelcomeEmail:     &params.SendWelcomeEmail,
	}

	// Only update if different from existing
	if existing.FromName != params.FromName || existing.FromEmail != params.FromEmail ||
		existing.ReplyTo != params.ReplyTo || existing.VerificationRequired != params.VerificationRequired ||
		existing.SendWelcomeEmail != params.SendWelcomeEmail {
		return s.UpdateCampaignEmailSettings(ctx, params.CampaignID, updateParams)
	}

	return existing, nil
}
