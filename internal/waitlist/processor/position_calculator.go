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

	// Check if email verification is required from campaign config
	emailVerificationRequired := false
	if campaign.EmailConfig != nil {
		if verificationRequired, ok := campaign.EmailConfig["verification_required"].(bool); ok {
			emailVerificationRequired = verificationRequired
		}
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "email_verification_required", Value: emailVerificationRequired},
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

	// 3. Calculate positions based on referral count and signup time
	userPositions := pc.calculatePositions(users, emailVerificationRequired)

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
// 1. Sort users by (referral_count DESC, created_at ASC, id ASC)
// 2. Assign positions 1, 2, 3, ... based on sorted order
func (pc *PositionCalculator) calculatePositions(users []store.WaitlistUser, emailVerificationRequired bool) map[uuid.UUID]int {
	// Create a copy of users to sort
	sortedUsers := make([]store.WaitlistUser, len(users))
	copy(sortedUsers, users)

	// Sort by:
	// 1. Referral count DESC (more referrals = better position)
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

		// More referrals = better position (comes first)
		if countI != countJ {
			return countI > countJ
		}

		// Among users with same referral count, earlier signup = better position
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

// CalculateUserPosition calculates and updates position for a single user using formula-based approach
// This method is LOCK-FREE and updates only the specified user's position (single row UPDATE)
// Formula: position = original_position - (referral_count × positions_per_referral)
// This approach eliminates database lock contention by avoiding full campaign scans
func (pc *PositionCalculator) CalculateUserPosition(ctx context.Context, userID uuid.UUID) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "user_id", Value: userID.String()},
		observability.Field{Key: "operation", Value: "calculate_user_position"},
	)

	pc.logger.Info(ctx, "calculating position for user")

	// 1. Get user
	user, err := pc.store.GetWaitlistUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrUserNotFound
		}
		pc.logger.Error(ctx, "failed to get user", err)
		return fmt.Errorf("failed to get user: %w", err)
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_id", Value: user.CampaignID.String()},
	)

	// 2. Get campaign to check configuration
	campaign, err := pc.store.GetCampaignByID(ctx, user.CampaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrCampaignNotFound
		}
		pc.logger.Error(ctx, "failed to get campaign", err)
		return fmt.Errorf("failed to get campaign: %w", err)
	}

	// 3. Get positions_per_referral from campaign config (default 1)
	positionsPerReferral := 1
	if campaign.ReferralConfig != nil {
		if val, ok := campaign.ReferralConfig["positions_per_referral"].(float64); ok {
			positionsPerReferral = int(val)
			// Enforce maximum to prevent abuse
			if positionsPerReferral > 100 {
				positionsPerReferral = 100
			}
			if positionsPerReferral < 1 {
				positionsPerReferral = 1
			}
		}
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "positions_per_referral", Value: positionsPerReferral},
	)

	// 4. Check if email verification is required
	emailVerificationRequired := false
	if campaign.EmailConfig != nil {
		if verificationRequired, ok := campaign.EmailConfig["verification_required"].(bool); ok {
			emailVerificationRequired = verificationRequired
		}
	}

	// 5. Determine which referral count to use
	referralCount := user.ReferralCount
	if emailVerificationRequired {
		referralCount = user.VerifiedReferralCount
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "referral_count", Value: referralCount},
		observability.Field{Key: "original_position", Value: user.OriginalPosition},
	)

	// 6. Calculate new position using formula
	// position = original_position - (referral_count × positions_per_referral)
	newPosition := user.OriginalPosition - (referralCount * positionsPerReferral)
	if newPosition < 1 {
		newPosition = 1
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "old_position", Value: user.Position},
		observability.Field{Key: "new_position", Value: newPosition},
	)

	// 7. Update position only if changed (single row UPDATE - minimal locking)
	if newPosition != user.Position {
		err = pc.store.UpdateWaitlistUserPosition(ctx, userID, newPosition)
		if err != nil {
			pc.logger.Error(ctx, "failed to update user position", err)
			return fmt.Errorf("failed to update position: %w", err)
		}
		pc.logger.Info(ctx, "successfully updated user position")
	} else {
		pc.logger.Info(ctx, "position unchanged, skipping update")
	}

	return nil
}

// ErrUserNotFound is returned when a user is not found
var ErrUserNotFound = errors.New("user not found")
