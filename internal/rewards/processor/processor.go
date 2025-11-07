package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrRewardNotFound       = errors.New("reward not found")
	ErrUserRewardNotFound   = errors.New("user reward not found")
	ErrInvalidRewardType    = errors.New("invalid reward type")
	ErrInvalidTriggerType   = errors.New("invalid trigger type")
	ErrInvalidDeliveryMethod = errors.New("invalid delivery method")
	ErrInvalidRewardStatus  = errors.New("invalid reward status")
	ErrUnauthorized         = errors.New("unauthorized access to reward")
	ErrRewardLimitReached   = errors.New("reward limit reached")
	ErrUserLimitReached     = errors.New("user has already claimed maximum rewards")
)

type RewardProcessor struct {
	store  store.Store
	logger *observability.Logger
}

func New(store store.Store, logger *observability.Logger) RewardProcessor {
	return RewardProcessor{
		store:  store,
		logger: logger,
	}
}

// CreateRewardRequest represents a request to create a reward
type CreateRewardRequest struct {
	Name           string
	Description    *string
	Type           string
	Config         store.JSONB
	TriggerType    string
	TriggerConfig  store.JSONB
	DeliveryMethod string
	DeliveryConfig store.JSONB
	TotalAvailable *int
	UserLimit      int
	StartsAt       *time.Time
	ExpiresAt      *time.Time
}

// CreateReward creates a new reward for a campaign
func (p *RewardProcessor) CreateReward(ctx context.Context, campaignID uuid.UUID, req CreateRewardRequest) (store.Reward, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "reward_name", Value: req.Name},
	)

	// Validate reward type
	if !isValidRewardType(req.Type) {
		return store.Reward{}, ErrInvalidRewardType
	}

	// Validate trigger type
	if !isValidTriggerType(req.TriggerType) {
		return store.Reward{}, ErrInvalidTriggerType
	}

	// Validate delivery method
	if !isValidDeliveryMethod(req.DeliveryMethod) {
		return store.Reward{}, ErrInvalidDeliveryMethod
	}

	// Set defaults for JSONB fields if not provided
	if req.Config == nil {
		req.Config = store.JSONB{}
	}
	if req.TriggerConfig == nil {
		req.TriggerConfig = store.JSONB{}
	}
	if req.DeliveryConfig == nil {
		req.DeliveryConfig = store.JSONB{}
	}

	// Default user limit to 1 if not set
	if req.UserLimit == 0 {
		req.UserLimit = 1
	}

	params := store.CreateRewardParams{
		CampaignID:     campaignID,
		Name:           req.Name,
		Description:    req.Description,
		Type:           req.Type,
		Config:         req.Config,
		TriggerType:    req.TriggerType,
		TriggerConfig:  req.TriggerConfig,
		DeliveryMethod: req.DeliveryMethod,
		DeliveryConfig: req.DeliveryConfig,
		TotalAvailable: req.TotalAvailable,
		UserLimit:      req.UserLimit,
		StartsAt:       req.StartsAt,
		ExpiresAt:      req.ExpiresAt,
	}

	reward, err := p.store.CreateReward(ctx, params)
	if err != nil {
		p.logger.Error(ctx, "failed to create reward", err)
		return store.Reward{}, err
	}

	p.logger.Info(ctx, "reward created successfully")
	return reward, nil
}

// GetReward retrieves a reward by ID
func (p *RewardProcessor) GetReward(ctx context.Context, campaignID, rewardID uuid.UUID) (store.Reward, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "reward_id", Value: rewardID.String()},
	)

	reward, err := p.store.GetRewardByID(ctx, rewardID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Reward{}, ErrRewardNotFound
		}
		p.logger.Error(ctx, "failed to get reward", err)
		return store.Reward{}, err
	}

	// Verify reward belongs to campaign
	if reward.CampaignID != campaignID {
		return store.Reward{}, ErrUnauthorized
	}

	return reward, nil
}

// ListRewards retrieves all rewards for a campaign
func (p *RewardProcessor) ListRewards(ctx context.Context, campaignID uuid.UUID) ([]store.Reward, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	rewards, err := p.store.GetRewardsByCampaign(ctx, campaignID)
	if err != nil {
		p.logger.Error(ctx, "failed to list rewards", err)
		return nil, err
	}

	return rewards, nil
}

// UpdateRewardRequest represents a request to update a reward
type UpdateRewardRequest struct {
	Name           *string
	Description    *string
	Config         store.JSONB
	TriggerConfig  store.JSONB
	DeliveryConfig store.JSONB
	Status         *string
}

// UpdateReward updates a reward
func (p *RewardProcessor) UpdateReward(ctx context.Context, campaignID, rewardID uuid.UUID, req UpdateRewardRequest) (store.Reward, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "reward_id", Value: rewardID.String()},
	)

	// First verify the reward belongs to the campaign
	existingReward, err := p.GetReward(ctx, campaignID, rewardID)
	if err != nil {
		return store.Reward{}, err
	}

	// Validate status if provided
	if req.Status != nil && !isValidRewardStatus(*req.Status) {
		return store.Reward{}, ErrInvalidRewardStatus
	}

	// Ensure we're updating the correct reward
	if existingReward.CampaignID != campaignID {
		return store.Reward{}, ErrUnauthorized
	}

	params := store.UpdateRewardParams{
		Name:           req.Name,
		Description:    req.Description,
		Config:         req.Config,
		TriggerConfig:  req.TriggerConfig,
		DeliveryConfig: req.DeliveryConfig,
		Status:         req.Status,
	}

	reward, err := p.store.UpdateReward(ctx, rewardID, params)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Reward{}, ErrRewardNotFound
		}
		p.logger.Error(ctx, "failed to update reward", err)
		return store.Reward{}, err
	}

	p.logger.Info(ctx, "reward updated successfully")
	return reward, nil
}

// DeleteReward soft deletes a reward
func (p *RewardProcessor) DeleteReward(ctx context.Context, campaignID, rewardID uuid.UUID) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "reward_id", Value: rewardID.String()},
	)

	// First verify the reward belongs to the campaign
	_, err := p.GetReward(ctx, campaignID, rewardID)
	if err != nil {
		return err
	}

	err = p.store.DeleteReward(ctx, rewardID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrRewardNotFound
		}
		p.logger.Error(ctx, "failed to delete reward", err)
		return err
	}

	p.logger.Info(ctx, "reward deleted successfully")
	return nil
}

// GrantRewardRequest represents a request to grant a reward to a user
type GrantRewardRequest struct {
	RewardID  uuid.UUID
	Reason    *string
	ExpiresAt *time.Time
}

// GrantReward manually grants a reward to a user
func (p *RewardProcessor) GrantReward(ctx context.Context, campaignID, userID uuid.UUID, req GrantRewardRequest) (store.UserReward, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
		observability.Field{Key: "reward_id", Value: req.RewardID.String()},
	)

	// Verify reward exists and belongs to campaign
	reward, err := p.GetReward(ctx, campaignID, req.RewardID)
	if err != nil {
		return store.UserReward{}, err
	}

	// Check if reward is active
	if reward.Status != "active" {
		return store.UserReward{}, ErrInvalidRewardStatus
	}

	// Check if reward has available quantity
	if reward.TotalAvailable != nil && reward.TotalClaimed >= *reward.TotalAvailable {
		return store.UserReward{}, ErrRewardLimitReached
	}

	// Check user's reward limit
	existingRewards, err := p.store.GetUserRewardsByUser(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get user rewards", err)
		return store.UserReward{}, err
	}

	// Count how many times user has claimed this specific reward
	claimCount := 0
	for _, ur := range existingRewards {
		if ur.RewardID == req.RewardID {
			claimCount++
		}
	}

	if claimCount >= reward.UserLimit {
		return store.UserReward{}, ErrUserLimitReached
	}

	// Create reward data snapshot
	rewardData := store.JSONB{
		"name":        reward.Name,
		"description": reward.Description,
		"type":        reward.Type,
		"config":      reward.Config,
	}

	if req.Reason != nil {
		rewardData["grant_reason"] = *req.Reason
	}

	// Determine expiry
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		expiresAt = req.ExpiresAt
	} else if reward.ExpiresAt != nil {
		expiresAt = reward.ExpiresAt
	}

	params := store.CreateUserRewardParams{
		UserID:     userID,
		RewardID:   req.RewardID,
		CampaignID: campaignID,
		RewardData: rewardData,
		ExpiresAt:  expiresAt,
	}

	userReward, err := p.store.CreateUserReward(ctx, params)
	if err != nil {
		p.logger.Error(ctx, "failed to create user reward", err)
		return store.UserReward{}, err
	}

	// Increment the claimed count
	err = p.store.IncrementRewardClaimed(ctx, req.RewardID)
	if err != nil {
		p.logger.Error(ctx, "failed to increment reward claimed", err)
		// Don't fail the whole operation, just log the error
	}

	p.logger.Info(ctx, "reward granted to user successfully")
	return userReward, nil
}

// GetUserRewards retrieves all rewards earned by a user
func (p *RewardProcessor) GetUserRewards(ctx context.Context, campaignID, userID uuid.UUID) ([]store.UserReward, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
	)

	rewards, err := p.store.GetUserRewardsByUser(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get user rewards", err)
		return nil, err
	}

	// Filter to only rewards for this campaign
	var campaignRewards []store.UserReward
	for _, reward := range rewards {
		if reward.CampaignID == campaignID {
			campaignRewards = append(campaignRewards, reward)
		}
	}

	return campaignRewards, nil
}

// Helper functions

func isValidRewardType(rewardType string) bool {
	validTypes := map[string]bool{
		"early_access":    true,
		"discount":        true,
		"premium_feature": true,
		"merchandise":     true,
		"custom":          true,
	}
	return validTypes[rewardType]
}

func isValidTriggerType(triggerType string) bool {
	validTypes := map[string]bool{
		"referral_count": true,
		"position":       true,
		"milestone":      true,
		"manual":         true,
	}
	return validTypes[triggerType]
}

func isValidDeliveryMethod(method string) bool {
	validMethods := map[string]bool{
		"email":   true,
		"webhook": true,
		"manual":  true,
	}
	return validMethods[method]
}

func isValidRewardStatus(status string) bool {
	validStatuses := map[string]bool{
		"active":  true,
		"paused":  true,
		"expired": true,
	}
	return validStatuses[status]
}
