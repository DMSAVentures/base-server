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

func TestSignupUser_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	mockEventDispatcher := NewMockEventDispatcher(ctrl)
	mockCaptcha := NewMockCaptchaVerifier(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, mockEventDispatcher, mockCaptcha)

	ctx := context.Background()
	campaignID := uuid.New()
	accountID := uuid.New()
	userID := uuid.New()
	email := "test@example.com"

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
		Slug:      "test-campaign",
		Status:    store.CampaignStatusActive,
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)
	mockStore.EXPECT().GetWaitlistUserByEmail(gomock.Any(), campaignID, email).Return(store.WaitlistUser{}, store.ErrNotFound)
	mockCaptcha.EXPECT().IsEnabled().Return(false)
	mockStore.EXPECT().CreateWaitlistUser(gomock.Any(), gomock.Any()).Return(store.WaitlistUser{
		ID:           userID,
		CampaignID:   campaignID,
		Email:        email,
		ReferralCode: "ABC123",
	}, nil)
	mockEventDispatcher.EXPECT().DispatchUserCreated(gomock.Any(), accountID, campaignID, gomock.Any())

	req := SignupUserRequest{
		Email: email,
	}

	result, err := processor.SignupUser(ctx, campaignID, req, "https://example.com")

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.User.ID != userID {
		t.Errorf("expected user ID %s, got %s", userID, result.User.ID)
	}
}

func TestSignupUser_CampaignNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{}, store.ErrNotFound)

	req := SignupUserRequest{
		Email: "test@example.com",
	}

	_, err := processor.SignupUser(ctx, campaignID, req, "https://example.com")

	if !errors.Is(err, ErrCampaignNotFound) {
		t.Errorf("expected ErrCampaignNotFound, got %v", err)
	}
}

func TestSignupUser_EmailAlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	mockCaptcha := NewMockCaptchaVerifier(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, mockCaptcha)

	ctx := context.Background()
	campaignID := uuid.New()
	accountID := uuid.New()
	email := "existing@example.com"

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Status:    store.CampaignStatusActive,
	}, nil)
	mockStore.EXPECT().GetWaitlistUserByEmail(gomock.Any(), campaignID, email).Return(store.WaitlistUser{
		ID:    uuid.New(),
		Email: email,
	}, nil)

	req := SignupUserRequest{
		Email: email,
	}

	_, err := processor.SignupUser(ctx, campaignID, req, "https://example.com")

	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestSignupUser_CampaignNotActive_Draft(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	campaignID := uuid.New()
	accountID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Status:    store.CampaignStatusDraft,
	}, nil)

	req := SignupUserRequest{
		Email: "test@example.com",
	}

	_, err := processor.SignupUser(ctx, campaignID, req, "https://example.com")

	if !errors.Is(err, ErrCampaignNotActive) {
		t.Errorf("expected ErrCampaignNotActive, got %v", err)
	}
}

func TestSignupUser_CampaignNotActive_Paused(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	campaignID := uuid.New()
	accountID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Status:    store.CampaignStatusPaused,
	}, nil)

	req := SignupUserRequest{
		Email: "test@example.com",
	}

	_, err := processor.SignupUser(ctx, campaignID, req, "https://example.com")

	if !errors.Is(err, ErrCampaignNotActive) {
		t.Errorf("expected ErrCampaignNotActive, got %v", err)
	}
}

func TestSignupUser_CampaignNotActive_Completed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	campaignID := uuid.New()
	accountID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Status:    store.CampaignStatusCompleted,
	}, nil)

	req := SignupUserRequest{
		Email: "test@example.com",
	}

	_, err := processor.SignupUser(ctx, campaignID, req, "https://example.com")

	if !errors.Is(err, ErrCampaignNotActive) {
		t.Errorf("expected ErrCampaignNotActive, got %v", err)
	}
}

func TestListUsers_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	users := []store.WaitlistUser{
		{ID: uuid.New(), Email: "user1@example.com"},
		{ID: uuid.New(), Email: "user2@example.com"},
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)
	mockStore.EXPECT().ListWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).Return(users, nil)
	mockStore.EXPECT().CountWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).Return(2, nil)

	result, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{Page: 1, Limit: 20})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(result.Users) != 2 {
		t.Errorf("expected 2 users, got %d", len(result.Users))
	}
}

func TestListUsers_CampaignNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{}, store.ErrNotFound)

	_, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{})

	if !errors.Is(err, ErrCampaignNotFound) {
		t.Errorf("expected ErrCampaignNotFound, got %v", err)
	}
}

func TestListUsers_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	otherAccountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: otherAccountID,
	}, nil)

	_, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{})

	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestGetUser_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()
	userID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)
	mockStore.EXPECT().GetWaitlistUserByID(gomock.Any(), userID).Return(store.WaitlistUser{
		ID:         userID,
		CampaignID: campaignID,
		Email:      "test@example.com",
	}, nil)

	result, err := processor.GetUser(ctx, accountID, campaignID, userID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.ID != userID {
		t.Errorf("expected user ID %s, got %s", userID, result.ID)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()
	userID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)
	mockStore.EXPECT().GetWaitlistUserByID(gomock.Any(), userID).Return(store.WaitlistUser{}, store.ErrNotFound)

	_, err := processor.GetUser(ctx, accountID, campaignID, userID)

	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestDeleteUser_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()
	userID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)
	mockStore.EXPECT().GetWaitlistUserByID(gomock.Any(), userID).Return(store.WaitlistUser{
		ID:         userID,
		CampaignID: campaignID,
	}, nil)
	mockStore.EXPECT().DeleteWaitlistUser(gomock.Any(), userID).Return(nil)

	err := processor.DeleteUser(ctx, accountID, campaignID, userID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()
	userID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)
	mockStore.EXPECT().GetWaitlistUserByID(gomock.Any(), userID).Return(store.WaitlistUser{}, store.ErrNotFound)

	err := processor.DeleteUser(ctx, accountID, campaignID, userID)

	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestVerifyUserByToken_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	mockEventDispatcher := NewMockEventDispatcher(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, mockEventDispatcher, nil)

	ctx := context.Background()
	campaignID := uuid.New()
	accountID := uuid.New()
	userID := uuid.New()
	token := "verification-token"

	mockStore.EXPECT().GetWaitlistUserByVerificationToken(gomock.Any(), token).Return(store.WaitlistUser{
		ID:            userID,
		CampaignID:    campaignID,
		EmailVerified: false,
	}, nil)
	mockStore.EXPECT().VerifyWaitlistUserEmail(gomock.Any(), userID).Return(nil)
	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
		Slug:      "test-campaign",
	}, nil)
	mockEventDispatcher.EXPECT().DispatchUserVerified(gomock.Any(), accountID, campaignID, gomock.Any())

	err := processor.VerifyUserByToken(ctx, campaignID, token)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestVerifyUserByToken_InvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	campaignID := uuid.New()
	token := "invalid-token"

	mockStore.EXPECT().GetWaitlistUserByVerificationToken(gomock.Any(), token).Return(store.WaitlistUser{}, store.ErrNotFound)

	err := processor.VerifyUserByToken(ctx, campaignID, token)

	if !errors.Is(err, ErrInvalidVerificationToken) {
		t.Errorf("expected ErrInvalidVerificationToken, got %v", err)
	}
}

func TestVerifyUserByToken_AlreadyVerified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	campaignID := uuid.New()
	userID := uuid.New()
	token := "verification-token"

	mockStore.EXPECT().GetWaitlistUserByVerificationToken(gomock.Any(), token).Return(store.WaitlistUser{
		ID:            userID,
		CampaignID:    campaignID,
		EmailVerified: true,
	}, nil)

	err := processor.VerifyUserByToken(ctx, campaignID, token)

	if !errors.Is(err, ErrEmailAlreadyVerified) {
		t.Errorf("expected ErrEmailAlreadyVerified, got %v", err)
	}
}

func TestListUsers_WithExtendedFilters_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	users := []store.WaitlistUser{
		{ID: uuid.New(), Email: "user1@example.com", Position: 1},
		{ID: uuid.New(), Email: "user2@example.com", Position: 2},
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)
	mockStore.EXPECT().ListWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).Return(users, nil)
	mockStore.EXPECT().CountWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).Return(2, nil)

	result, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{
		Page:  1,
		Limit: 20,
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(result.Users) != 2 {
		t.Errorf("expected 2 users, got %d", len(result.Users))
	}
	if result.TotalCount != 2 {
		t.Errorf("expected total count 2, got %d", result.TotalCount)
	}
}

func TestListUsers_WithStatusFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	users := []store.WaitlistUser{
		{ID: uuid.New(), Email: "verified@example.com", Status: store.WaitlistUserStatusVerified},
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)

	// Verify filter params are correctly passed
	mockStore.EXPECT().ListWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.ExtendedListWaitlistUsersParams) ([]store.WaitlistUser, error) {
			if len(params.Statuses) != 1 || params.Statuses[0] != "verified" {
				t.Errorf("expected status filter [verified], got %v", params.Statuses)
			}
			return users, nil
		})
	mockStore.EXPECT().CountWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).Return(1, nil)

	result, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{
		Statuses: []string{"verified"},
		Page:     1,
		Limit:    20,
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(result.Users) != 1 {
		t.Errorf("expected 1 user, got %d", len(result.Users))
	}
}

func TestListUsers_WithSourceFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)

	// Verify multiple sources are passed correctly
	mockStore.EXPECT().ListWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.ExtendedListWaitlistUsersParams) ([]store.WaitlistUser, error) {
			if len(params.Sources) != 2 {
				t.Errorf("expected 2 sources, got %d", len(params.Sources))
			}
			return []store.WaitlistUser{}, nil
		})
	mockStore.EXPECT().CountWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).Return(0, nil)

	_, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{
		Sources: []string{"direct", "referral"},
		Page:    1,
		Limit:   20,
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestListUsers_WithPositionRangeFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)

	minPos := 5
	maxPos := 10
	mockStore.EXPECT().ListWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.ExtendedListWaitlistUsersParams) ([]store.WaitlistUser, error) {
			if params.MinPosition == nil || *params.MinPosition != minPos {
				t.Errorf("expected min position %d, got %v", minPos, params.MinPosition)
			}
			if params.MaxPosition == nil || *params.MaxPosition != maxPos {
				t.Errorf("expected max position %d, got %v", maxPos, params.MaxPosition)
			}
			return []store.WaitlistUser{}, nil
		})
	mockStore.EXPECT().CountWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).Return(0, nil)

	_, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{
		MinPosition: &minPos,
		MaxPosition: &maxPos,
		Page:        1,
		Limit:       20,
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestListUsers_WithHasReferralsFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)

	hasReferrals := true
	mockStore.EXPECT().ListWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.ExtendedListWaitlistUsersParams) ([]store.WaitlistUser, error) {
			if params.HasReferrals == nil || *params.HasReferrals != true {
				t.Errorf("expected hasReferrals true, got %v", params.HasReferrals)
			}
			return []store.WaitlistUser{}, nil
		})
	mockStore.EXPECT().CountWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).Return(0, nil)

	_, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{
		HasReferrals: &hasReferrals,
		Page:         1,
		Limit:        20,
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestListUsers_WithCustomFieldsFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)

	customFields := map[string]string{
		"company": "Acme",
		"role":    "Developer",
	}

	mockStore.EXPECT().ListWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.ExtendedListWaitlistUsersParams) ([]store.WaitlistUser, error) {
			if len(params.CustomFields) != 2 {
				t.Errorf("expected 2 custom fields, got %d", len(params.CustomFields))
			}
			if params.CustomFields["company"] != "Acme" {
				t.Errorf("expected company=Acme, got %s", params.CustomFields["company"])
			}
			if params.CustomFields["role"] != "Developer" {
				t.Errorf("expected role=Developer, got %s", params.CustomFields["role"])
			}
			return []store.WaitlistUser{}, nil
		})
	mockStore.EXPECT().CountWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).Return(0, nil)

	_, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{
		CustomFields: customFields,
		Page:         1,
		Limit:        20,
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestListUsers_WithDateRangeFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)

	dateFrom := "2024-01-01"
	dateTo := "2024-12-31"

	mockStore.EXPECT().ListWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.ExtendedListWaitlistUsersParams) ([]store.WaitlistUser, error) {
			if params.DateFrom == nil {
				t.Error("expected DateFrom to be set")
			}
			if params.DateTo == nil {
				t.Error("expected DateTo to be set")
			}
			return []store.WaitlistUser{}, nil
		})
	mockStore.EXPECT().CountWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).Return(0, nil)

	_, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{
		DateFrom: &dateFrom,
		DateTo:   &dateTo,
		Page:     1,
		Limit:    20,
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestListUsers_WithSortingParams(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)

	mockStore.EXPECT().ListWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.ExtendedListWaitlistUsersParams) ([]store.WaitlistUser, error) {
			if params.SortBy != "created_at" {
				t.Errorf("expected SortBy=created_at, got %s", params.SortBy)
			}
			if params.SortOrder != "desc" {
				t.Errorf("expected SortOrder=desc, got %s", params.SortOrder)
			}
			return []store.WaitlistUser{}, nil
		})
	mockStore.EXPECT().CountWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).Return(0, nil)

	_, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{
		SortBy:    "created_at",
		SortOrder: "desc",
		Page:      1,
		Limit:     20,
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestListUsers_WithCombinedFilters(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)

	hasReferrals := true
	minPos := 1
	maxPos := 100

	mockStore.EXPECT().ListWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.ExtendedListWaitlistUsersParams) ([]store.WaitlistUser, error) {
			// Verify all filters are passed
			if len(params.Statuses) != 1 || params.Statuses[0] != "verified" {
				t.Errorf("expected status verified, got %v", params.Statuses)
			}
			if len(params.Sources) != 1 || params.Sources[0] != "social" {
				t.Errorf("expected source social, got %v", params.Sources)
			}
			if params.HasReferrals == nil || !*params.HasReferrals {
				t.Error("expected hasReferrals=true")
			}
			if params.MinPosition == nil || *params.MinPosition != 1 {
				t.Errorf("expected minPosition=1, got %v", params.MinPosition)
			}
			if params.MaxPosition == nil || *params.MaxPosition != 100 {
				t.Errorf("expected maxPosition=100, got %v", params.MaxPosition)
			}
			if params.CustomFields["company"] != "Acme" {
				t.Errorf("expected company=Acme, got %v", params.CustomFields["company"])
			}
			return []store.WaitlistUser{}, nil
		})
	mockStore.EXPECT().CountWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).Return(0, nil)

	_, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{
		Statuses:     []string{"verified"},
		Sources:      []string{"social"},
		HasReferrals: &hasReferrals,
		MinPosition:  &minPos,
		MaxPosition:  &maxPos,
		CustomFields: map[string]string{"company": "Acme"},
		Page:         1,
		Limit:        20,
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestListUsers_WithInvalidStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)

	_, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{
		Statuses: []string{"invalid_status"},
		Page:     1,
		Limit:    20,
	})

	if !errors.Is(err, ErrInvalidStatus) {
		t.Errorf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestListUsers_WithInvalidSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)

	// Test with "facebook" which is a valid referral_source but NOT a valid user_source
	_, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{
		Sources: []string{"facebook"},
		Page:    1,
		Limit:   20,
	})

	if !errors.Is(err, ErrInvalidSource) {
		t.Errorf("expected ErrInvalidSource, got %v", err)
	}
}

func TestListUsers_WithInvalidSourceAmongValid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)

	// Test with mix of valid and invalid sources - should fail on invalid
	_, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{
		Sources: []string{"referral", "facebook"}, // referral is valid, facebook is not
		Page:    1,
		Limit:   20,
	})

	if !errors.Is(err, ErrInvalidSource) {
		t.Errorf("expected ErrInvalidSource, got %v", err)
	}
}

func TestListUsers_Pagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)

	// Verify offset calculation: page 3 with limit 25 = offset 50
	mockStore.EXPECT().ListWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.ExtendedListWaitlistUsersParams) ([]store.WaitlistUser, error) {
			if params.Limit != 25 {
				t.Errorf("expected limit 25, got %d", params.Limit)
			}
			if params.Offset != 50 { // (3-1) * 25 = 50
				t.Errorf("expected offset 50, got %d", params.Offset)
			}
			return []store.WaitlistUser{}, nil
		})
	mockStore.EXPECT().CountWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).Return(100, nil)

	result, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{
		Page:  3,
		Limit: 25,
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Page != 3 {
		t.Errorf("expected page 3, got %d", result.Page)
	}
	if result.PageSize != 25 {
		t.Errorf("expected page size 25, got %d", result.PageSize)
	}
	if result.TotalPages != 4 { // 100 / 25 = 4
		t.Errorf("expected 4 total pages, got %d", result.TotalPages)
	}
}

func TestListUsers_EmptyResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, createTestTierService(), logger, nil, nil)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
	}, nil)
	mockStore.EXPECT().ListWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).Return(nil, nil)
	mockStore.EXPECT().CountWaitlistUsersWithExtendedFilters(gomock.Any(), gomock.Any()).Return(0, nil)

	result, err := processor.ListUsers(ctx, accountID, campaignID, ListUsersRequest{
		Page:  1,
		Limit: 20,
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Users == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(result.Users) != 0 {
		t.Errorf("expected 0 users, got %d", len(result.Users))
	}
}
