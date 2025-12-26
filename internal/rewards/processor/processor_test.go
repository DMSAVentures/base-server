package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestCreateReward_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockRewardStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	campaignID := uuid.New()
	rewardID := uuid.New()

	req := CreateRewardRequest{
		Name:           "Early Access",
		Type:           "early_access",
		TriggerType:    "referral_count",
		DeliveryMethod: "email",
	}

	mockStore.EXPECT().CreateReward(gomock.Any(), gomock.Any()).Return(store.Reward{
		ID:         rewardID,
		CampaignID: campaignID,
		Name:       req.Name,
		Type:       req.Type,
	}, nil)

	result, err := processor.CreateReward(ctx, campaignID, req)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.ID != rewardID {
		t.Errorf("expected reward ID %s, got %s", rewardID, result.ID)
	}
}

func TestCreateReward_InvalidRewardType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockRewardStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	campaignID := uuid.New()

	req := CreateRewardRequest{
		Name:           "Invalid Reward",
		Type:           "invalid_type",
		TriggerType:    "referral_count",
		DeliveryMethod: "email",
	}

	_, err := processor.CreateReward(ctx, campaignID, req)

	if !errors.Is(err, ErrInvalidRewardType) {
		t.Errorf("expected ErrInvalidRewardType, got %v", err)
	}
}

func TestCreateReward_InvalidTriggerType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockRewardStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	campaignID := uuid.New()

	req := CreateRewardRequest{
		Name:           "Invalid Trigger",
		Type:           "early_access",
		TriggerType:    "invalid_trigger",
		DeliveryMethod: "email",
	}

	_, err := processor.CreateReward(ctx, campaignID, req)

	if !errors.Is(err, ErrInvalidTriggerType) {
		t.Errorf("expected ErrInvalidTriggerType, got %v", err)
	}
}

func TestGetReward_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockRewardStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	campaignID := uuid.New()
	rewardID := uuid.New()

	mockStore.EXPECT().GetRewardByID(gomock.Any(), rewardID).Return(store.Reward{
		ID:         rewardID,
		CampaignID: campaignID,
		Name:       "Test Reward",
	}, nil)

	result, err := processor.GetReward(ctx, campaignID, rewardID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.ID != rewardID {
		t.Errorf("expected reward ID %s, got %s", rewardID, result.ID)
	}
}

func TestGetReward_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockRewardStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	campaignID := uuid.New()
	rewardID := uuid.New()

	mockStore.EXPECT().GetRewardByID(gomock.Any(), rewardID).Return(store.Reward{}, store.ErrNotFound)

	_, err := processor.GetReward(ctx, campaignID, rewardID)

	if !errors.Is(err, ErrRewardNotFound) {
		t.Errorf("expected ErrRewardNotFound, got %v", err)
	}
}

func TestGetReward_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockRewardStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	campaignID := uuid.New()
	otherCampaignID := uuid.New()
	rewardID := uuid.New()

	mockStore.EXPECT().GetRewardByID(gomock.Any(), rewardID).Return(store.Reward{
		ID:         rewardID,
		CampaignID: otherCampaignID,
	}, nil)

	_, err := processor.GetReward(ctx, campaignID, rewardID)

	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestListRewards_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockRewardStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	campaignID := uuid.New()

	rewards := []store.Reward{
		{ID: uuid.New(), Name: "Reward 1"},
		{ID: uuid.New(), Name: "Reward 2"},
	}

	mockStore.EXPECT().GetRewardsByCampaign(gomock.Any(), campaignID).Return(rewards, nil)

	result, err := processor.ListRewards(ctx, campaignID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 rewards, got %d", len(result))
	}
}

func TestDeleteReward_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockRewardStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	campaignID := uuid.New()
	rewardID := uuid.New()

	mockStore.EXPECT().GetRewardByID(gomock.Any(), rewardID).Return(store.Reward{
		ID:         rewardID,
		CampaignID: campaignID,
	}, nil)
	mockStore.EXPECT().DeleteReward(gomock.Any(), rewardID).Return(nil)

	err := processor.DeleteReward(ctx, campaignID, rewardID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDeleteReward_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockRewardStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	campaignID := uuid.New()
	rewardID := uuid.New()

	mockStore.EXPECT().GetRewardByID(gomock.Any(), rewardID).Return(store.Reward{}, store.ErrNotFound)

	err := processor.DeleteReward(ctx, campaignID, rewardID)

	if !errors.Is(err, ErrRewardNotFound) {
		t.Errorf("expected ErrRewardNotFound, got %v", err)
	}
}

func TestGrantReward_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockRewardStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	campaignID := uuid.New()
	userID := uuid.New()
	rewardID := uuid.New()
	userRewardID := uuid.New()

	mockStore.EXPECT().GetRewardByID(gomock.Any(), rewardID).Return(store.Reward{
		ID:         rewardID,
		CampaignID: campaignID,
		Name:       "Test Reward",
		Status:     "active",
		UserLimit:  3,
	}, nil)
	mockStore.EXPECT().GetUserRewardsByUser(gomock.Any(), userID).Return([]store.UserReward{}, nil)
	mockStore.EXPECT().CreateUserReward(gomock.Any(), gomock.Any()).Return(store.UserReward{
		ID:         userRewardID,
		UserID:     userID,
		RewardID:   rewardID,
		CampaignID: campaignID,
	}, nil)
	mockStore.EXPECT().IncrementRewardClaimed(gomock.Any(), rewardID).Return(nil)

	req := GrantRewardRequest{
		RewardID: rewardID,
	}

	result, err := processor.GrantReward(ctx, campaignID, userID, req)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.ID != userRewardID {
		t.Errorf("expected user reward ID %s, got %s", userRewardID, result.ID)
	}
}

func TestGrantReward_UserLimitReached(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockRewardStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	campaignID := uuid.New()
	userID := uuid.New()
	rewardID := uuid.New()

	mockStore.EXPECT().GetRewardByID(gomock.Any(), rewardID).Return(store.Reward{
		ID:         rewardID,
		CampaignID: campaignID,
		Status:     "active",
		UserLimit:  1,
	}, nil)
	mockStore.EXPECT().GetUserRewardsByUser(gomock.Any(), userID).Return([]store.UserReward{
		{RewardID: rewardID},
	}, nil)

	req := GrantRewardRequest{
		RewardID: rewardID,
	}

	_, err := processor.GrantReward(ctx, campaignID, userID, req)

	if !errors.Is(err, ErrUserLimitReached) {
		t.Errorf("expected ErrUserLimitReached, got %v", err)
	}
}

func TestGetUserRewards_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockRewardStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	campaignID := uuid.New()
	userID := uuid.New()

	rewards := []store.UserReward{
		{ID: uuid.New(), CampaignID: campaignID},
		{ID: uuid.New(), CampaignID: campaignID},
		{ID: uuid.New(), CampaignID: uuid.New()}, // Different campaign
	}

	mockStore.EXPECT().GetUserRewardsByUser(gomock.Any(), userID).Return(rewards, nil)

	result, err := processor.GetUserRewards(ctx, campaignID, userID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 rewards (filtered by campaign), got %d", len(result))
	}
}
