package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"base-server/internal/jobs"
	"base-server/internal/observability"
	"base-server/internal/store"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// PositionWorker handles position recalculation jobs
type PositionWorker struct {
	store  *store.Store
	logger *observability.Logger
}

// NewPositionWorker creates a new position worker
func NewPositionWorker(store *store.Store, logger *observability.Logger) *PositionWorker {
	return &PositionWorker{
		store:  store,
		logger: logger,
	}
}

// ProcessPositionRecalcTask processes a position recalculation task
func (w *PositionWorker) ProcessPositionRecalcTask(ctx context.Context, task *asynq.Task) error {
	var payload jobs.PositionRecalcJobPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		w.logger.Error(ctx, "failed to unmarshal position recalc job payload", err)
		return fmt.Errorf("failed to unmarshal position recalc job payload: %w", err)
	}

	// Get campaign to check referral config
	campaign, err := w.store.GetCampaignByID(ctx, payload.CampaignID)
	if err != nil {
		w.logger.Error(ctx, "failed to get campaign", err)
		return fmt.Errorf("failed to get campaign: %w", err)
	}

	// Extract points per referral from config
	pointsPerReferral := 1
	if referralConfig, ok := campaign.ReferralConfig.(map[string]interface{}); ok {
		if points, ok := referralConfig["points_per_referral"].(float64); ok {
			pointsPerReferral = int(points)
		}
	}

	// If specific user IDs are provided, recalculate only those users
	if len(payload.UserIDs) > 0 {
		for _, userID := range payload.UserIDs {
			if err := w.recalculateUserPosition(ctx, payload.CampaignID, userID, pointsPerReferral); err != nil {
				w.logger.Error(ctx, fmt.Sprintf("failed to recalculate position for user %s", userID), err)
				// Continue with other users even if one fails
			}
		}
		return nil
	}

	// Otherwise, recalculate all users in the campaign
	if err := w.recalculateAllPositions(ctx, payload.CampaignID, pointsPerReferral); err != nil {
		w.logger.Error(ctx, "failed to recalculate all positions", err)
		return fmt.Errorf("failed to recalculate all positions: %w", err)
	}

	w.logger.Info(ctx, fmt.Sprintf("successfully recalculated positions for campaign %s", payload.CampaignID))
	return nil
}

// recalculateUserPosition recalculates the position for a specific user
func (w *PositionWorker) recalculateUserPosition(ctx context.Context, campaignID, userID uuid.UUID, pointsPerReferral int) error {
	// Get user
	user, err := w.store.GetWaitlistUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Calculate new position based on the algorithm:
	// Position = Original Position - (Verified Referral Count * Points Per Referral)
	// But the actual position is determined by ranking all users

	// Get the user's rank among all active users
	newPosition, err := w.calculateUserRank(ctx, campaignID, user.OriginalPosition, user.VerifiedReferralCount, pointsPerReferral)
	if err != nil {
		return fmt.Errorf("failed to calculate user rank: %w", err)
	}

	// Update user position if it changed
	if newPosition != user.Position {
		oldPosition := user.Position
		if err := w.store.UpdateWaitlistUserPosition(ctx, userID, newPosition); err != nil {
			return fmt.Errorf("failed to update user position: %w", err)
		}

		w.logger.Info(ctx, fmt.Sprintf("updated position for user %s from %d to %d", userID, oldPosition, newPosition))
	}

	return nil
}

// recalculateAllPositions recalculates positions for all users in a campaign
func (w *PositionWorker) recalculateAllPositions(ctx context.Context, campaignID uuid.UUID, pointsPerReferral int) error {
	// This query recalculates and updates all positions in one go using a CTE
	query := `
		WITH ranked_users AS (
			SELECT
				id,
				original_position - (verified_referral_count * $2) as calculated_position,
				ROW_NUMBER() OVER (ORDER BY (original_position - (verified_referral_count * $2)), created_at) as new_position
			FROM waitlist_users
			WHERE campaign_id = $1
				AND deleted_at IS NULL
				AND status != 'blocked'
		)
		UPDATE waitlist_users wu
		SET position = ru.new_position,
		    updated_at = CURRENT_TIMESTAMP
		FROM ranked_users ru
		WHERE wu.id = ru.id
	`

	_, err := w.store.DB().ExecContext(ctx, query, campaignID, pointsPerReferral)
	if err != nil {
		return fmt.Errorf("failed to execute position recalculation query: %w", err)
	}

	return nil
}

// calculateUserRank calculates the rank/position for a specific user
func (w *PositionWorker) calculateUserRank(ctx context.Context, campaignID uuid.UUID, originalPosition, verifiedReferralCount, pointsPerReferral int) (int, error) {
	// This calculates where the user ranks among all users
	query := `
		WITH ranked_users AS (
			SELECT
				id,
				original_position - (verified_referral_count * $2) as calculated_position,
				ROW_NUMBER() OVER (ORDER BY (original_position - (verified_referral_count * $2)), created_at) as new_position
			FROM waitlist_users
			WHERE campaign_id = $1
				AND deleted_at IS NULL
				AND status != 'blocked'
		)
		SELECT new_position
		FROM ranked_users
		WHERE calculated_position = $3
		ORDER BY calculated_position, new_position
		LIMIT 1
	`

	calculatedPosition := originalPosition - (verifiedReferralCount * pointsPerReferral)

	var position int
	err := w.store.DB().GetContext(ctx, &position, query, campaignID, pointsPerReferral, calculatedPosition)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate user rank: %w", err)
	}

	return position, nil
}
