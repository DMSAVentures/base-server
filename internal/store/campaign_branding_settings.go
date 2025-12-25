package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// CreateCampaignBrandingSettingsParams represents parameters for creating branding settings
type CreateCampaignBrandingSettingsParams struct {
	CampaignID   uuid.UUID
	LogoURL      *string
	PrimaryColor *string
	FontFamily   *string
	CustomDomain *string
}

// UpdateCampaignBrandingSettingsParams represents parameters for updating branding settings
type UpdateCampaignBrandingSettingsParams struct {
	LogoURL      *string
	PrimaryColor *string
	FontFamily   *string
	CustomDomain *string
}

const sqlCreateCampaignBrandingSettings = `
INSERT INTO campaign_branding_settings (campaign_id, logo_url, primary_color, font_family, custom_domain)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, campaign_id, logo_url, primary_color, font_family, custom_domain, created_at, updated_at
`

// CreateCampaignBrandingSettings creates branding settings for a campaign
func (s *Store) CreateCampaignBrandingSettings(ctx context.Context, params CreateCampaignBrandingSettingsParams) (CampaignBrandingSettings, error) {
	var settings CampaignBrandingSettings
	err := s.db.GetContext(ctx, &settings, sqlCreateCampaignBrandingSettings,
		params.CampaignID,
		params.LogoURL,
		params.PrimaryColor,
		params.FontFamily,
		params.CustomDomain)
	if err != nil {
		s.logger.Error(ctx, "failed to create campaign branding settings", err)
		return CampaignBrandingSettings{}, fmt.Errorf("failed to create campaign branding settings: %w", err)
	}
	return settings, nil
}

const sqlGetCampaignBrandingSettings = `
SELECT id, campaign_id, logo_url, primary_color, font_family, custom_domain, created_at, updated_at
FROM campaign_branding_settings
WHERE campaign_id = $1
`

// GetCampaignBrandingSettings retrieves branding settings for a campaign
func (s *Store) GetCampaignBrandingSettings(ctx context.Context, campaignID uuid.UUID) (CampaignBrandingSettings, error) {
	var settings CampaignBrandingSettings
	err := s.db.GetContext(ctx, &settings, sqlGetCampaignBrandingSettings, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignBrandingSettings{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get campaign branding settings", err)
		return CampaignBrandingSettings{}, fmt.Errorf("failed to get campaign branding settings: %w", err)
	}
	return settings, nil
}

const sqlUpdateCampaignBrandingSettings = `
UPDATE campaign_branding_settings
SET logo_url = COALESCE($2, logo_url),
    primary_color = COALESCE($3, primary_color),
    font_family = COALESCE($4, font_family),
    custom_domain = COALESCE($5, custom_domain),
    updated_at = CURRENT_TIMESTAMP
WHERE campaign_id = $1
RETURNING id, campaign_id, logo_url, primary_color, font_family, custom_domain, created_at, updated_at
`

// UpdateCampaignBrandingSettings updates branding settings for a campaign
func (s *Store) UpdateCampaignBrandingSettings(ctx context.Context, campaignID uuid.UUID, params UpdateCampaignBrandingSettingsParams) (CampaignBrandingSettings, error) {
	var settings CampaignBrandingSettings
	err := s.db.GetContext(ctx, &settings, sqlUpdateCampaignBrandingSettings,
		campaignID,
		params.LogoURL,
		params.PrimaryColor,
		params.FontFamily,
		params.CustomDomain)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignBrandingSettings{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to update campaign branding settings", err)
		return CampaignBrandingSettings{}, fmt.Errorf("failed to update campaign branding settings: %w", err)
	}
	return settings, nil
}

const sqlDeleteCampaignBrandingSettings = `
DELETE FROM campaign_branding_settings WHERE campaign_id = $1
`

// DeleteCampaignBrandingSettings deletes branding settings for a campaign
func (s *Store) DeleteCampaignBrandingSettings(ctx context.Context, campaignID uuid.UUID) error {
	result, err := s.db.ExecContext(ctx, sqlDeleteCampaignBrandingSettings, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to delete campaign branding settings", err)
		return fmt.Errorf("failed to delete campaign branding settings: %w", err)
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

// UpsertCampaignBrandingSettings creates or updates branding settings for a campaign
func (s *Store) UpsertCampaignBrandingSettings(ctx context.Context, params CreateCampaignBrandingSettingsParams) (CampaignBrandingSettings, error) {
	// Try to get existing settings
	_, err := s.GetCampaignBrandingSettings(ctx, params.CampaignID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Create new settings
			return s.CreateCampaignBrandingSettings(ctx, params)
		}
		return CampaignBrandingSettings{}, err
	}

	// Update existing settings
	updateParams := UpdateCampaignBrandingSettingsParams{
		LogoURL:      params.LogoURL,
		PrimaryColor: params.PrimaryColor,
		FontFamily:   params.FontFamily,
		CustomDomain: params.CustomDomain,
	}

	return s.UpdateCampaignBrandingSettings(ctx, params.CampaignID, updateParams)
}
