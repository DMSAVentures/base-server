package leaderboard

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Processor handles leaderboard business logic
type Processor struct {
	hybridService *HybridService
	store         store.Store
	logger        *observability.Logger
}

// NewProcessor creates a new leaderboard processor
func NewProcessor(hybridService *HybridService, store store.Store, logger *observability.Logger) *Processor {
	return &Processor{
		hybridService: hybridService,
		store:         store,
		logger:        logger,
	}
}

// GetUserRankRequest represents a request to get a user's rank
type GetUserRankRequest struct {
	CampaignID uuid.UUID `json:"campaign_id" binding:"required"`
	UserID     uuid.UUID `json:"user_id" binding:"required"`
	Strategy   string    `json:"strategy,omitempty"` // "formula", "redis", or "auto" (default)
}

// GetUserRankResponse represents the response for getting a user's rank
type GetUserRankResponse struct {
	UserID     string    `json:"user_id"`
	CampaignID string    `json:"campaign_id"`
	Rank       int       `json:"rank"`
	Score      float64   `json:"score"`
	Strategy   string    `json:"strategy"`
	Timestamp  time.Time `json:"timestamp"`
}

// GetUserRank retrieves a user's rank in the leaderboard
func (p *Processor) GetUserRank(ctx context.Context, customer store.Customer, req GetUserRankRequest) (GetUserRankResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customer.ID.String()},
		observability.Field{Key: "campaign_id", Value: req.CampaignID.String()},
		observability.Field{Key: "user_id", Value: req.UserID.String()},
	)

	// Parse strategy
	strategy := StrategyAuto
	if req.Strategy != "" {
		strategy = Strategy(req.Strategy)
	}

	// Get rank
	rank, err := p.hybridService.GetUserRank(ctx, customer, req.CampaignID, req.UserID, strategy)
	if err != nil {
		p.logger.Error(ctx, "failed to get user rank", err)
		return GetUserRankResponse{}, fmt.Errorf("failed to get user rank: %w", err)
	}

	// Get user for score
	user, err := p.store.GetWaitlistUserByID(ctx, req.UserID)
	if err != nil {
		p.logger.Error(ctx, "failed to get user", err)
		return GetUserRankResponse{}, fmt.Errorf("failed to get user: %w", err)
	}

	return GetUserRankResponse{
		UserID:     req.UserID.String(),
		CampaignID: req.CampaignID.String(),
		Rank:       rank,
		Score:      float64(user.Position),
		Strategy:   string(strategy),
		Timestamp:  time.Now(),
	}, nil
}

// GetTopUsersRequest represents a request to get top users
type GetTopUsersRequest struct {
	CampaignID uuid.UUID `json:"campaign_id" binding:"required"`
	Limit      int       `json:"limit" binding:"min=1,max=1000"`
	Strategy   string    `json:"strategy,omitempty"`
}

// LeaderboardUser represents a user in the leaderboard
type LeaderboardUser struct {
	UserID string  `json:"user_id"`
	Rank   int     `json:"rank"`
	Score  float64 `json:"score"`
	Email  string  `json:"email,omitempty"`
}

// GetTopUsersResponse represents the response for getting top users
type GetTopUsersResponse struct {
	CampaignID string            `json:"campaign_id"`
	Users      []LeaderboardUser `json:"users"`
	Total      int               `json:"total"`
	Strategy   string            `json:"strategy"`
	Timestamp  time.Time         `json:"timestamp"`
}

// GetTopUsers retrieves the top N users in the leaderboard
func (p *Processor) GetTopUsers(ctx context.Context, customer store.Customer, req GetTopUsersRequest) (GetTopUsersResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customer.ID.String()},
		observability.Field{Key: "campaign_id", Value: req.CampaignID.String()},
		observability.Field{Key: "limit", Value: req.Limit},
	)

	// Default limit
	if req.Limit == 0 {
		req.Limit = 10
	}

	// Parse strategy
	strategy := StrategyAuto
	if req.Strategy != "" {
		strategy = Strategy(req.Strategy)
	}

	// Get top users
	entries, err := p.hybridService.GetTopUsers(ctx, customer, req.CampaignID, req.Limit, strategy)
	if err != nil {
		p.logger.Error(ctx, "failed to get top users", err)
		return GetTopUsersResponse{}, fmt.Errorf("failed to get top users: %w", err)
	}

	// Get total count
	total, err := p.hybridService.GetUserCount(ctx, customer, req.CampaignID, strategy)
	if err != nil {
		p.logger.Error(ctx, "failed to get user count", err)
		// Don't fail the request, just log the error
		total = 0
	}

	// Convert to response format
	users := make([]LeaderboardUser, len(entries))
	for i, entry := range entries {
		users[i] = LeaderboardUser{
			UserID: entry.UserID,
			Rank:   entry.Rank,
			Score:  entry.Score,
		}
	}

	return GetTopUsersResponse{
		CampaignID: req.CampaignID.String(),
		Users:      users,
		Total:      total,
		Strategy:   string(strategy),
		Timestamp:  time.Now(),
	}, nil
}

// GetUsersAroundRequest represents a request to get users around a specific user
type GetUsersAroundRequest struct {
	CampaignID uuid.UUID `json:"campaign_id" binding:"required"`
	UserID     uuid.UUID `json:"user_id" binding:"required"`
	Radius     int       `json:"radius" binding:"min=1,max=100"`
	Strategy   string    `json:"strategy,omitempty"`
}

// GetUsersAroundResponse represents the response for getting users around
type GetUsersAroundResponse struct {
	CampaignID string            `json:"campaign_id"`
	UserID     string            `json:"user_id"`
	Users      []LeaderboardUser `json:"users"`
	Strategy   string            `json:"strategy"`
	Timestamp  time.Time         `json:"timestamp"`
}

// GetUsersAround retrieves users around a specific user in the leaderboard
func (p *Processor) GetUsersAround(ctx context.Context, customer store.Customer, req GetUsersAroundRequest) (GetUsersAroundResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customer.ID.String()},
		observability.Field{Key: "campaign_id", Value: req.CampaignID.String()},
		observability.Field{Key: "user_id", Value: req.UserID.String()},
		observability.Field{Key: "radius", Value: req.Radius},
	)

	// Default radius
	if req.Radius == 0 {
		req.Radius = 5
	}

	// Parse strategy
	strategy := StrategyAuto
	if req.Strategy != "" {
		strategy = Strategy(req.Strategy)
	}

	// Get users around
	entries, err := p.hybridService.GetUsersAround(ctx, customer, req.CampaignID, req.UserID, req.Radius, strategy)
	if err != nil {
		p.logger.Error(ctx, "failed to get users around", err)
		return GetUsersAroundResponse{}, fmt.Errorf("failed to get users around: %w", err)
	}

	// Convert to response format
	users := make([]LeaderboardUser, len(entries))
	for i, entry := range entries {
		users[i] = LeaderboardUser{
			UserID: entry.UserID,
			Rank:   entry.Rank,
			Score:  entry.Score,
		}
	}

	return GetUsersAroundResponse{
		CampaignID: req.CampaignID.String(),
		UserID:     req.UserID.String(),
		Users:      users,
		Strategy:   string(strategy),
		Timestamp:  time.Now(),
	}, nil
}

// UpdateUserScoreRequest represents a request to update a user's score
type UpdateUserScoreRequest struct {
	CampaignID uuid.UUID `json:"campaign_id" binding:"required"`
	UserID     uuid.UUID `json:"user_id" binding:"required"`
	Strategy   string    `json:"strategy,omitempty"`
}

// UpdateUserScoreResponse represents the response for updating a user's score
type UpdateUserScoreResponse struct {
	UserID     string    `json:"user_id"`
	CampaignID string    `json:"campaign_id"`
	Success    bool      `json:"success"`
	Strategy   string    `json:"strategy"`
	Timestamp  time.Time `json:"timestamp"`
}

// UpdateUserScore updates a user's position in the leaderboard
func (p *Processor) UpdateUserScore(ctx context.Context, customer store.Customer, req UpdateUserScoreRequest) (UpdateUserScoreResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customer.ID.String()},
		observability.Field{Key: "campaign_id", Value: req.CampaignID.String()},
		observability.Field{Key: "user_id", Value: req.UserID.String()},
	)

	// Parse strategy
	strategy := StrategyAuto
	if req.Strategy != "" {
		strategy = Strategy(req.Strategy)
	}

	// Update position
	err := p.hybridService.UpdateUserPosition(ctx, customer, req.CampaignID, req.UserID, strategy)
	if err != nil {
		p.logger.Error(ctx, "failed to update user score", err)
		return UpdateUserScoreResponse{}, fmt.Errorf("failed to update user score: %w", err)
	}

	return UpdateUserScoreResponse{
		UserID:     req.UserID.String(),
		CampaignID: req.CampaignID.String(),
		Success:    true,
		Strategy:   string(strategy),
		Timestamp:  time.Now(),
	}, nil
}

// SyncToRedisRequest represents a request to sync a leaderboard to Redis
type SyncToRedisRequest struct {
	CampaignID uuid.UUID `json:"campaign_id" binding:"required"`
}

// SyncToRedisResponse represents the response for syncing to Redis
type SyncToRedisResponse struct {
	CampaignID string    `json:"campaign_id"`
	Success    bool      `json:"success"`
	Timestamp  time.Time `json:"timestamp"`
}

// SyncToRedis syncs a leaderboard from PostgreSQL to Redis
func (p *Processor) SyncToRedis(ctx context.Context, customer store.Customer, req SyncToRedisRequest) (SyncToRedisResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "customer_id", Value: customer.ID.String()},
		observability.Field{Key: "campaign_id", Value: req.CampaignID.String()},
	)

	err := p.hybridService.SyncToRedis(ctx, customer, req.CampaignID)
	if err != nil {
		p.logger.Error(ctx, "failed to sync to Redis", err)
		return SyncToRedisResponse{}, fmt.Errorf("failed to sync to Redis: %w", err)
	}

	return SyncToRedisResponse{
		CampaignID: req.CampaignID.String(),
		Success:    true,
		Timestamp:  time.Now(),
	}, nil
}
