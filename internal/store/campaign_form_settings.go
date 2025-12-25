package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// CreateCampaignFormSettingsParams represents parameters for creating form settings
type CreateCampaignFormSettingsParams struct {
	CampaignID      uuid.UUID
	CaptchaEnabled  bool
	CaptchaProvider *CaptchaProvider
	CaptchaSiteKey  *string
	DoubleOptIn     bool
	Design          JSONB
	SuccessTitle    *string
	SuccessMessage  *string
}

// UpdateCampaignFormSettingsParams represents parameters for updating form settings
type UpdateCampaignFormSettingsParams struct {
	CaptchaEnabled  *bool
	CaptchaProvider *CaptchaProvider
	CaptchaSiteKey  *string
	DoubleOptIn     *bool
	Design          JSONB
	SuccessTitle    *string
	SuccessMessage  *string
}

const sqlCreateCampaignFormSettings = `
INSERT INTO campaign_form_settings (campaign_id, captcha_enabled, captcha_provider, captcha_site_key, double_opt_in, design, success_title, success_message)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, campaign_id, captcha_enabled, captcha_provider, captcha_site_key, double_opt_in, design, success_title, success_message, created_at, updated_at
`

// CreateCampaignFormSettings creates form settings for a campaign
func (s *Store) CreateCampaignFormSettings(ctx context.Context, params CreateCampaignFormSettingsParams) (CampaignFormSettings, error) {
	var settings CampaignFormSettings
	err := s.db.GetContext(ctx, &settings, sqlCreateCampaignFormSettings,
		params.CampaignID,
		params.CaptchaEnabled,
		params.CaptchaProvider,
		params.CaptchaSiteKey,
		params.DoubleOptIn,
		params.Design,
		params.SuccessTitle,
		params.SuccessMessage)
	if err != nil {
		return CampaignFormSettings{}, fmt.Errorf("failed to create campaign form settings: %w", err)
	}
	return settings, nil
}

const sqlGetCampaignFormSettings = `
SELECT id, campaign_id, captcha_enabled, captcha_provider, captcha_site_key, double_opt_in, design, success_title, success_message, created_at, updated_at
FROM campaign_form_settings
WHERE campaign_id = $1
`

// GetCampaignFormSettings retrieves form settings for a campaign
func (s *Store) GetCampaignFormSettings(ctx context.Context, campaignID uuid.UUID) (CampaignFormSettings, error) {
	var settings CampaignFormSettings
	err := s.db.GetContext(ctx, &settings, sqlGetCampaignFormSettings, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignFormSettings{}, ErrNotFound
		}
		return CampaignFormSettings{}, fmt.Errorf("failed to get campaign form settings: %w", err)
	}
	return settings, nil
}

const sqlUpdateCampaignFormSettings = `
UPDATE campaign_form_settings
SET captcha_enabled = COALESCE($2, captcha_enabled),
    captcha_provider = COALESCE($3, captcha_provider),
    captcha_site_key = COALESCE($4, captcha_site_key),
    double_opt_in = COALESCE($5, double_opt_in),
    design = COALESCE($6, design),
    success_title = COALESCE($7, success_title),
    success_message = COALESCE($8, success_message),
    updated_at = CURRENT_TIMESTAMP
WHERE campaign_id = $1
RETURNING id, campaign_id, captcha_enabled, captcha_provider, captcha_site_key, double_opt_in, design, success_title, success_message, created_at, updated_at
`

// UpdateCampaignFormSettings updates form settings for a campaign
func (s *Store) UpdateCampaignFormSettings(ctx context.Context, campaignID uuid.UUID, params UpdateCampaignFormSettingsParams) (CampaignFormSettings, error) {
	var settings CampaignFormSettings
	err := s.db.GetContext(ctx, &settings, sqlUpdateCampaignFormSettings,
		campaignID,
		params.CaptchaEnabled,
		params.CaptchaProvider,
		params.CaptchaSiteKey,
		params.DoubleOptIn,
		params.Design,
		params.SuccessTitle,
		params.SuccessMessage)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignFormSettings{}, ErrNotFound
		}
		return CampaignFormSettings{}, fmt.Errorf("failed to update campaign form settings: %w", err)
	}
	return settings, nil
}

const sqlDeleteCampaignFormSettings = `
DELETE FROM campaign_form_settings WHERE campaign_id = $1
`

// DeleteCampaignFormSettings deletes form settings for a campaign
func (s *Store) DeleteCampaignFormSettings(ctx context.Context, campaignID uuid.UUID) error {
	result, err := s.db.ExecContext(ctx, sqlDeleteCampaignFormSettings, campaignID)
	if err != nil {
		return fmt.Errorf("failed to delete campaign form settings: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// UpsertCampaignFormSettings creates or updates form settings for a campaign
func (s *Store) UpsertCampaignFormSettings(ctx context.Context, params CreateCampaignFormSettingsParams) (CampaignFormSettings, error) {
	// Try to get existing settings
	_, err := s.GetCampaignFormSettings(ctx, params.CampaignID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Create new settings
			return s.CreateCampaignFormSettings(ctx, params)
		}
		return CampaignFormSettings{}, err
	}

	// Update existing settings
	updateParams := UpdateCampaignFormSettingsParams{
		CaptchaEnabled:  &params.CaptchaEnabled,
		CaptchaProvider: params.CaptchaProvider,
		CaptchaSiteKey:  params.CaptchaSiteKey,
		DoubleOptIn:     &params.DoubleOptIn,
		Design:          params.Design,
		SuccessTitle:    params.SuccessTitle,
		SuccessMessage:  params.SuccessMessage,
	}

	return s.UpdateCampaignFormSettings(ctx, params.CampaignID, updateParams)
}
