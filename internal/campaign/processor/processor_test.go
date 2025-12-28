package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"base-server/internal/tiers"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

// mockTierStore is a test implementation that returns unlimited tier info
type mockTierStore struct{}

func (m *mockTierStore) GetTierInfoByAccountID(ctx context.Context, accountID uuid.UUID) (store.TierInfo, error) {
	return store.TierInfo{
		PriceDescription: "team",
		Features:         map[string]bool{"webhooks_zapier": true, "email_blasts": true, "json_export": true},
		Limits:           map[string]*int{"campaigns": nil, "leads": nil}, // nil means unlimited
	}, nil
}

func (m *mockTierStore) GetTierInfoByUserID(ctx context.Context, userID uuid.UUID) (store.TierInfo, error) {
	return m.GetTierInfoByAccountID(ctx, uuid.Nil)
}

func (m *mockTierStore) GetTierInfoByPriceID(ctx context.Context, priceID uuid.UUID) (store.TierInfo, error) {
	return m.GetTierInfoByAccountID(ctx, uuid.Nil)
}

func (m *mockTierStore) GetFreeTierInfo(ctx context.Context) (store.TierInfo, error) {
	return m.GetTierInfoByAccountID(ctx, uuid.Nil)
}

// createTestTierService creates a TierService with unlimited access for testing
func createTestTierService() *tiers.TierService {
	logger := observability.NewLogger()
	return tiers.New(&mockTierStore{}, logger)
}

func TestCreateCampaign_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockCampaignStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()
	params := CreateCampaignParams{
		Name: "Test Campaign",
		Slug: "test-campaign",
		Type: "waitlist",
	}

	mockStore.EXPECT().GetCampaignBySlug(gomock.Any(), accountID, params.Slug).
		Return(store.Campaign{}, store.ErrNotFound)
	mockStore.EXPECT().CreateCampaign(gomock.Any(), gomock.Any()).
		Return(store.Campaign{ID: campaignID, Name: params.Name, Slug: params.Slug, Type: params.Type}, nil)
	mockStore.EXPECT().GetCampaignWithSettings(gomock.Any(), campaignID).
		Return(store.Campaign{ID: campaignID, Name: params.Name, Slug: params.Slug, Type: params.Type}, nil)

	result, err := processor.CreateCampaign(ctx, accountID, params)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Name != params.Name {
		t.Errorf("expected name %s, got %s", params.Name, result.Name)
	}
}

func TestCreateCampaign_SlugExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockCampaignStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger)

	ctx := context.Background()
	accountID := uuid.New()
	params := CreateCampaignParams{
		Name: "Test Campaign",
		Slug: "existing-slug",
		Type: "waitlist",
	}

	mockStore.EXPECT().GetCampaignBySlug(gomock.Any(), accountID, params.Slug).
		Return(store.Campaign{ID: uuid.New()}, nil)

	_, err := processor.CreateCampaign(ctx, accountID, params)

	if !errors.Is(err, ErrSlugAlreadyExists) {
		t.Errorf("expected ErrSlugAlreadyExists, got %v", err)
	}
}

func TestGetCampaign_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockCampaignStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignWithSettings(gomock.Any(), campaignID).
		Return(store.Campaign{ID: campaignID, AccountID: accountID, Name: "Test"}, nil)

	result, err := processor.GetCampaign(ctx, accountID, campaignID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.ID != campaignID {
		t.Errorf("expected campaign ID %s, got %s", campaignID, result.ID)
	}
}

func TestGetCampaign_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockCampaignStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignWithSettings(gomock.Any(), campaignID).
		Return(store.Campaign{}, store.ErrNotFound)

	_, err := processor.GetCampaign(ctx, accountID, campaignID)

	if !errors.Is(err, ErrCampaignNotFound) {
		t.Errorf("expected ErrCampaignNotFound, got %v", err)
	}
}

func TestGetCampaign_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockCampaignStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger)

	ctx := context.Background()
	accountID := uuid.New()
	otherAccountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignWithSettings(gomock.Any(), campaignID).
		Return(store.Campaign{ID: campaignID, AccountID: otherAccountID}, nil)

	_, err := processor.GetCampaign(ctx, accountID, campaignID)

	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestListCampaigns_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockCampaignStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger)

	ctx := context.Background()
	accountID := uuid.New()

	campaigns := []store.Campaign{
		{ID: uuid.New(), Name: "Campaign 1"},
		{ID: uuid.New(), Name: "Campaign 2"},
	}

	mockStore.EXPECT().ListCampaigns(gomock.Any(), gomock.Any()).
		Return(store.ListCampaignsResult{Campaigns: campaigns, TotalCount: 2}, nil)

	result, err := processor.ListCampaigns(ctx, accountID, nil, nil, 1, 10)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(result.Campaigns) != 2 {
		t.Errorf("expected 2 campaigns, got %d", len(result.Campaigns))
	}
}

func TestDeleteCampaign_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockCampaignStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().DeleteCampaign(gomock.Any(), accountID, campaignID).
		Return(nil)

	err := processor.DeleteCampaign(ctx, accountID, campaignID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDeleteCampaign_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockCampaignStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().DeleteCampaign(gomock.Any(), accountID, campaignID).
		Return(store.ErrNotFound)

	err := processor.DeleteCampaign(ctx, accountID, campaignID)

	if !errors.Is(err, ErrCampaignNotFound) {
		t.Errorf("expected ErrCampaignNotFound, got %v", err)
	}
}
