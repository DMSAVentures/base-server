package processor

import (
	"base-server/internal/store"
	"context"

	"github.com/google/uuid"
)

// RewardStore defines the database operations required by RewardProcessor
type RewardStore interface {
	CreateReward(ctx context.Context, params store.CreateRewardParams) (store.Reward, error)
	GetRewardByID(ctx context.Context, rewardID uuid.UUID) (store.Reward, error)
	GetRewardsByCampaign(ctx context.Context, campaignID uuid.UUID) ([]store.Reward, error)
	UpdateReward(ctx context.Context, rewardID uuid.UUID, params store.UpdateRewardParams) (store.Reward, error)
	DeleteReward(ctx context.Context, rewardID uuid.UUID) error
	GetUserRewardsByUser(ctx context.Context, userID uuid.UUID) ([]store.UserReward, error)
	CreateUserReward(ctx context.Context, params store.CreateUserRewardParams) (store.UserReward, error)
	IncrementRewardClaimed(ctx context.Context, rewardID uuid.UUID) error
}
