package leaderboard

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"base-server/internal/waitlist/processor"
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Strategy determines which leaderboard implementation to use
type Strategy string

const (
	StrategyFormula Strategy = "formula" // Formula-based (PostgreSQL only)
	StrategyRedis   Strategy = "redis"   // Redis ZSET (high performance)
	StrategyAuto    Strategy = "auto"    // Automatic selection based on size
)

// Thresholds for automatic strategy selection
const (
	RedisThreshold = 10000 // Use Redis for leaderboards with > 10k users
)

// HybridService orchestrates between formula-based and Redis-based leaderboards
type HybridService struct {
	redisService       *RedisLeaderboardService
	positionCalculator *processor.PositionCalculator
	store              store.Store
	logger             *observability.Logger
}

// NewHybridService creates a new hybrid leaderboard service
func NewHybridService(
	redisService *RedisLeaderboardService,
	positionCalculator *processor.PositionCalculator,
	store store.Store,
	logger *observability.Logger,
) *HybridService {
	return &HybridService{
		redisService:       redisService,
		positionCalculator: positionCalculator,
		store:              store,
		logger:             logger,
	}
}

// selectStrategy determines which strategy to use based on customer and campaign
func (h *HybridService) selectStrategy(ctx context.Context, customer store.Customer, campaignID uuid.UUID, preferredStrategy Strategy) (Strategy, error) {
	// If customer doesn't have Redis enabled in their plan, use formula
	if !customer.RedisEnabled {
		h.logger.Info(ctx, "using formula strategy: customer plan doesn't include Redis")
		return StrategyFormula, nil
	}

	// If explicit strategy requested and allowed, use it
	if preferredStrategy == StrategyFormula || preferredStrategy == StrategyRedis {
		h.logger.Info(ctx, fmt.Sprintf("using %s strategy: explicit customer preference", preferredStrategy))
		return preferredStrategy, nil
	}

	// Auto strategy: decide based on leaderboard size
	count, err := h.store.GetWaitlistUserCount(ctx, campaignID)
	if err != nil {
		h.logger.Error(ctx, "failed to get user count for strategy selection", err)
		// Fall back to formula on error
		return StrategyFormula, nil
	}

	if count > RedisThreshold {
		h.logger.Info(ctx, "using Redis strategy: leaderboard exceeds threshold",
			observability.Field{Key: "user_count", Value: count},
			observability.Field{Key: "threshold", Value: RedisThreshold},
		)
		return StrategyRedis, nil
	}

	h.logger.Info(ctx, "using formula strategy: leaderboard below threshold",
		observability.Field{Key: "user_count", Value: count},
		observability.Field{Key: "threshold", Value: RedisThreshold},
	)
	return StrategyFormula, nil
}

// UpdateUserPosition updates a user's position using the appropriate strategy
func (h *HybridService) UpdateUserPosition(
	ctx context.Context,
	customer store.Customer,
	campaignID, userID uuid.UUID,
	strategy Strategy,
) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customer.ID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
	)

	selectedStrategy, err := h.selectStrategy(ctx, customer, campaignID, strategy)
	if err != nil {
		return fmt.Errorf("failed to select strategy: %w", err)
	}

	switch selectedStrategy {
	case StrategyRedis:
		return h.updateUserPositionRedis(ctx, customer.ID, campaignID, userID)
	case StrategyFormula:
		return h.positionCalculator.CalculateUserPosition(ctx, userID)
	default:
		return fmt.Errorf("unknown strategy: %s", selectedStrategy)
	}
}

// updateUserPositionRedis updates position using Redis ZSET
func (h *HybridService) updateUserPositionRedis(ctx context.Context, customerID, campaignID, userID uuid.UUID) error {
	// Get user data from PostgreSQL (source of truth)
	user, err := h.store.GetWaitlistUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Get campaign configuration
	campaign, err := h.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("failed to get campaign: %w", err)
	}

	// Calculate score using same formula as formula-based approach
	positionsPerReferral := 1
	if campaign.ReferralConfig != nil {
		if val, ok := campaign.ReferralConfig["positions_per_referral"].(float64); ok {
			positionsPerReferral = int(val)
			if positionsPerReferral > 100 {
				positionsPerReferral = 100
			}
		}
	}

	// Determine referral count based on verification requirement
	referralCount := user.ReferralCount
	if campaign.ReferralConfig != nil {
		if verifiedOnly, ok := campaign.ReferralConfig["verified_only"].(bool); ok && verifiedOnly {
			referralCount = user.VerifiedReferralCount
		}
	}

	// Calculate score (lower is better)
	score := float64(user.OriginalPosition - (referralCount * positionsPerReferral))
	if score < 1 {
		score = 1
	}

	// Update in Redis
	err = h.redisService.UpdateScore(ctx, customerID, campaignID, userID, score)
	if err != nil {
		return fmt.Errorf("failed to update Redis score: %w", err)
	}

	// Also update PostgreSQL to keep it in sync (source of truth)
	newPosition := int(score)
	if newPosition != user.Position {
		err = h.store.UpdateWaitlistUserPosition(ctx, userID, newPosition)
		if err != nil {
			h.logger.Error(ctx, "failed to update PostgreSQL position", err)
			// Don't fail the request if PostgreSQL update fails
			// Redis is updated, which is what matters for performance
		}
	}

	return nil
}

// GetUserRank retrieves a user's rank using the appropriate strategy
func (h *HybridService) GetUserRank(
	ctx context.Context,
	customer store.Customer,
	campaignID, userID uuid.UUID,
	strategy Strategy,
) (int, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customer.ID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
	)

	selectedStrategy, err := h.selectStrategy(ctx, customer, campaignID, strategy)
	if err != nil {
		return 0, fmt.Errorf("failed to select strategy: %w", err)
	}

	switch selectedStrategy {
	case StrategyRedis:
		rank, err := h.redisService.GetRank(ctx, customer.ID, campaignID, userID)
		if err != nil {
			return 0, err
		}
		return int(rank), nil

	case StrategyFormula:
		// Get position from PostgreSQL
		user, err := h.store.GetWaitlistUserByID(ctx, userID)
		if err != nil {
			return 0, fmt.Errorf("failed to get user: %w", err)
		}
		return user.Position, nil

	default:
		return 0, fmt.Errorf("unknown strategy: %s", selectedStrategy)
	}
}

// GetTopUsers retrieves top N users using the appropriate strategy
func (h *HybridService) GetTopUsers(
	ctx context.Context,
	customer store.Customer,
	campaignID uuid.UUID,
	limit int,
	strategy Strategy,
) ([]LeaderboardEntry, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customer.ID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "limit", Value: limit},
	)

	selectedStrategy, err := h.selectStrategy(ctx, customer, campaignID, strategy)
	if err != nil {
		return nil, fmt.Errorf("failed to select strategy: %w", err)
	}

	switch selectedStrategy {
	case StrategyRedis:
		return h.redisService.GetTopN(ctx, customer.ID, campaignID, limit)

	case StrategyFormula:
		// Get top users from PostgreSQL
		users, err := h.store.GetTopWaitlistUsers(ctx, campaignID, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to get top users: %w", err)
		}

		entries := make([]LeaderboardEntry, len(users))
		for i, user := range users {
			entries[i] = LeaderboardEntry{
				UserID: user.ID.String(),
				Score:  float64(user.Position),
				Rank:   user.Position,
			}
		}
		return entries, nil

	default:
		return nil, fmt.Errorf("unknown strategy: %s", selectedStrategy)
	}
}

// GetUsersAround retrieves users around a specific user
func (h *HybridService) GetUsersAround(
	ctx context.Context,
	customer store.Customer,
	campaignID, userID uuid.UUID,
	radius int,
	strategy Strategy,
) ([]LeaderboardEntry, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customer.ID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
		observability.Field{Key: "radius", Value: radius},
	)

	selectedStrategy, err := h.selectStrategy(ctx, customer, campaignID, strategy)
	if err != nil {
		return nil, fmt.Errorf("failed to select strategy: %w", err)
	}

	switch selectedStrategy {
	case StrategyRedis:
		return h.redisService.GetUsersAround(ctx, customer.ID, campaignID, userID, radius)

	case StrategyFormula:
		// Get user's position
		user, err := h.store.GetWaitlistUserByID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user: %w", err)
		}

		// Calculate range
		start := user.Position - radius
		if start < 1 {
			start = 1
		}
		end := user.Position + radius

		// Get users in range
		users, err := h.store.GetWaitlistUsersByPositionRange(ctx, campaignID, start, end)
		if err != nil {
			return nil, fmt.Errorf("failed to get users in range: %w", err)
		}

		entries := make([]LeaderboardEntry, len(users))
		for i, u := range users {
			entries[i] = LeaderboardEntry{
				UserID: u.ID.String(),
				Score:  float64(u.Position),
				Rank:   u.Position,
			}
		}
		return entries, nil

	default:
		return nil, fmt.Errorf("unknown strategy: %s", selectedStrategy)
	}
}

// SyncToRedis syncs a campaign's leaderboard from PostgreSQL to Redis
// Used for migrating existing campaigns to Redis or recovering from Redis data loss
func (h *HybridService) SyncToRedis(ctx context.Context, customer store.Customer, campaignID uuid.UUID) error {
	if !customer.RedisEnabled {
		return fmt.Errorf("customer doesn't have Redis enabled")
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customer.ID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "operation", Value: "sync_to_redis"},
	)

	h.logger.Info(ctx, "starting sync from PostgreSQL to Redis")

	return h.redisService.SyncFromDatabase(ctx, customer.ID, campaignID)
}

// GetUserCount returns the total number of users in the leaderboard
func (h *HybridService) GetUserCount(
	ctx context.Context,
	customer store.Customer,
	campaignID uuid.UUID,
	strategy Strategy,
) (int, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customer.ID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	selectedStrategy, err := h.selectStrategy(ctx, customer, campaignID, strategy)
	if err != nil {
		return 0, fmt.Errorf("failed to select strategy: %w", err)
	}

	switch selectedStrategy {
	case StrategyRedis:
		count, err := h.redisService.GetUserCount(ctx, customer.ID, campaignID)
		if err != nil {
			return 0, err
		}
		return int(count), nil

	case StrategyFormula:
		count, err := h.store.GetWaitlistUserCount(ctx, campaignID)
		if err != nil {
			return 0, fmt.Errorf("failed to get user count: %w", err)
		}
		return count, nil

	default:
		return 0, fmt.Errorf("unknown strategy: %s", selectedStrategy)
	}
}
