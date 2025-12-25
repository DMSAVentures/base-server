package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// CreateCampaignReferralSettingsParams represents parameters for creating referral settings
type CreateCampaignReferralSettingsParams struct {
	CampaignID              uuid.UUID
	Enabled                 bool
	PointsPerReferral       int
	VerifiedOnly            bool
	PositionsToJump         int
	ReferrerPositionsToJump int
	SharingChannels         SharingChannelArray
}

// UpdateCampaignReferralSettingsParams represents parameters for updating referral settings
type UpdateCampaignReferralSettingsParams struct {
	Enabled                 *bool
	PointsPerReferral       *int
	VerifiedOnly            *bool
	PositionsToJump         *int
	ReferrerPositionsToJump *int
	SharingChannels         SharingChannelArray
}

const sqlCreateCampaignReferralSettings = `
INSERT INTO campaign_referral_settings (campaign_id, enabled, points_per_referral, verified_only, positions_to_jump, referrer_positions_to_jump, sharing_channels)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, campaign_id, enabled, points_per_referral, verified_only, positions_to_jump, referrer_positions_to_jump, sharing_channels, created_at, updated_at
`

// CreateCampaignReferralSettings creates referral settings for a campaign
func (s *Store) CreateCampaignReferralSettings(ctx context.Context, params CreateCampaignReferralSettingsParams) (CampaignReferralSettings, error) {
	var settings CampaignReferralSettings
	err := s.db.GetContext(ctx, &settings, sqlCreateCampaignReferralSettings,
		params.CampaignID,
		params.Enabled,
		params.PointsPerReferral,
		params.VerifiedOnly,
		params.PositionsToJump,
		params.ReferrerPositionsToJump,
		params.SharingChannels)
	if err != nil {
		s.logger.Error(ctx, "failed to create campaign referral settings", err)
		return CampaignReferralSettings{}, fmt.Errorf("failed to create campaign referral settings: %w", err)
	}
	return settings, nil
}

const sqlGetCampaignReferralSettings = `
SELECT id, campaign_id, enabled, points_per_referral, verified_only, positions_to_jump, referrer_positions_to_jump, sharing_channels, created_at, updated_at
FROM campaign_referral_settings
WHERE campaign_id = $1
`

// GetCampaignReferralSettings retrieves referral settings for a campaign
func (s *Store) GetCampaignReferralSettings(ctx context.Context, campaignID uuid.UUID) (CampaignReferralSettings, error) {
	var settings CampaignReferralSettings
	err := s.db.GetContext(ctx, &settings, sqlGetCampaignReferralSettings, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignReferralSettings{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get campaign referral settings", err)
		return CampaignReferralSettings{}, fmt.Errorf("failed to get campaign referral settings: %w", err)
	}
	return settings, nil
}

const sqlUpdateCampaignReferralSettings = `
UPDATE campaign_referral_settings
SET enabled = COALESCE($2, enabled),
    points_per_referral = COALESCE($3, points_per_referral),
    verified_only = COALESCE($4, verified_only),
    positions_to_jump = COALESCE($5, positions_to_jump),
    referrer_positions_to_jump = COALESCE($6, referrer_positions_to_jump),
    sharing_channels = COALESCE($7, sharing_channels),
    updated_at = CURRENT_TIMESTAMP
WHERE campaign_id = $1
RETURNING id, campaign_id, enabled, points_per_referral, verified_only, positions_to_jump, referrer_positions_to_jump, sharing_channels, created_at, updated_at
`

// UpdateCampaignReferralSettings updates referral settings for a campaign
func (s *Store) UpdateCampaignReferralSettings(ctx context.Context, campaignID uuid.UUID, params UpdateCampaignReferralSettingsParams) (CampaignReferralSettings, error) {
	var settings CampaignReferralSettings
	err := s.db.GetContext(ctx, &settings, sqlUpdateCampaignReferralSettings,
		campaignID,
		params.Enabled,
		params.PointsPerReferral,
		params.VerifiedOnly,
		params.PositionsToJump,
		params.ReferrerPositionsToJump,
		params.SharingChannels)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignReferralSettings{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to update campaign referral settings", err)
		return CampaignReferralSettings{}, fmt.Errorf("failed to update campaign referral settings: %w", err)
	}
	return settings, nil
}

const sqlDeleteCampaignReferralSettings = `
DELETE FROM campaign_referral_settings WHERE campaign_id = $1
`

// DeleteCampaignReferralSettings deletes referral settings for a campaign
func (s *Store) DeleteCampaignReferralSettings(ctx context.Context, campaignID uuid.UUID) error {
	result, err := s.db.ExecContext(ctx, sqlDeleteCampaignReferralSettings, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to delete campaign referral settings", err)
		return fmt.Errorf("failed to delete campaign referral settings: %w", err)
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

// UpsertCampaignReferralSettings creates or updates referral settings for a campaign
func (s *Store) UpsertCampaignReferralSettings(ctx context.Context, params CreateCampaignReferralSettingsParams) (CampaignReferralSettings, error) {
	// Try to get existing settings
	_, err := s.GetCampaignReferralSettings(ctx, params.CampaignID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Create new settings
			return s.CreateCampaignReferralSettings(ctx, params)
		}
		return CampaignReferralSettings{}, err
	}

	// Update existing settings
	updateParams := UpdateCampaignReferralSettingsParams{
		Enabled:                 &params.Enabled,
		PointsPerReferral:       &params.PointsPerReferral,
		VerifiedOnly:            &params.VerifiedOnly,
		PositionsToJump:         &params.PositionsToJump,
		ReferrerPositionsToJump: &params.ReferrerPositionsToJump,
		SharingChannels:         params.SharingChannels,
	}

	return s.UpdateCampaignReferralSettings(ctx, params.CampaignID, updateParams)
}
