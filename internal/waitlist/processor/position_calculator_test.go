package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockWaitlistStore is a mock implementation of WaitlistStore
type MockWaitlistStore struct {
	mock.Mock
}

func (m *MockWaitlistStore) GetWaitlistUserByID(ctx context.Context, userID uuid.UUID) (store.WaitlistUser, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(store.WaitlistUser), args.Error(1)
}

func (m *MockWaitlistStore) GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (store.Campaign, error) {
	args := m.Called(ctx, campaignID)
	return args.Get(0).(store.Campaign), args.Error(1)
}

func (m *MockWaitlistStore) UpdateWaitlistUserPosition(ctx context.Context, userID uuid.UUID, position int) error {
	args := m.Called(ctx, userID, position)
	return args.Error(0)
}

func (m *MockWaitlistStore) GetAllWaitlistUsersForPositionCalculation(ctx context.Context, campaignID uuid.UUID) ([]store.WaitlistUser, error) {
	args := m.Called(ctx, campaignID)
	return args.Get(0).([]store.WaitlistUser), args.Error(1)
}

func (m *MockWaitlistStore) BulkUpdateWaitlistUserPositions(ctx context.Context, userIDs []uuid.UUID, positions []int) error {
	args := m.Called(ctx, userIDs, positions)
	return args.Error(0)
}

func TestCalculateUserPosition_BasicFormula(t *testing.T) {
	// Test basic formula: position = original_position - (referral_count × positions_per_referral)
	mockStore := new(MockWaitlistStore)
	logger := observability.NewLogger()
	calc := NewPositionCalculator(mockStore, logger)

	userID := uuid.New()
	campaignID := uuid.New()

	// Setup: User at position 100 with 5 referrals, positions_per_referral = 1
	user := store.WaitlistUser{
		ID:               userID,
		CampaignID:       campaignID,
		OriginalPosition: 100,
		Position:         100,
		ReferralCount:    5,
	}

	campaign := store.Campaign{
		ID:             campaignID,
		ReferralConfig: store.JSONB{"positions_per_referral": float64(1)},
	}

	mockStore.On("GetWaitlistUserByID", mock.Anything, userID).Return(user, nil)
	mockStore.On("GetCampaignByID", mock.Anything, campaignID).Return(campaign, nil)
	mockStore.On("UpdateWaitlistUserPosition", mock.Anything, userID, 95).Return(nil)

	// Execute
	err := calc.CalculateUserPosition(context.Background(), userID)

	// Assert
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
}

func TestCalculateUserPosition_MultiplePositionsPerReferral(t *testing.T) {
	// Test with positions_per_referral = 5
	mockStore := new(MockWaitlistStore)
	logger := observability.NewLogger()
	calc := NewPositionCalculator(mockStore, logger)

	userID := uuid.New()
	campaignID := uuid.New()

	// Setup: User at position 100 with 3 referrals, positions_per_referral = 5
	// Expected: 100 - (3 × 5) = 85
	user := store.WaitlistUser{
		ID:               userID,
		CampaignID:       campaignID,
		OriginalPosition: 100,
		Position:         100,
		ReferralCount:    3,
	}

	campaign := store.Campaign{
		ID:             campaignID,
		ReferralConfig: store.JSONB{"positions_per_referral": float64(5)},
	}

	mockStore.On("GetWaitlistUserByID", mock.Anything, userID).Return(user, nil)
	mockStore.On("GetCampaignByID", mock.Anything, campaignID).Return(campaign, nil)
	mockStore.On("UpdateWaitlistUserPosition", mock.Anything, userID, 85).Return(nil)

	// Execute
	err := calc.CalculateUserPosition(context.Background(), userID)

	// Assert
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
}

func TestCalculateUserPosition_MinPositionCap(t *testing.T) {
	// Test that position cannot go below 1
	mockStore := new(MockWaitlistStore)
	logger := observability.NewLogger()
	calc := NewPositionCalculator(mockStore, logger)

	userID := uuid.New()
	campaignID := uuid.New()

	// Setup: User at position 10 with 20 referrals, positions_per_referral = 1
	// Expected: 10 - (20 × 1) = -10, but capped at 1
	user := store.WaitlistUser{
		ID:               userID,
		CampaignID:       campaignID,
		OriginalPosition: 10,
		Position:         10,
		ReferralCount:    20,
	}

	campaign := store.Campaign{
		ID:             campaignID,
		ReferralConfig: store.JSONB{"positions_per_referral": float64(1)},
	}

	mockStore.On("GetWaitlistUserByID", mock.Anything, userID).Return(user, nil)
	mockStore.On("GetCampaignByID", mock.Anything, campaignID).Return(campaign, nil)
	mockStore.On("UpdateWaitlistUserPosition", mock.Anything, userID, 1).Return(nil)

	// Execute
	err := calc.CalculateUserPosition(context.Background(), userID)

	// Assert
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
}

func TestCalculateUserPosition_NoUpdate_PositionUnchanged(t *testing.T) {
	// Test that no update occurs if position hasn't changed
	mockStore := new(MockWaitlistStore)
	logger := observability.NewLogger()
	calc := NewPositionCalculator(mockStore, logger)

	userID := uuid.New()
	campaignID := uuid.New()

	// Setup: User at position 100, 0 referrals, original position 100
	// Expected: 100 - (0 × 1) = 100, no change
	user := store.WaitlistUser{
		ID:               userID,
		CampaignID:       campaignID,
		OriginalPosition: 100,
		Position:         100,
		ReferralCount:    0,
	}

	campaign := store.Campaign{
		ID:             campaignID,
		ReferralConfig: store.JSONB{"positions_per_referral": float64(1)},
	}

	mockStore.On("GetWaitlistUserByID", mock.Anything, userID).Return(user, nil)
	mockStore.On("GetCampaignByID", mock.Anything, campaignID).Return(campaign, nil)
	// Note: UpdateWaitlistUserPosition should NOT be called

	// Execute
	err := calc.CalculateUserPosition(context.Background(), userID)

	// Assert
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
	mockStore.AssertNotCalled(t, "UpdateWaitlistUserPosition")
}

func TestCalculateUserPosition_VerifiedReferralsOnly(t *testing.T) {
	// Test using verified_referral_count when verification is required
	mockStore := new(MockWaitlistStore)
	logger := observability.NewLogger()
	calc := NewPositionCalculator(mockStore, logger)

	userID := uuid.New()
	campaignID := uuid.New()

	// Setup: User has 10 total referrals but only 3 verified
	user := store.WaitlistUser{
		ID:                    userID,
		CampaignID:            campaignID,
		OriginalPosition:      100,
		Position:              100,
		ReferralCount:         10,
		VerifiedReferralCount: 3,
	}

	campaign := store.Campaign{
		ID:             campaignID,
		ReferralConfig: store.JSONB{"positions_per_referral": float64(1)},
		EmailConfig:    store.JSONB{"verification_required": true},
	}

	mockStore.On("GetWaitlistUserByID", mock.Anything, userID).Return(user, nil)
	mockStore.On("GetCampaignByID", mock.Anything, campaignID).Return(campaign, nil)
	// Expected: 100 - (3 × 1) = 97 (using verified count, not total)
	mockStore.On("UpdateWaitlistUserPosition", mock.Anything, userID, 97).Return(nil)

	// Execute
	err := calc.CalculateUserPosition(context.Background(), userID)

	// Assert
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
}

func TestCalculateUserPosition_MaxPositionsPerReferralCap(t *testing.T) {
	// Test that positions_per_referral is capped at 100
	mockStore := new(MockWaitlistStore)
	logger := observability.NewLogger()
	calc := NewPositionCalculator(mockStore, logger)

	userID := uuid.New()
	campaignID := uuid.New()

	// Setup: positions_per_referral = 500, should be capped at 100
	user := store.WaitlistUser{
		ID:               userID,
		CampaignID:       campaignID,
		OriginalPosition: 200,
		Position:         200,
		ReferralCount:    1,
	}

	campaign := store.Campaign{
		ID:             campaignID,
		ReferralConfig: store.JSONB{"positions_per_referral": float64(500)},
	}

	mockStore.On("GetWaitlistUserByID", mock.Anything, userID).Return(user, nil)
	mockStore.On("GetCampaignByID", mock.Anything, campaignID).Return(campaign, nil)
	// Expected: 200 - (1 × 100) = 100 (capped multiplier)
	mockStore.On("UpdateWaitlistUserPosition", mock.Anything, userID, 100).Return(nil)

	// Execute
	err := calc.CalculateUserPosition(context.Background(), userID)

	// Assert
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
}

func TestCalculateUserPosition_DefaultPositionsPerReferral(t *testing.T) {
	// Test default positions_per_referral = 1 when not configured
	mockStore := new(MockWaitlistStore)
	logger := observability.NewLogger()
	calc := NewPositionCalculator(mockStore, logger)

	userID := uuid.New()
	campaignID := uuid.New()

	// Setup: No referral_config specified
	user := store.WaitlistUser{
		ID:               userID,
		CampaignID:       campaignID,
		OriginalPosition: 100,
		Position:         100,
		ReferralCount:    5,
	}

	campaign := store.Campaign{
		ID:             campaignID,
		ReferralConfig: nil, // No config
	}

	mockStore.On("GetWaitlistUserByID", mock.Anything, userID).Return(user, nil)
	mockStore.On("GetCampaignByID", mock.Anything, campaignID).Return(campaign, nil)
	// Expected: 100 - (5 × 1) = 95 (default multiplier = 1)
	mockStore.On("UpdateWaitlistUserPosition", mock.Anything, userID, 95).Return(nil)

	// Execute
	err := calc.CalculateUserPosition(context.Background(), userID)

	// Assert
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
}
