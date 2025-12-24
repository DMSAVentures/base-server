package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Channel code SQL queries
const (
	sqlCreateUserChannelCode = `
		INSERT INTO user_channel_codes (user_id, channel, code)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, channel, code, created_at
	`

	sqlGetUserChannelCodes = `
		SELECT id, user_id, channel, code, created_at
		FROM user_channel_codes
		WHERE user_id = $1
		ORDER BY channel
	`

	sqlGetUserByChannelCode = `
		SELECT wu.id, wu.campaign_id, wu.email, wu.first_name, wu.last_name, wu.status,
			wu.position, wu.original_position, wu.referral_code, wu.referred_by_id,
			wu.referral_count, wu.verified_referral_count, wu.points,
			wu.email_verified, wu.verification_token, wu.verification_sent_at, wu.verified_at,
			wu.source, wu.utm_source, wu.utm_medium, wu.utm_campaign, wu.utm_term, wu.utm_content,
			wu.ip_address, wu.user_agent, wu.country_code, wu.city, wu.device_fingerprint,
			wu.metadata, wu.marketing_consent, wu.marketing_consent_at, wu.terms_accepted, wu.terms_accepted_at,
			wu.last_activity_at, wu.share_count, wu.created_at, wu.updated_at, wu.deleted_at
		FROM waitlist_users wu
		INNER JOIN user_channel_codes ucc ON wu.id = ucc.user_id
		WHERE ucc.code = $1 AND wu.deleted_at IS NULL
	`

	sqlGetChannelByCode = `
		SELECT channel FROM user_channel_codes WHERE code = $1
	`

	sqlDeleteUserChannelCodes = `
		DELETE FROM user_channel_codes WHERE user_id = $1
	`
)

// CreateUserChannelCode creates a single channel code for a user
func (s *Store) CreateUserChannelCode(ctx context.Context, userID uuid.UUID, channel, code string) (UserChannelCode, error) {
	var channelCode UserChannelCode
	err := s.db.GetContext(ctx, &channelCode, sqlCreateUserChannelCode, userID, channel, code)
	if err != nil {
		return UserChannelCode{}, fmt.Errorf("failed to create user channel code: %w", err)
	}
	return channelCode, nil
}

// CreateUserChannelCodes creates multiple channel codes for a user
func (s *Store) CreateUserChannelCodes(ctx context.Context, userID uuid.UUID, codes map[string]string) ([]UserChannelCode, error) {
	var createdCodes []UserChannelCode

	for channel, code := range codes {
		channelCode, err := s.CreateUserChannelCode(ctx, userID, channel, code)
		if err != nil {
			return nil, err
		}
		createdCodes = append(createdCodes, channelCode)
	}

	return createdCodes, nil
}

// GetUserChannelCodes retrieves all channel codes for a user
func (s *Store) GetUserChannelCodes(ctx context.Context, userID uuid.UUID) (map[string]string, error) {
	var codes []UserChannelCode
	err := s.db.SelectContext(ctx, &codes, sqlGetUserChannelCodes, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user channel codes: %w", err)
	}

	result := make(map[string]string)
	for _, c := range codes {
		result[c.Channel] = c.Code
	}
	return result, nil
}

// GetUserByChannelCode retrieves a user by their channel-specific referral code
func (s *Store) GetUserByChannelCode(ctx context.Context, code string) (*WaitlistUser, string, error) {
	var user WaitlistUser
	err := s.db.GetContext(ctx, &user, sqlGetUserByChannelCode, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", nil // Not found, return nil (not an error)
		}
		return nil, "", fmt.Errorf("failed to get user by channel code: %w", err)
	}

	// Get the channel for this code
	var channel string
	err = s.db.GetContext(ctx, &channel, sqlGetChannelByCode, code)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get channel for code: %w", err)
	}

	return &user, channel, nil
}

// DeleteUserChannelCodes deletes all channel codes for a user
func (s *Store) DeleteUserChannelCodes(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlDeleteUserChannelCodes, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user channel codes: %w", err)
	}
	return nil
}
