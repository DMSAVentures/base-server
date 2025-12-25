package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/google/uuid"
)

// PositionCalculator handles asynchronous position calculation for waitlist users
type PositionCalculator struct {
	store  WaitlistStore
	logger *observability.Logger

	// Per-campaign mutex to prevent concurrent position calculations for the same campaign
	campaignLocks sync.Map // map[uuid.UUID]*sync.Mutex
}

// NewPositionCalculator creates a new PositionCalculator
func NewPositionCalculator(store WaitlistStore, logger *observability.Logger) *PositionCalculator {
	return &PositionCalculator{
		store:  store,
		logger: logger,
	}
}

// CalculatePositionsForCampaign calculates and updates positions for all users in a campaign
// This method is idempotent and can be called multiple times safely
func (pc *PositionCalculator) CalculatePositionsForCampaign(ctx context.Context, campaignID uuid.UUID) error {
	// Acquire campaign-specific lock to prevent concurrent calculations
	lock := pc.getCampaignLock(campaignID)
	lock.Lock()
	defer lock.Unlock()

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "operation", Value: "calculate_positions"},
	)

	pc.logger.Info(ctx, "starting position calculation for campaign")

	// 1. Get campaign to check email verification settings
	campaign, err := pc.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrCampaignNotFound
		}
		pc.logger.Error(ctx, "failed to get campaign", err)
		return fmt.Errorf("failed to get campaign: %w", err)
	}

	// Check if email verification is required from campaign email settings
	emailVerificationRequired := false
	if campaign.EmailSettings != nil {
		emailVerificationRequired = campaign.EmailSettings.VerificationRequired
	}

	// Get positions_to_jump from referral settings (number of positions a referred user jumps ahead)
	positionsToJump := 0
	if campaign.ReferralSettings != nil {
		positionsToJump = campaign.ReferralSettings.PositionsToJump
	}

	// Get referrer_positions_to_jump from referral settings (positions the referrer jumps per referral)
	// Default to 1 to maintain backward compatibility (each referral = 1 position)
	referrerPositionsToJump := 1
	if campaign.ReferralSettings != nil && campaign.ReferralSettings.ReferrerPositionsToJump > 0 {
		referrerPositionsToJump = campaign.ReferralSettings.ReferrerPositionsToJump
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "email_verification_required", Value: emailVerificationRequired},
		observability.Field{Key: "positions_to_jump", Value: positionsToJump},
		observability.Field{Key: "referrer_positions_to_jump", Value: referrerPositionsToJump},
	)

	// 2. Get all users for this campaign
	users, err := pc.store.GetAllWaitlistUsersForPositionCalculation(ctx, campaignID)
	if err != nil {
		pc.logger.Error(ctx, "failed to get users for position calculation", err)
		return fmt.Errorf("failed to get users: %w", err)
	}

	if len(users) == 0 {
		pc.logger.Info(ctx, "no users found for position calculation")
		return nil
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "user_count", Value: len(users)},
	)

	// 3. Calculate positions based on referral count, signup time, and referral bonus
	userPositions := pc.calculatePositions(users, emailVerificationRequired, positionsToJump, referrerPositionsToJump)

	// 4. Update all positions in bulk
	userIDs := make([]uuid.UUID, 0, len(userPositions))
	positions := make([]int, 0, len(userPositions))

	for userID, position := range userPositions {
		userIDs = append(userIDs, userID)
		positions = append(positions, position)
	}

	err = pc.store.BulkUpdateWaitlistUserPositions(ctx, userIDs, positions)
	if err != nil {
		pc.logger.Error(ctx, "failed to bulk update positions", err)
		return fmt.Errorf("failed to update positions: %w", err)
	}

	pc.logger.Info(ctx, "successfully calculated and updated positions for campaign")
	return nil
}

// calculatePositions implements the position calculation algorithm
// Algorithm:
// 1. Sort users by (effective_score DESC, created_at ASC, id ASC)
//   - effective_score = (referral_count * referrer_positions_to_jump) + positions_to_jump (if user was referred)
//
// 2. Assign positions 1, 2, 3, ... based on sorted order
func (pc *PositionCalculator) calculatePositions(users []store.WaitlistUser, emailVerificationRequired bool, positionsToJump int, referrerPositionsToJump int) map[uuid.UUID]int {
	// Create a copy of users to sort
	sortedUsers := make([]store.WaitlistUser, len(users))
	copy(sortedUsers, users)

	// Sort by:
	// 1. Effective score DESC (referral count * referrer jump + referee bonus)
	// 2. Created at ASC (earlier signup = better position)
	// 3. ID ASC (tiebreaker)
	sort.Slice(sortedUsers, func(i, j int) bool {
		userI := sortedUsers[i]
		userJ := sortedUsers[j]

		// Determine which referral count to use based on email verification requirement
		var countI, countJ int
		if emailVerificationRequired {
			countI = userI.VerifiedReferralCount
			countJ = userJ.VerifiedReferralCount
		} else {
			countI = userI.ReferralCount
			countJ = userJ.ReferralCount
		}

		// Multiply referral count by referrer_positions_to_jump
		// This means each referral is worth N positions for the referrer
		countI = countI * referrerPositionsToJump
		countJ = countJ * referrerPositionsToJump

		// Add positions_to_jump bonus for users who were referred (referee bonus)
		if userI.ReferredByID != nil && positionsToJump > 0 {
			countI += positionsToJump
		}
		if userJ.ReferredByID != nil && positionsToJump > 0 {
			countJ += positionsToJump
		}

		// More referrals/points = better position (comes first)
		if countI != countJ {
			return countI > countJ
		}

		// Among users with same score, earlier signup = better position
		if !userI.CreatedAt.Equal(userJ.CreatedAt) {
			return userI.CreatedAt.Before(userJ.CreatedAt)
		}

		// Tiebreaker: sort by ID
		return userI.ID.String() < userJ.ID.String()
	})

	// Assign positions 1, 2, 3, ... based on sorted order
	positions := make(map[uuid.UUID]int, len(sortedUsers))
	for i, user := range sortedUsers {
		positions[user.ID] = i + 1
	}

	return positions
}

// getCampaignLock gets or creates a mutex for the given campaign
func (pc *PositionCalculator) getCampaignLock(campaignID uuid.UUID) *sync.Mutex {
	actual, _ := pc.campaignLocks.LoadOrStore(campaignID, &sync.Mutex{})
	return actual.(*sync.Mutex)
}
