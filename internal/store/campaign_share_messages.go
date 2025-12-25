package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// CreateCampaignShareMessageParams represents parameters for creating a share message
type CreateCampaignShareMessageParams struct {
	CampaignID uuid.UUID
	Channel    SharingChannel
	Message    string
}

// UpdateCampaignShareMessageParams represents parameters for updating a share message
type UpdateCampaignShareMessageParams struct {
	Message *string
}

const sqlCreateCampaignShareMessage = `
INSERT INTO campaign_share_messages (campaign_id, channel, message)
VALUES ($1, $2, $3)
RETURNING id, campaign_id, channel, message, created_at, updated_at
`

// CreateCampaignShareMessage creates a share message for a campaign
func (s *Store) CreateCampaignShareMessage(ctx context.Context, params CreateCampaignShareMessageParams) (CampaignShareMessage, error) {
	var message CampaignShareMessage
	err := s.db.GetContext(ctx, &message, sqlCreateCampaignShareMessage,
		params.CampaignID,
		params.Channel,
		params.Message)
	if err != nil {
		s.logger.Error(ctx, "failed to create campaign share message", err)
		return CampaignShareMessage{}, fmt.Errorf("failed to create campaign share message: %w", err)
	}
	return message, nil
}

const sqlGetCampaignShareMessageByID = `
SELECT id, campaign_id, channel, message, created_at, updated_at
FROM campaign_share_messages
WHERE id = $1
`

// GetCampaignShareMessageByID retrieves a share message by ID
func (s *Store) GetCampaignShareMessageByID(ctx context.Context, messageID uuid.UUID) (CampaignShareMessage, error) {
	var message CampaignShareMessage
	err := s.db.GetContext(ctx, &message, sqlGetCampaignShareMessageByID, messageID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignShareMessage{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get campaign share message by id", err)
		return CampaignShareMessage{}, fmt.Errorf("failed to get campaign share message by id: %w", err)
	}
	return message, nil
}

const sqlGetCampaignShareMessageByChannel = `
SELECT id, campaign_id, channel, message, created_at, updated_at
FROM campaign_share_messages
WHERE campaign_id = $1 AND channel = $2
`

// GetCampaignShareMessageByChannel retrieves a share message by campaign and channel
func (s *Store) GetCampaignShareMessageByChannel(ctx context.Context, campaignID uuid.UUID, channel SharingChannel) (CampaignShareMessage, error) {
	var message CampaignShareMessage
	err := s.db.GetContext(ctx, &message, sqlGetCampaignShareMessageByChannel, campaignID, channel)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignShareMessage{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get campaign share message by channel", err)
		return CampaignShareMessage{}, fmt.Errorf("failed to get campaign share message by channel: %w", err)
	}
	return message, nil
}

const sqlGetCampaignShareMessages = `
SELECT id, campaign_id, channel, message, created_at, updated_at
FROM campaign_share_messages
WHERE campaign_id = $1
ORDER BY channel ASC
`

// GetCampaignShareMessages retrieves all share messages for a campaign
func (s *Store) GetCampaignShareMessages(ctx context.Context, campaignID uuid.UUID) ([]CampaignShareMessage, error) {
	var messages []CampaignShareMessage
	err := s.db.SelectContext(ctx, &messages, sqlGetCampaignShareMessages, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to get campaign share messages", err)
		return nil, fmt.Errorf("failed to get campaign share messages: %w", err)
	}
	return messages, nil
}

const sqlUpdateCampaignShareMessage = `
UPDATE campaign_share_messages
SET message = COALESCE($2, message),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, campaign_id, channel, message, created_at, updated_at
`

// UpdateCampaignShareMessage updates a share message
func (s *Store) UpdateCampaignShareMessage(ctx context.Context, messageID uuid.UUID, params UpdateCampaignShareMessageParams) (CampaignShareMessage, error) {
	var message CampaignShareMessage
	err := s.db.GetContext(ctx, &message, sqlUpdateCampaignShareMessage,
		messageID,
		params.Message)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignShareMessage{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to update campaign share message", err)
		return CampaignShareMessage{}, fmt.Errorf("failed to update campaign share message: %w", err)
	}
	return message, nil
}

const sqlDeleteCampaignShareMessage = `
DELETE FROM campaign_share_messages WHERE id = $1
`

// DeleteCampaignShareMessage deletes a share message
func (s *Store) DeleteCampaignShareMessage(ctx context.Context, messageID uuid.UUID) error {
	result, err := s.db.ExecContext(ctx, sqlDeleteCampaignShareMessage, messageID)
	if err != nil {
		s.logger.Error(ctx, "failed to delete campaign share message", err)
		return fmt.Errorf("failed to delete campaign share message: %w", err)
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

const sqlDeleteCampaignShareMessagesByCampaignID = `
DELETE FROM campaign_share_messages WHERE campaign_id = $1
`

// DeleteCampaignShareMessagesByCampaignID deletes all share messages for a campaign
func (s *Store) DeleteCampaignShareMessagesByCampaignID(ctx context.Context, campaignID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlDeleteCampaignShareMessagesByCampaignID, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to delete campaign share messages", err)
		return fmt.Errorf("failed to delete campaign share messages: %w", err)
	}
	return nil
}

// UpsertCampaignShareMessage creates or updates a share message for a campaign channel
func (s *Store) UpsertCampaignShareMessage(ctx context.Context, params CreateCampaignShareMessageParams) (CampaignShareMessage, error) {
	// Try to get existing message for this channel
	existing, err := s.GetCampaignShareMessageByChannel(ctx, params.CampaignID, params.Channel)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Create new message
			return s.CreateCampaignShareMessage(ctx, params)
		}
		return CampaignShareMessage{}, err
	}

	// Update existing message
	updateParams := UpdateCampaignShareMessageParams{
		Message: &params.Message,
	}

	return s.UpdateCampaignShareMessage(ctx, existing.ID, updateParams)
}

// ReplaceCampaignShareMessages deletes all existing messages and creates new ones
func (s *Store) ReplaceCampaignShareMessages(ctx context.Context, campaignID uuid.UUID, messages []CreateCampaignShareMessageParams) ([]CampaignShareMessage, error) {
	// Delete existing messages
	err := s.DeleteCampaignShareMessagesByCampaignID(ctx, campaignID)
	if err != nil {
		return nil, err
	}

	// Create new messages
	result := make([]CampaignShareMessage, 0, len(messages))
	for _, params := range messages {
		msg, err := s.CreateCampaignShareMessage(ctx, params)
		if err != nil {
			return nil, err
		}
		result = append(result, msg)
	}

	return result, nil
}
