package ratelimit

import (
	"base-server/internal/clients/redis"
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RateLimitResult represents the result of a rate limit check
type RateLimitResult struct {
	Allowed         bool      `json:"allowed"`
	Limit           int       `json:"limit"`
	Remaining       int       `json:"remaining"`
	ResetAt         time.Time `json:"reset_at"`
	RetryAfterMs    int       `json:"retry_after_ms,omitempty"`
}

// Service handles rate limiting for account API requests
type Service struct {
	redis  *redis.Client
	store  store.Store
	logger *observability.Logger
}

// NewService creates a new rate limiting service
func NewService(redis *redis.Client, store store.Store, logger *observability.Logger) *Service {
	return &Service{
		redis:  redis,
		store:  store,
		logger: logger,
	}
}

// CheckRateLimit checks if an account is within their rate limit
// Uses Redis for fast distributed rate limiting, falls back to PostgreSQL
func (s *Service) CheckRateLimit(ctx context.Context, accountID uuid.UUID, rateLimit int) (RateLimitResult, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "rate_limit", Value: rateLimit},
	)

	// Try Redis-based rate limiting first (preferred for performance)
	if s.redis != nil && s.redis.IsEnabled() {
		result, err := s.checkRateLimitRedis(ctx, accountID, rateLimit)
		if err != nil {
			// Redis failed, fall back to PostgreSQL
			s.logger.Warn(ctx, "Redis rate limit check failed, falling back to PostgreSQL", err)
			return s.checkRateLimitPostgres(ctx, accountID, rateLimit)
		}
		return result, nil
	}

	// Fall back to PostgreSQL if Redis is not available
	return s.checkRateLimitPostgres(ctx, accountID, rateLimit)
}

// checkRateLimitRedis implements Redis-based rate limiting using sliding window
func (s *Service) checkRateLimitRedis(ctx context.Context, accountID uuid.UUID, rateLimit int) (RateLimitResult, error) {
	// Use sliding window rate limiting with Redis sorted sets
	// Key: rl:{account_id}
	// Members: request timestamps
	// Score: timestamp in milliseconds

	key := fmt.Sprintf("rl:%s", accountID.String())
	now := time.Now()
	nowMs := now.UnixMilli()
	windowStart := now.Add(-1 * time.Minute)
	windowStartMs := windowStart.UnixMilli()

	// Remove old entries outside the 1-minute window
	err := s.redis.GetClient().ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStartMs)).Err()
	if err != nil {
		return RateLimitResult{}, fmt.Errorf("failed to remove old entries: %w", err)
	}

	// Count current requests in window
	count, err := s.redis.GetClient().ZCard(ctx, key).Result()
	if err != nil {
		return RateLimitResult{}, fmt.Errorf("failed to count requests: %w", err)
	}

	// Check if limit exceeded
	if int(count) >= rateLimit {
		// Get the oldest request timestamp to calculate retry after
		oldest, err := s.redis.GetClient().ZRange(ctx, key, 0, 0).Result()
		if err != nil || len(oldest) == 0 {
			return RateLimitResult{
				Allowed:      false,
				Limit:        rateLimit,
				Remaining:    0,
				ResetAt:      now.Add(1 * time.Minute),
				RetryAfterMs: 60000,
			}, nil
		}

		// Calculate retry after based on oldest request
		var oldestTs int64
		fmt.Sscanf(oldest[0], "%d", &oldestTs)
		retryAfter := time.UnixMilli(oldestTs).Add(1 * time.Minute).Sub(now)
		if retryAfter < 0 {
			retryAfter = 0
		}

		return RateLimitResult{
			Allowed:      false,
			Limit:        rateLimit,
			Remaining:    0,
			ResetAt:      time.UnixMilli(oldestTs).Add(1 * time.Minute),
			RetryAfterMs: int(retryAfter.Milliseconds()),
		}, nil
	}

	// Add current request to the window
	err = s.redis.GetClient().ZAdd(ctx, key,
		redis.Z{
			Score:  float64(nowMs),
			Member: fmt.Sprintf("%d", nowMs),
		},
	).Err()
	if err != nil {
		return RateLimitResult{}, fmt.Errorf("failed to add request: %w", err)
	}

	// Set expiration on the key (2 minutes to be safe)
	err = s.redis.Expire(ctx, key, 2*time.Minute)
	if err != nil {
		s.logger.Warn(ctx, "failed to set expiration on rate limit key", err)
	}

	return RateLimitResult{
		Allowed:   true,
		Limit:     rateLimit,
		Remaining: rateLimit - int(count) - 1,
		ResetAt:   now.Add(1 * time.Minute),
	}, nil
}

// checkRateLimitPostgres implements PostgreSQL-based rate limiting (fallback)
func (s *Service) checkRateLimitPostgres(ctx context.Context, accountID uuid.UUID, rateLimit int) (RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Add(-1 * time.Minute).Truncate(time.Minute)
	windowEnd := windowStart.Add(1 * time.Minute)

	// Try to get existing rate limit record
	var rateLimitRecord store.RateLimit
	query := `
		SELECT id, account_id, window_start, window_end, requests_count, requests_limit,
		       is_throttled, created_at, updated_at
		FROM rate_limits
		WHERE account_id = $1 AND window_start = $2
	`

	err := s.store.GetDB().GetContext(ctx, &rateLimitRecord, query, accountID, windowStart)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return RateLimitResult{}, fmt.Errorf("failed to get rate limit record: %w", err)
	}

	// If record doesn't exist, create it
	if err != nil {
		createQuery := `
			INSERT INTO rate_limits (account_id, window_start, window_end, requests_count, requests_limit, is_throttled)
			VALUES ($1, $2, $3, 1, $4, false)
			RETURNING id, account_id, window_start, window_end, requests_count, requests_limit,
			          is_throttled, created_at, updated_at
		`

		err = s.store.GetDB().GetContext(ctx, &rateLimitRecord, createQuery,
			accountID, windowStart, windowEnd, rateLimit)
		if err != nil {
			return RateLimitResult{}, fmt.Errorf("failed to create rate limit record: %w", err)
		}

		return RateLimitResult{
			Allowed:   true,
			Limit:     rateLimit,
			Remaining: rateLimit - 1,
			ResetAt:   windowEnd,
		}, nil
	}

	// Check if limit exceeded
	if rateLimitRecord.RequestsCount >= rateLimit {
		retryAfter := windowEnd.Sub(now)
		if retryAfter < 0 {
			retryAfter = 0
		}

		return RateLimitResult{
			Allowed:      false,
			Limit:        rateLimit,
			Remaining:    0,
			ResetAt:      windowEnd,
			RetryAfterMs: int(retryAfter.Milliseconds()),
		}, nil
	}

	// Increment request count
	updateQuery := `
		UPDATE rate_limits
		SET requests_count = requests_count + 1,
		    is_throttled = (requests_count + 1 >= requests_limit),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
		RETURNING requests_count
	`

	var newCount int
	err = s.store.GetDB().GetContext(ctx, &newCount, updateQuery, rateLimitRecord.ID)
	if err != nil {
		return RateLimitResult{}, fmt.Errorf("failed to increment rate limit: %w", err)
	}

	return RateLimitResult{
		Allowed:   true,
		Limit:     rateLimit,
		Remaining: rateLimit - newCount,
		ResetAt:   windowEnd,
	}, nil
}

// GetRateLimitStatus retrieves the current rate limit status for an account
func (s *Service) GetRateLimitStatus(ctx context.Context, accountID uuid.UUID, rateLimit int) (RateLimitResult, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
	)

	// Try Redis first
	if s.redis != nil && s.redis.IsEnabled() {
		key := fmt.Sprintf("rl:%s", accountID.String())
		now := time.Now()
		windowStart := now.Add(-1 * time.Minute)
		windowStartMs := windowStart.UnixMilli()

		// Count current requests in window
		count, err := s.redis.GetClient().ZCount(ctx, key,
			fmt.Sprintf("%d", windowStartMs),
			fmt.Sprintf("%d", now.UnixMilli())).Result()
		if err != nil {
			s.logger.Warn(ctx, "Redis status check failed, falling back to PostgreSQL", err)
			return s.getRateLimitStatusPostgres(ctx, accountID, rateLimit)
		}

		return RateLimitResult{
			Allowed:   int(count) < rateLimit,
			Limit:     rateLimit,
			Remaining: max(0, rateLimit-int(count)),
			ResetAt:   now.Add(1 * time.Minute),
		}, nil
	}

	// Fall back to PostgreSQL
	return s.getRateLimitStatusPostgres(ctx, accountID, rateLimit)
}

// getRateLimitStatusPostgres retrieves rate limit status from PostgreSQL
func (s *Service) getRateLimitStatusPostgres(ctx context.Context, accountID uuid.UUID, rateLimit int) (RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Add(-1 * time.Minute).Truncate(time.Minute)
	windowEnd := windowStart.Add(1 * time.Minute)

	var rateLimitRecord store.RateLimit
	query := `
		SELECT id, account_id, window_start, window_end, requests_count, requests_limit,
		       is_throttled, created_at, updated_at
		FROM rate_limits
		WHERE account_id = $1 AND window_start = $2
	`

	err := s.store.GetDB().GetContext(ctx, &rateLimitRecord, query, accountID, windowStart)
	if err != nil {
		// No record means no requests in this window
		return RateLimitResult{
			Allowed:   true,
			Limit:     rateLimit,
			Remaining: rateLimit,
			ResetAt:   windowEnd,
		}, nil
	}

	return RateLimitResult{
		Allowed:   rateLimitRecord.RequestsCount < rateLimit,
		Limit:     rateLimit,
		Remaining: max(0, rateLimit-rateLimitRecord.RequestsCount),
		ResetAt:   windowEnd,
	}, nil
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// CleanupExpiredRecords removes expired rate limit records from PostgreSQL
// Should be called periodically (e.g., via cron job)
func (s *Service) CleanupExpiredRecords(ctx context.Context) error {
	// Delete records older than 1 hour
	cutoff := time.Now().Add(-1 * time.Hour)

	query := `
		DELETE FROM rate_limits
		WHERE window_end < $1
	`

	result, err := s.store.GetDB().ExecContext(ctx, query, cutoff)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired rate limits: %w", err)
	}

	rowsDeleted, _ := result.RowsAffected()
	s.logger.Info(ctx, "cleaned up expired rate limit records",
		observability.Field{Key: "rows_deleted", Value: rowsDeleted},
	)

	return nil
}
