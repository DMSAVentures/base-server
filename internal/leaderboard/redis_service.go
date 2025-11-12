package leaderboard

import (
	"base-server/internal/clients/redis"
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"fmt"

	redisLib "github.com/redis/go-redis/v9"
	"github.com/google/uuid"
)

// RedisLeaderboardService handles high-performance leaderboard operations using Redis ZSET
type RedisLeaderboardService struct {
	redis  *redis.Client
	store  store.Store
	logger *observability.Logger
}

// LeaderboardEntry represents a user's position in the leaderboard
type LeaderboardEntry struct {
	UserID string  `json:"user_id"`
	Score  float64 `json:"score"`
	Rank   int     `json:"rank"`
}

// NewRedisLeaderboardService creates a new Redis-based leaderboard service
func NewRedisLeaderboardService(redis *redis.Client, store store.Store, logger *observability.Logger) *RedisLeaderboardService {
	return &RedisLeaderboardService{
		redis:  redis,
		store:  store,
		logger: logger,
	}
}

// buildKey creates a namespaced Redis key for multi-tenancy
// Format: lb:{customer_id}:{campaign_id}
func (s *RedisLeaderboardService) buildKey(customerID, campaignID uuid.UUID) string {
	return fmt.Sprintf("lb:%s:%s", customerID.String(), campaignID.String())
}

// UpdateScore updates a user's score in the leaderboard
// Score represents position (lower is better): score = original_position - (referrals × multiplier)
func (s *RedisLeaderboardService) UpdateScore(
	ctx context.Context,
	customerID, campaignID, userID uuid.UUID,
	score float64,
) error {
	if !s.redis.IsEnabled() {
		return fmt.Errorf("Redis is not enabled")
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customerID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
		observability.Field{Key: "score", Value: score},
	)

	key := s.buildKey(customerID, campaignID)

	err := s.redis.ZAdd(ctx, key, redisLib.Z{
		Score:  score,
		Member: userID.String(),
	})

	if err != nil {
		s.logger.Error(ctx, "failed to update score in Redis", err)
		return fmt.Errorf("failed to update score: %w", err)
	}

	s.logger.Info(ctx, "successfully updated score in Redis")
	return nil
}

// GetRank returns the user's rank in the leaderboard (1-indexed)
// Lower score = better rank (position 1 is best)
func (s *RedisLeaderboardService) GetRank(
	ctx context.Context,
	customerID, campaignID, userID uuid.UUID,
) (int64, error) {
	if !s.redis.IsEnabled() {
		return 0, fmt.Errorf("Redis is not enabled")
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customerID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
	)

	key := s.buildKey(customerID, campaignID)

	// ZRank returns 0-indexed rank (ascending order by score)
	rank, err := s.redis.ZRank(ctx, key, userID.String())
	if err == redisLib.Nil {
		return 0, fmt.Errorf("user not found in leaderboard")
	}
	if err != nil {
		s.logger.Error(ctx, "failed to get rank from Redis", err)
		return 0, fmt.Errorf("failed to get rank: %w", err)
	}

	// Convert to 1-indexed
	return rank + 1, nil
}

// GetScore returns the user's score in the leaderboard
func (s *RedisLeaderboardService) GetScore(
	ctx context.Context,
	customerID, campaignID, userID uuid.UUID,
) (float64, error) {
	if !s.redis.IsEnabled() {
		return 0, fmt.Errorf("Redis is not enabled")
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customerID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
	)

	key := s.buildKey(customerID, campaignID)

	score, err := s.redis.ZScore(ctx, key, userID.String())
	if err == redisLib.Nil {
		return 0, fmt.Errorf("user not found in leaderboard")
	}
	if err != nil {
		s.logger.Error(ctx, "failed to get score from Redis", err)
		return 0, fmt.Errorf("failed to get score: %w", err)
	}

	return score, nil
}

// GetTopN returns the top N users in the leaderboard
func (s *RedisLeaderboardService) GetTopN(
	ctx context.Context,
	customerID, campaignID uuid.UUID,
	limit int,
) ([]LeaderboardEntry, error) {
	if !s.redis.IsEnabled() {
		return nil, fmt.Errorf("Redis is not enabled")
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customerID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "limit", Value: limit},
	)

	key := s.buildKey(customerID, campaignID)

	// Get top N with scores (ascending order - lower score is better)
	results, err := s.redis.ZRangeWithScores(ctx, key, 0, int64(limit-1))
	if err != nil {
		s.logger.Error(ctx, "failed to get top N from Redis", err)
		return nil, fmt.Errorf("failed to get top users: %w", err)
	}

	entries := make([]LeaderboardEntry, len(results))
	for i, result := range results {
		entries[i] = LeaderboardEntry{
			UserID: result.Member.(string),
			Score:  result.Score,
			Rank:   i + 1,
		}
	}

	s.logger.Info(ctx, "successfully retrieved top N from Redis",
		observability.Field{Key: "count", Value: len(entries)},
	)

	return entries, nil
}

// GetUserCount returns the total number of users in the leaderboard
func (s *RedisLeaderboardService) GetUserCount(
	ctx context.Context,
	customerID, campaignID uuid.UUID,
) (int64, error) {
	if !s.redis.IsEnabled() {
		return 0, fmt.Errorf("Redis is not enabled")
	}

	key := s.buildKey(customerID, campaignID)

	count, err := s.redis.ZCard(ctx, key)
	if err != nil {
		s.logger.Error(ctx, "failed to get user count from Redis", err)
		return 0, fmt.Errorf("failed to get user count: %w", err)
	}

	return count, nil
}

// RemoveUser removes a user from the leaderboard
func (s *RedisLeaderboardService) RemoveUser(
	ctx context.Context,
	customerID, campaignID, userID uuid.UUID,
) error {
	if !s.redis.IsEnabled() {
		return fmt.Errorf("Redis is not enabled")
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customerID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
	)

	key := s.buildKey(customerID, campaignID)

	err := s.redis.ZRem(ctx, key, userID.String())
	if err != nil {
		s.logger.Error(ctx, "failed to remove user from Redis", err)
		return fmt.Errorf("failed to remove user: %w", err)
	}

	s.logger.Info(ctx, "successfully removed user from Redis")
	return nil
}

// GetUsersAround returns users around a specific user in the leaderboard
func (s *RedisLeaderboardService) GetUsersAround(
	ctx context.Context,
	customerID, campaignID, userID uuid.UUID,
	radius int,
) ([]LeaderboardEntry, error) {
	if !s.redis.IsEnabled() {
		return nil, fmt.Errorf("Redis is not enabled")
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customerID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
		observability.Field{Key: "radius", Value: radius},
	)

	// Get user's rank
	rank, err := s.GetRank(ctx, customerID, campaignID, userID)
	if err != nil {
		return nil, err
	}

	// Calculate range
	start := rank - int64(radius) - 1 // Convert to 0-indexed
	if start < 0 {
		start = 0
	}
	stop := rank + int64(radius) - 1

	key := s.buildKey(customerID, campaignID)

	// Get users in range
	results, err := s.redis.ZRangeWithScores(ctx, key, start, stop)
	if err != nil {
		s.logger.Error(ctx, "failed to get users around from Redis", err)
		return nil, fmt.Errorf("failed to get users around: %w", err)
	}

	entries := make([]LeaderboardEntry, len(results))
	for i, result := range results {
		entries[i] = LeaderboardEntry{
			UserID: result.Member.(string),
			Score:  result.Score,
			Rank:   int(start) + i + 1,
		}
	}

	return entries, nil
}

// SyncFromDatabase populates Redis from PostgreSQL (for initial load or recovery)
func (s *RedisLeaderboardService) SyncFromDatabase(
	ctx context.Context,
	customerID, campaignID uuid.UUID,
) error {
	if !s.redis.IsEnabled() {
		return fmt.Errorf("Redis is not enabled")
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customerID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "operation", Value: "sync_from_database"},
	)

	s.logger.Info(ctx, "starting sync from database to Redis")

	// Get all users for this campaign from PostgreSQL
	users, err := s.store.GetAllWaitlistUsersForPositionCalculation(ctx, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to get users from database", err)
		return fmt.Errorf("failed to get users: %w", err)
	}

	if len(users) == 0 {
		s.logger.Info(ctx, "no users to sync")
		return nil
	}

	key := s.buildKey(customerID, campaignID)

	// Build Redis ZADD command with all users
	members := make([]redisLib.Z, len(users))
	for i, user := range users {
		members[i] = redisLib.Z{
			Score:  float64(user.Position),
			Member: user.ID.String(),
		}
	}

	// Batch add all users to Redis
	err = s.redis.ZAdd(ctx, key, members...)
	if err != nil {
		s.logger.Error(ctx, "failed to sync users to Redis", err)
		return fmt.Errorf("failed to sync users: %w", err)
	}

	s.logger.Info(ctx, "successfully synced users to Redis",
		observability.Field{Key: "user_count", Value: len(users)},
	)

	return nil
}
