package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateRewardParams represents parameters for creating a reward
type CreateRewardParams struct {
	CampaignID     uuid.UUID
	Name           string
	Description    *string
	Type           string
	Config         JSONB
	TriggerType    string
	TriggerConfig  JSONB
	DeliveryMethod string
	DeliveryConfig JSONB
	TotalAvailable *int
	UserLimit      int
	StartsAt       *time.Time
	ExpiresAt      *time.Time
}

const sqlCreateReward = `
INSERT INTO rewards (campaign_id, name, description, type, config, trigger_type, trigger_config, delivery_method, delivery_config, total_available, user_limit, starts_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id, campaign_id, name, description, type, config, trigger_type, trigger_config, delivery_method, delivery_config, total_available, total_claimed, user_limit, status, starts_at, expires_at, created_at, updated_at, deleted_at
`

// CreateReward creates a new reward
func (s *Store) CreateReward(ctx context.Context, params CreateRewardParams) (Reward, error) {
	var reward Reward
	err := s.db.GetContext(ctx, &reward, sqlCreateReward,
		params.CampaignID,
		params.Name,
		params.Description,
		params.Type,
		params.Config,
		params.TriggerType,
		params.TriggerConfig,
		params.DeliveryMethod,
		params.DeliveryConfig,
		params.TotalAvailable,
		params.UserLimit,
		params.StartsAt,
		params.ExpiresAt)
	if err != nil {
		return Reward{}, fmt.Errorf("failed to create reward: %w", err)
	}
	return reward, nil
}

const sqlGetRewardByID = `
SELECT id, campaign_id, name, description, type, config, trigger_type, trigger_config, delivery_method, delivery_config, total_available, total_claimed, user_limit, status, starts_at, expires_at, created_at, updated_at, deleted_at
FROM rewards
WHERE id = $1 AND deleted_at IS NULL
`

// GetRewardByID retrieves a reward by ID
func (s *Store) GetRewardByID(ctx context.Context, rewardID uuid.UUID) (Reward, error) {
	var reward Reward
	err := s.db.GetContext(ctx, &reward, sqlGetRewardByID, rewardID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Reward{}, ErrNotFound
		}
		return Reward{}, fmt.Errorf("failed to get reward by id: %w", err)
	}
	return reward, nil
}

const sqlGetRewardsByCampaign = `
SELECT id, campaign_id, name, description, type, config, trigger_type, trigger_config, delivery_method, delivery_config, total_available, total_claimed, user_limit, status, starts_at, expires_at, created_at, updated_at, deleted_at
FROM rewards
WHERE campaign_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
`

// GetRewardsByCampaign retrieves all rewards for a campaign
func (s *Store) GetRewardsByCampaign(ctx context.Context, campaignID uuid.UUID) ([]Reward, error) {
	var rewards []Reward
	err := s.db.SelectContext(ctx, &rewards, sqlGetRewardsByCampaign, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to get rewards by campaign: %w", err)
	}
	return rewards, nil
}

const sqlGetActiveRewardsByCampaign = `
SELECT id, campaign_id, name, description, type, config, trigger_type, trigger_config, delivery_method, delivery_config, total_available, total_claimed, user_limit, status, starts_at, expires_at, created_at, updated_at, deleted_at
FROM rewards
WHERE campaign_id = $1 AND status = 'active' AND deleted_at IS NULL
  AND (starts_at IS NULL OR starts_at <= CURRENT_TIMESTAMP)
  AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
ORDER BY created_at DESC
`

// GetActiveRewardsByCampaign retrieves active rewards for a campaign
func (s *Store) GetActiveRewardsByCampaign(ctx context.Context, campaignID uuid.UUID) ([]Reward, error) {
	var rewards []Reward
	err := s.db.SelectContext(ctx, &rewards, sqlGetActiveRewardsByCampaign, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active rewards: %w", err)
	}
	return rewards, nil
}

const sqlUpdateReward = `
UPDATE rewards
SET name = COALESCE($2, name),
    description = COALESCE($3, description),
    config = COALESCE($4, config),
    trigger_config = COALESCE($5, trigger_config),
    delivery_config = COALESCE($6, delivery_config),
    status = COALESCE($7, status),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, campaign_id, name, description, type, config, trigger_type, trigger_config, delivery_method, delivery_config, total_available, total_claimed, user_limit, status, starts_at, expires_at, created_at, updated_at, deleted_at
`

// UpdateRewardParams represents parameters for updating a reward
type UpdateRewardParams struct {
	Name           *string
	Description    *string
	Config         JSONB
	TriggerConfig  JSONB
	DeliveryConfig JSONB
	Status         *string
}

// UpdateReward updates a reward
func (s *Store) UpdateReward(ctx context.Context, rewardID uuid.UUID, params UpdateRewardParams) (Reward, error) {
	var reward Reward
	err := s.db.GetContext(ctx, &reward, sqlUpdateReward,
		rewardID,
		params.Name,
		params.Description,
		params.Config,
		params.TriggerConfig,
		params.DeliveryConfig,
		params.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Reward{}, ErrNotFound
		}
		return Reward{}, fmt.Errorf("failed to update reward: %w", err)
	}
	return reward, nil
}

const sqlDeleteReward = `
UPDATE rewards
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// DeleteReward soft deletes a reward
func (s *Store) DeleteReward(ctx context.Context, rewardID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, sqlDeleteReward, rewardID)
	if err != nil {
		return fmt.Errorf("failed to delete reward: %w", err)
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

const sqlIncrementRewardClaimed = `
UPDATE rewards
SET total_claimed = total_claimed + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// IncrementRewardClaimed increments the total claimed count for a reward
func (s *Store) IncrementRewardClaimed(ctx context.Context, rewardID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlIncrementRewardClaimed, rewardID)
	if err != nil {
		return fmt.Errorf("failed to increment reward claimed: %w", err)
	}
	return nil
}

// User Reward operations

// CreateUserRewardParams represents parameters for creating a user reward
type CreateUserRewardParams struct {
	UserID     uuid.UUID
	RewardID   uuid.UUID
	CampaignID uuid.UUID
	RewardData JSONB
	ExpiresAt  *time.Time
}

const sqlCreateUserReward = `
INSERT INTO user_rewards (user_id, reward_id, campaign_id, reward_data, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, user_id, reward_id, campaign_id, status, reward_data, earned_at, delivered_at, redeemed_at, revoked_at, expires_at, delivery_attempts, last_delivery_attempt_at, delivery_error, revoked_reason, revoked_by, created_at, updated_at
`

// CreateUserReward creates a user reward record
func (s *Store) CreateUserReward(ctx context.Context, params CreateUserRewardParams) (UserReward, error) {
	var userReward UserReward
	err := s.db.GetContext(ctx, &userReward, sqlCreateUserReward,
		params.UserID,
		params.RewardID,
		params.CampaignID,
		params.RewardData,
		params.ExpiresAt)
	if err != nil {
		return UserReward{}, fmt.Errorf("failed to create user reward: %w", err)
	}
	return userReward, nil
}

const sqlGetUserRewardsByUser = `
SELECT id, user_id, reward_id, campaign_id, status, reward_data, earned_at, delivered_at, redeemed_at, revoked_at, expires_at, delivery_attempts, last_delivery_attempt_at, delivery_error, revoked_reason, revoked_by, created_at, updated_at
FROM user_rewards
WHERE user_id = $1
ORDER BY earned_at DESC
`

// GetUserRewardsByUser retrieves all rewards for a user
func (s *Store) GetUserRewardsByUser(ctx context.Context, userID uuid.UUID) ([]UserReward, error) {
	var rewards []UserReward
	err := s.db.SelectContext(ctx, &rewards, sqlGetUserRewardsByUser, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user rewards: %w", err)
	}
	return rewards, nil
}

const sqlGetUserRewardsByCampaign = `
SELECT id, user_id, reward_id, campaign_id, status, reward_data, earned_at, delivered_at, redeemed_at, revoked_at, expires_at, delivery_attempts, last_delivery_attempt_at, delivery_error, revoked_reason, revoked_by, created_at, updated_at
FROM user_rewards
WHERE campaign_id = $1
ORDER BY earned_at DESC
LIMIT $2 OFFSET $3
`

// GetUserRewardsByCampaign retrieves user rewards for a campaign with pagination
func (s *Store) GetUserRewardsByCampaign(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]UserReward, error) {
	var rewards []UserReward
	err := s.db.SelectContext(ctx, &rewards, sqlGetUserRewardsByCampaign, campaignID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get user rewards by campaign: %w", err)
	}
	return rewards, nil
}

const sqlUpdateUserRewardStatus = `
UPDATE user_rewards
SET status = $2,
    delivered_at = CASE WHEN $2 = 'delivered' THEN COALESCE(delivered_at, CURRENT_TIMESTAMP) ELSE delivered_at END,
    redeemed_at = CASE WHEN $2 = 'redeemed' THEN CURRENT_TIMESTAMP ELSE redeemed_at END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// UpdateUserRewardStatus updates the status of a user reward
func (s *Store) UpdateUserRewardStatus(ctx context.Context, userRewardID uuid.UUID, status string) error {
	res, err := s.db.ExecContext(ctx, sqlUpdateUserRewardStatus, userRewardID, status)
	if err != nil {
		return fmt.Errorf("failed to update user reward status: %w", err)
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

const sqlIncrementDeliveryAttempts = `
UPDATE user_rewards
SET delivery_attempts = delivery_attempts + 1,
    last_delivery_attempt_at = CURRENT_TIMESTAMP,
    delivery_error = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// IncrementDeliveryAttempts increments delivery attempts and records error
func (s *Store) IncrementDeliveryAttempts(ctx context.Context, userRewardID uuid.UUID, errorMsg string) error {
	_, err := s.db.ExecContext(ctx, sqlIncrementDeliveryAttempts, userRewardID, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to increment delivery attempts: %w", err)
	}
	return nil
}

const sqlGetPendingUserRewards = `
SELECT id, user_id, reward_id, campaign_id, status, reward_data, earned_at, delivered_at, redeemed_at, revoked_at, expires_at, delivery_attempts, last_delivery_attempt_at, delivery_error, revoked_reason, revoked_by, created_at, updated_at
FROM user_rewards
WHERE status = 'pending' AND delivery_attempts < 5
ORDER BY created_at ASC
LIMIT $1
`

// GetPendingUserRewards retrieves pending user rewards for delivery
func (s *Store) GetPendingUserRewards(ctx context.Context, limit int) ([]UserReward, error) {
	var rewards []UserReward
	err := s.db.SelectContext(ctx, &rewards, sqlGetPendingUserRewards, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending user rewards: %w", err)
	}
	return rewards, nil
}
