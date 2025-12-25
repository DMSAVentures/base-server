package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// CreateCampaignTrackingIntegrationParams represents parameters for creating a tracking integration
type CreateCampaignTrackingIntegrationParams struct {
	CampaignID      uuid.UUID
	IntegrationType TrackingIntegrationType
	Enabled         bool
	TrackingID      string
	TrackingLabel   *string
}

// UpdateCampaignTrackingIntegrationParams represents parameters for updating a tracking integration
type UpdateCampaignTrackingIntegrationParams struct {
	Enabled       *bool
	TrackingID    *string
	TrackingLabel *string
}

const sqlCreateCampaignTrackingIntegration = `
INSERT INTO campaign_tracking_integrations (campaign_id, integration_type, enabled, tracking_id, tracking_label)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, campaign_id, integration_type, enabled, tracking_id, tracking_label, created_at, updated_at
`

// CreateCampaignTrackingIntegration creates a tracking integration for a campaign
func (s *Store) CreateCampaignTrackingIntegration(ctx context.Context, params CreateCampaignTrackingIntegrationParams) (CampaignTrackingIntegration, error) {
	var integration CampaignTrackingIntegration
	err := s.db.GetContext(ctx, &integration, sqlCreateCampaignTrackingIntegration,
		params.CampaignID,
		params.IntegrationType,
		params.Enabled,
		params.TrackingID,
		params.TrackingLabel)
	if err != nil {
		return CampaignTrackingIntegration{}, fmt.Errorf("failed to create campaign tracking integration: %w", err)
	}
	return integration, nil
}

const sqlGetCampaignTrackingIntegrationByID = `
SELECT id, campaign_id, integration_type, enabled, tracking_id, tracking_label, created_at, updated_at
FROM campaign_tracking_integrations
WHERE id = $1
`

// GetCampaignTrackingIntegrationByID retrieves a tracking integration by ID
func (s *Store) GetCampaignTrackingIntegrationByID(ctx context.Context, integrationID uuid.UUID) (CampaignTrackingIntegration, error) {
	var integration CampaignTrackingIntegration
	err := s.db.GetContext(ctx, &integration, sqlGetCampaignTrackingIntegrationByID, integrationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignTrackingIntegration{}, ErrNotFound
		}
		return CampaignTrackingIntegration{}, fmt.Errorf("failed to get campaign tracking integration by id: %w", err)
	}
	return integration, nil
}

const sqlGetCampaignTrackingIntegrationByType = `
SELECT id, campaign_id, integration_type, enabled, tracking_id, tracking_label, created_at, updated_at
FROM campaign_tracking_integrations
WHERE campaign_id = $1 AND integration_type = $2
`

// GetCampaignTrackingIntegrationByType retrieves a tracking integration by campaign and type
func (s *Store) GetCampaignTrackingIntegrationByType(ctx context.Context, campaignID uuid.UUID, integrationType TrackingIntegrationType) (CampaignTrackingIntegration, error) {
	var integration CampaignTrackingIntegration
	err := s.db.GetContext(ctx, &integration, sqlGetCampaignTrackingIntegrationByType, campaignID, integrationType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignTrackingIntegration{}, ErrNotFound
		}
		return CampaignTrackingIntegration{}, fmt.Errorf("failed to get campaign tracking integration by type: %w", err)
	}
	return integration, nil
}

const sqlGetCampaignTrackingIntegrations = `
SELECT id, campaign_id, integration_type, enabled, tracking_id, tracking_label, created_at, updated_at
FROM campaign_tracking_integrations
WHERE campaign_id = $1
ORDER BY integration_type ASC
`

// GetCampaignTrackingIntegrations retrieves all tracking integrations for a campaign
func (s *Store) GetCampaignTrackingIntegrations(ctx context.Context, campaignID uuid.UUID) ([]CampaignTrackingIntegration, error) {
	var integrations []CampaignTrackingIntegration
	err := s.db.SelectContext(ctx, &integrations, sqlGetCampaignTrackingIntegrations, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign tracking integrations: %w", err)
	}
	return integrations, nil
}

const sqlGetEnabledCampaignTrackingIntegrations = `
SELECT id, campaign_id, integration_type, enabled, tracking_id, tracking_label, created_at, updated_at
FROM campaign_tracking_integrations
WHERE campaign_id = $1 AND enabled = true
ORDER BY integration_type ASC
`

// GetEnabledCampaignTrackingIntegrations retrieves only enabled tracking integrations for a campaign
func (s *Store) GetEnabledCampaignTrackingIntegrations(ctx context.Context, campaignID uuid.UUID) ([]CampaignTrackingIntegration, error) {
	var integrations []CampaignTrackingIntegration
	err := s.db.SelectContext(ctx, &integrations, sqlGetEnabledCampaignTrackingIntegrations, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled campaign tracking integrations: %w", err)
	}
	return integrations, nil
}

const sqlUpdateCampaignTrackingIntegration = `
UPDATE campaign_tracking_integrations
SET enabled = COALESCE($2, enabled),
    tracking_id = COALESCE($3, tracking_id),
    tracking_label = COALESCE($4, tracking_label),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, campaign_id, integration_type, enabled, tracking_id, tracking_label, created_at, updated_at
`

// UpdateCampaignTrackingIntegration updates a tracking integration
func (s *Store) UpdateCampaignTrackingIntegration(ctx context.Context, integrationID uuid.UUID, params UpdateCampaignTrackingIntegrationParams) (CampaignTrackingIntegration, error) {
	var integration CampaignTrackingIntegration
	err := s.db.GetContext(ctx, &integration, sqlUpdateCampaignTrackingIntegration,
		integrationID,
		params.Enabled,
		params.TrackingID,
		params.TrackingLabel)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignTrackingIntegration{}, ErrNotFound
		}
		return CampaignTrackingIntegration{}, fmt.Errorf("failed to update campaign tracking integration: %w", err)
	}
	return integration, nil
}

const sqlDeleteCampaignTrackingIntegration = `
DELETE FROM campaign_tracking_integrations WHERE id = $1
`

// DeleteCampaignTrackingIntegration deletes a tracking integration
func (s *Store) DeleteCampaignTrackingIntegration(ctx context.Context, integrationID uuid.UUID) error {
	result, err := s.db.ExecContext(ctx, sqlDeleteCampaignTrackingIntegration, integrationID)
	if err != nil {
		return fmt.Errorf("failed to delete campaign tracking integration: %w", err)
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

const sqlDeleteCampaignTrackingIntegrationsByCampaignID = `
DELETE FROM campaign_tracking_integrations WHERE campaign_id = $1
`

// DeleteCampaignTrackingIntegrationsByCampaignID deletes all tracking integrations for a campaign
func (s *Store) DeleteCampaignTrackingIntegrationsByCampaignID(ctx context.Context, campaignID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlDeleteCampaignTrackingIntegrationsByCampaignID, campaignID)
	if err != nil {
		return fmt.Errorf("failed to delete campaign tracking integrations: %w", err)
	}
	return nil
}

// UpsertCampaignTrackingIntegration creates or updates a tracking integration for a campaign type
func (s *Store) UpsertCampaignTrackingIntegration(ctx context.Context, params CreateCampaignTrackingIntegrationParams) (CampaignTrackingIntegration, error) {
	// Try to get existing integration for this type
	existing, err := s.GetCampaignTrackingIntegrationByType(ctx, params.CampaignID, params.IntegrationType)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Create new integration
			return s.CreateCampaignTrackingIntegration(ctx, params)
		}
		return CampaignTrackingIntegration{}, err
	}

	// Update existing integration
	updateParams := UpdateCampaignTrackingIntegrationParams{
		Enabled:       &params.Enabled,
		TrackingID:    &params.TrackingID,
		TrackingLabel: params.TrackingLabel,
	}

	return s.UpdateCampaignTrackingIntegration(ctx, existing.ID, updateParams)
}

// ReplaceCampaignTrackingIntegrations deletes all existing integrations and creates new ones
func (s *Store) ReplaceCampaignTrackingIntegrations(ctx context.Context, campaignID uuid.UUID, integrations []CreateCampaignTrackingIntegrationParams) ([]CampaignTrackingIntegration, error) {
	// Delete existing integrations
	err := s.DeleteCampaignTrackingIntegrationsByCampaignID(ctx, campaignID)
	if err != nil {
		return nil, err
	}

	// Create new integrations
	result := make([]CampaignTrackingIntegration, 0, len(integrations))
	for _, params := range integrations {
		integration, err := s.CreateCampaignTrackingIntegration(ctx, params)
		if err != nil {
			return nil, err
		}
		result = append(result, integration)
	}

	return result, nil
}
