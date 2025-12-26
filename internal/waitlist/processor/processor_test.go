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

func TestSignupUser_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	mockEventDispatcher := NewMockEventDispatcher(ctrl)
	mockCaptcha := NewMockCaptchaVerifier(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockEventDispatcher, mockCaptcha)

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

	processor := New(mockStore, logger, nil, nil)

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

	processor := New(mockStore, logger, nil, mockCaptcha)

	ctx := context.Background()
	campaignID := uuid.New()
	accountID := uuid.New()
	email := "existing@example.com"

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
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

func TestListUsers_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWaitlistStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, nil, nil)

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
	mockStore.EXPECT().GetWaitlistUsersByCampaignWithFilters(gomock.Any(), gomock.Any()).Return(users, nil)
	mockStore.EXPECT().CountWaitlistUsersWithFilters(gomock.Any(), campaignID, gomock.Any(), gomock.Any()).Return(2, nil)

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

	processor := New(mockStore, logger, nil, nil)

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

	processor := New(mockStore, logger, nil, nil)

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

	processor := New(mockStore, logger, nil, nil)

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

	processor := New(mockStore, logger, nil, nil)

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

	processor := New(mockStore, logger, nil, nil)

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

	processor := New(mockStore, logger, nil, nil)

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

	processor := New(mockStore, logger, mockEventDispatcher, nil)

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

	processor := New(mockStore, logger, nil, nil)

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

	processor := New(mockStore, logger, nil, nil)

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
