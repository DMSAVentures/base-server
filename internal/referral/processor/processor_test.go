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

func TestListReferrals_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockReferralStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	referrals := []store.Referral{
		{ID: uuid.New()},
		{ID: uuid.New()},
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).
		Return(store.Campaign{ID: campaignID, AccountID: accountID}, nil)
	mockStore.EXPECT().GetReferralsByCampaignWithStatusFilter(gomock.Any(), campaignID, nil, 20, 0).
		Return(referrals, nil)
	mockStore.EXPECT().CountReferralsByCampaignWithStatusFilter(gomock.Any(), campaignID, nil).
		Return(2, nil)

	result, err := processor.ListReferrals(ctx, accountID, campaignID, ListReferralsRequest{Page: 1, Limit: 20})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(result.Referrals) != 2 {
		t.Errorf("expected 2 referrals, got %d", len(result.Referrals))
	}
}

func TestListReferrals_CampaignNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockReferralStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).
		Return(store.Campaign{}, store.ErrNotFound)

	_, err := processor.ListReferrals(ctx, accountID, campaignID, ListReferralsRequest{})

	if !errors.Is(err, ErrCampaignNotFound) {
		t.Errorf("expected ErrCampaignNotFound, got %v", err)
	}
}

func TestListReferrals_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockReferralStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	otherAccountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).
		Return(store.Campaign{ID: campaignID, AccountID: otherAccountID}, nil)

	_, err := processor.ListReferrals(ctx, accountID, campaignID, ListReferralsRequest{})

	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestTrackReferral_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockReferralStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	campaignID := uuid.New()
	referrerID := uuid.New()
	referralCode := "ABC123"
	firstName := "John"

	mockStore.EXPECT().GetWaitlistUserByReferralCode(gomock.Any(), referralCode).
		Return(store.WaitlistUser{
			ID:         referrerID,
			CampaignID: campaignID,
			FirstName:  &firstName,
			Email:      "test@example.com",
		}, nil)

	result, err := processor.TrackReferral(ctx, campaignID, TrackReferralRequest{ReferralCode: referralCode})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Referrer.ID != referrerID {
		t.Errorf("expected referrer ID %s, got %s", referrerID, result.Referrer.ID)
	}
}

func TestTrackReferral_EmptyCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockReferralStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	campaignID := uuid.New()

	_, err := processor.TrackReferral(ctx, campaignID, TrackReferralRequest{ReferralCode: ""})

	if !errors.Is(err, ErrReferralCodeEmpty) {
		t.Errorf("expected ErrReferralCodeEmpty, got %v", err)
	}
}

func TestTrackReferral_InvalidCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockReferralStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	campaignID := uuid.New()
	referralCode := "INVALID"

	mockStore.EXPECT().GetWaitlistUserByReferralCode(gomock.Any(), referralCode).
		Return(store.WaitlistUser{}, store.ErrNotFound)

	_, err := processor.TrackReferral(ctx, campaignID, TrackReferralRequest{ReferralCode: referralCode})

	if !errors.Is(err, ErrInvalidReferral) {
		t.Errorf("expected ErrInvalidReferral, got %v", err)
	}
}

func TestGetUserReferrals_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockReferralStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()
	userID := uuid.New()

	referrals := []store.Referral{{ID: uuid.New()}}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).
		Return(store.Campaign{ID: campaignID, AccountID: accountID}, nil)
	mockStore.EXPECT().GetWaitlistUserByID(gomock.Any(), userID).
		Return(store.WaitlistUser{ID: userID, CampaignID: campaignID}, nil)
	mockStore.EXPECT().GetReferralsByReferrerWithPagination(gomock.Any(), userID, 20, 0).
		Return(referrals, nil)
	mockStore.EXPECT().CountReferralsByReferrer(gomock.Any(), userID).
		Return(1, nil)
	mockStore.EXPECT().GetVerifiedReferralCountByReferrer(gomock.Any(), userID).
		Return(1, nil)

	result, err := processor.GetUserReferrals(ctx, accountID, campaignID, userID, GetUserReferralsRequest{Page: 1, Limit: 20})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(result.Referrals) != 1 {
		t.Errorf("expected 1 referral, got %d", len(result.Referrals))
	}
}
