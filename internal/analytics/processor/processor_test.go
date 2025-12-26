package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestGetAnalyticsOverview_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
	}

	overview := store.AnalyticsOverviewResult{
		TotalSignups:     100,
		TotalVerified:    80,
		TotalReferrals:   30,
		VerificationRate: 0.8,
		AverageReferrals: 0.3,
		ViralCoefficient: 0.5,
	}

	topReferrers := []store.TopReferrerResult{
		{UserID: uuid.New(), Email: "user1@example.com", Name: "User One", ReferralCount: 10},
		{UserID: uuid.New(), Email: "user2@example.com", Name: "User Two", ReferralCount: 5},
	}

	signupsOverTime := []store.SignupsOverTimeDataPoint{
		{Date: time.Now().AddDate(0, 0, -1), Count: 5},
		{Date: time.Now(), Count: 10},
	}

	referralSources := []store.SourceBreakdownResult{
		{Source: "direct", Count: 50},
		{Source: "social", Count: 30},
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)
	mockStore.EXPECT().GetAnalyticsOverview(gomock.Any(), campaignID).Return(overview, nil)
	mockStore.EXPECT().GetTopReferrers(gomock.Any(), campaignID, 10).Return(topReferrers, nil)
	mockStore.EXPECT().GetSignupsOverTime(gomock.Any(), campaignID, gomock.Any(), gomock.Any(), "day").Return(signupsOverTime, nil)
	mockStore.EXPECT().GetReferralSourceBreakdown(gomock.Any(), campaignID, nil, nil).Return(referralSources, nil)

	result, err := processor.GetAnalyticsOverview(ctx, accountID, campaignID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.TotalSignups != 100 {
		t.Errorf("expected TotalSignups 100, got %d", result.TotalSignups)
	}
	if result.TotalVerified != 80 {
		t.Errorf("expected TotalVerified 80, got %d", result.TotalVerified)
	}
	if len(result.TopReferrers) != 2 {
		t.Errorf("expected 2 top referrers, got %d", len(result.TopReferrers))
	}
}

func TestGetAnalyticsOverview_CampaignNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{}, store.ErrNotFound)

	_, err := processor.GetAnalyticsOverview(ctx, accountID, campaignID)

	if !errors.Is(err, ErrCampaignNotFound) {
		t.Errorf("expected ErrCampaignNotFound, got %v", err)
	}
}

func TestGetAnalyticsOverview_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	otherAccountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: otherAccountID,
		Name:      "Test Campaign",
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)

	_, err := processor.GetAnalyticsOverview(ctx, accountID, campaignID)

	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestGetConversionAnalytics_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
	}

	conversionData := store.ConversionAnalyticsResult{
		TotalSignups:     100,
		TotalVerified:    80,
		TotalConverted:   50,
		VerificationRate: 0.8,
		ConversionRate:   0.5,
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)
	mockStore.EXPECT().GetConversionAnalytics(gomock.Any(), campaignID, nil, nil).Return(conversionData, nil)

	result, err := processor.GetConversionAnalytics(ctx, accountID, campaignID, nil, nil)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.TotalSignups != 100 {
		t.Errorf("expected TotalSignups 100, got %d", result.TotalSignups)
	}
	if result.TotalConverted != 50 {
		t.Errorf("expected TotalConverted 50, got %d", result.TotalConverted)
	}
	if len(result.FunnelData) != 3 {
		t.Errorf("expected 3 funnel steps, got %d", len(result.FunnelData))
	}
}

func TestGetConversionAnalytics_InvalidDateRange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
	}

	dateFrom := time.Now()
	dateTo := time.Now().AddDate(0, 0, -7)

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)

	_, err := processor.GetConversionAnalytics(ctx, accountID, campaignID, &dateFrom, &dateTo)

	if !errors.Is(err, ErrInvalidDateRange) {
		t.Errorf("expected ErrInvalidDateRange, got %v", err)
	}
}

func TestGetConversionAnalytics_CampaignNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{}, store.ErrNotFound)

	_, err := processor.GetConversionAnalytics(ctx, accountID, campaignID, nil, nil)

	if !errors.Is(err, ErrCampaignNotFound) {
		t.Errorf("expected ErrCampaignNotFound, got %v", err)
	}
}

func TestGetReferralAnalytics_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
	}

	referralData := store.ReferralAnalyticsResult{
		TotalReferrals:          100,
		VerifiedReferrals:       80,
		AverageReferralsPerUser: 2.5,
		ViralCoefficient:        1.2,
	}

	topReferrers := []store.TopReferrerResult{
		{UserID: uuid.New(), Email: "user1@example.com", Name: "User One", ReferralCount: 10},
	}

	referralSources := []store.SourceBreakdownResult{
		{Source: "direct", Count: 50},
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)
	mockStore.EXPECT().GetReferralAnalytics(gomock.Any(), campaignID, nil, nil).Return(referralData, nil)
	mockStore.EXPECT().GetTopReferrers(gomock.Any(), campaignID, 10).Return(topReferrers, nil)
	mockStore.EXPECT().GetReferralSourceBreakdown(gomock.Any(), campaignID, nil, nil).Return(referralSources, nil)

	result, err := processor.GetReferralAnalytics(ctx, accountID, campaignID, nil, nil)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.TotalReferrals != 100 {
		t.Errorf("expected TotalReferrals 100, got %d", result.TotalReferrals)
	}
	if result.VerifiedReferrals != 80 {
		t.Errorf("expected VerifiedReferrals 80, got %d", result.VerifiedReferrals)
	}
	if len(result.TopReferrers) != 1 {
		t.Errorf("expected 1 top referrer, got %d", len(result.TopReferrers))
	}
}

func TestGetReferralAnalytics_InvalidDateRange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
	}

	dateFrom := time.Now()
	dateTo := time.Now().AddDate(0, 0, -7)

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)

	_, err := processor.GetReferralAnalytics(ctx, accountID, campaignID, &dateFrom, &dateTo)

	if !errors.Is(err, ErrInvalidDateRange) {
		t.Errorf("expected ErrInvalidDateRange, got %v", err)
	}
}

func TestGetReferralAnalytics_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	otherAccountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: otherAccountID,
		Name:      "Test Campaign",
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)

	_, err := processor.GetReferralAnalytics(ctx, accountID, campaignID, nil, nil)

	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestGetSignupsOverTime_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
	}

	signupsData := []store.SignupsOverTimeDataPoint{
		{Date: time.Now().AddDate(0, 0, -1).UTC(), Count: 5},
		{Date: time.Now().UTC(), Count: 10},
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)
	mockStore.EXPECT().GetSignupsOverTime(gomock.Any(), campaignID, gomock.Any(), gomock.Any(), "day").Return(signupsData, nil)

	result, err := processor.GetSignupsOverTime(ctx, accountID, campaignID, nil, nil, "day")

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Period != "day" {
		t.Errorf("expected period 'day', got %s", result.Period)
	}
}

func TestGetSignupsOverTime_InvalidPeriodDefaultsToDay(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
	}

	signupsData := []store.SignupsOverTimeDataPoint{}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)
	mockStore.EXPECT().GetSignupsOverTime(gomock.Any(), campaignID, gomock.Any(), gomock.Any(), "day").Return(signupsData, nil)

	result, err := processor.GetSignupsOverTime(ctx, accountID, campaignID, nil, nil, "invalid")

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Period != "day" {
		t.Errorf("expected period to default to 'day', got %s", result.Period)
	}
}

func TestGetSignupsOverTime_InvalidDateRange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
	}

	dateFrom := time.Now()
	dateTo := time.Now().AddDate(0, 0, -7)

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)

	_, err := processor.GetSignupsOverTime(ctx, accountID, campaignID, &dateFrom, &dateTo, "day")

	if !errors.Is(err, ErrInvalidDateRange) {
		t.Errorf("expected ErrInvalidDateRange, got %v", err)
	}
}

func TestGetSignupsOverTime_CampaignNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{}, store.ErrNotFound)

	_, err := processor.GetSignupsOverTime(ctx, accountID, campaignID, nil, nil, "day")

	if !errors.Is(err, ErrCampaignNotFound) {
		t.Errorf("expected ErrCampaignNotFound, got %v", err)
	}
}

func TestGetSignupsBySource_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
	}

	source1 := "google"
	source2 := "facebook"
	signupsData := []store.SignupsBySourceDataPoint{
		{Date: time.Now().AddDate(0, 0, -1).UTC(), UTMSource: &source1, Count: 5},
		{Date: time.Now().UTC(), UTMSource: &source2, Count: 10},
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)
	mockStore.EXPECT().GetSignupsBySource(gomock.Any(), campaignID, gomock.Any(), gomock.Any(), "day").Return(signupsData, nil)

	result, err := processor.GetSignupsBySource(ctx, accountID, campaignID, nil, nil, "day")

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Period != "day" {
		t.Errorf("expected period 'day', got %s", result.Period)
	}
}

func TestGetSignupsBySource_CampaignNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{}, store.ErrNotFound)

	_, err := processor.GetSignupsBySource(ctx, accountID, campaignID, nil, nil, "day")

	if !errors.Is(err, ErrCampaignNotFound) {
		t.Errorf("expected ErrCampaignNotFound, got %v", err)
	}
}

func TestGetSignupsBySource_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	otherAccountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: otherAccountID,
		Name:      "Test Campaign",
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)

	_, err := processor.GetSignupsBySource(ctx, accountID, campaignID, nil, nil, "day")

	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestGetSourceAnalytics_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
	}

	sources := []store.SourceBreakdownResult{
		{Source: "direct", Count: 50},
		{Source: "social", Count: 30},
	}

	utmCampaigns := []map[string]interface{}{
		{"utm_campaign": "summer_sale", "count": 25},
	}

	utmSources := []map[string]interface{}{
		{"utm_source": "google", "count": 40},
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)
	mockStore.EXPECT().GetSignupSourceBreakdown(gomock.Any(), campaignID, nil, nil).Return(sources, nil)
	mockStore.EXPECT().GetUTMCampaignBreakdown(gomock.Any(), campaignID, nil, nil).Return(utmCampaigns, nil)
	mockStore.EXPECT().GetUTMSourceBreakdown(gomock.Any(), campaignID, nil, nil).Return(utmSources, nil)

	result, err := processor.GetSourceAnalytics(ctx, accountID, campaignID, nil, nil)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(result.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(result.Sources))
	}
	if len(result.UTMCampaigns) != 1 {
		t.Errorf("expected 1 UTM campaign, got %d", len(result.UTMCampaigns))
	}
	if len(result.UTMSources) != 1 {
		t.Errorf("expected 1 UTM source, got %d", len(result.UTMSources))
	}
}

func TestGetSourceAnalytics_CampaignNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{}, store.ErrNotFound)

	_, err := processor.GetSourceAnalytics(ctx, accountID, campaignID, nil, nil)

	if !errors.Is(err, ErrCampaignNotFound) {
		t.Errorf("expected ErrCampaignNotFound, got %v", err)
	}
}

func TestGetSourceAnalytics_InvalidDateRange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
	}

	dateFrom := time.Now()
	dateTo := time.Now().AddDate(0, 0, -7)

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)

	_, err := processor.GetSourceAnalytics(ctx, accountID, campaignID, &dateFrom, &dateTo)

	if !errors.Is(err, ErrInvalidDateRange) {
		t.Errorf("expected ErrInvalidDateRange, got %v", err)
	}
}

func TestGetFunnelAnalytics_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
	}

	funnelSteps := []store.FunnelStepResult{
		{Step: "signed_up", Count: 100, Percentage: 100, DropOffRate: 0},
		{Step: "verified", Count: 80, Percentage: 80, DropOffRate: 20},
		{Step: "converted", Count: 50, Percentage: 50, DropOffRate: 50},
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)
	mockStore.EXPECT().GetFunnelAnalytics(gomock.Any(), campaignID, nil, nil).Return(funnelSteps, nil)

	result, err := processor.GetFunnelAnalytics(ctx, accountID, campaignID, nil, nil)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Error("expected result to not be nil")
	}
	if result["funnel_steps"] == nil {
		t.Error("expected funnel_steps in result")
	}
}

func TestGetFunnelAnalytics_CampaignNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(store.Campaign{}, store.ErrNotFound)

	_, err := processor.GetFunnelAnalytics(ctx, accountID, campaignID, nil, nil)

	if !errors.Is(err, ErrCampaignNotFound) {
		t.Errorf("expected ErrCampaignNotFound, got %v", err)
	}
}

func TestGetFunnelAnalytics_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	otherAccountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: otherAccountID,
		Name:      "Test Campaign",
	}

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)

	_, err := processor.GetFunnelAnalytics(ctx, accountID, campaignID, nil, nil)

	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestGetFunnelAnalytics_InvalidDateRange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
	}

	dateFrom := time.Now()
	dateTo := time.Now().AddDate(0, 0, -7)

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)

	_, err := processor.GetFunnelAnalytics(ctx, accountID, campaignID, &dateFrom, &dateTo)

	if !errors.Is(err, ErrInvalidDateRange) {
		t.Errorf("expected ErrInvalidDateRange, got %v", err)
	}
}

// Test helper functions
func TestTruncateTime(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		period   string
		expected time.Time
	}{
		{
			name:     "truncate to hour",
			input:    time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC),
			period:   "hour",
			expected: time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC),
		},
		{
			name:     "truncate to day",
			input:    time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC),
			period:   "day",
			expected: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "truncate to week (Monday)",
			input:    time.Date(2024, 1, 17, 14, 30, 45, 0, time.UTC), // Wednesday
			period:   "week",
			expected: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), // Monday
		},
		{
			name:     "truncate to month",
			input:    time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC),
			period:   "month",
			expected: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "invalid period defaults to day",
			input:    time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC),
			period:   "invalid",
			expected: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateTime(tt.input, tt.period)
			if !result.Equal(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAdvanceTime(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		period   string
		expected time.Time
	}{
		{
			name:     "advance by hour",
			input:    time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC),
			period:   "hour",
			expected: time.Date(2024, 1, 15, 15, 0, 0, 0, time.UTC),
		},
		{
			name:     "advance by day",
			input:    time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			period:   "day",
			expected: time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "advance by week",
			input:    time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			period:   "week",
			expected: time.Date(2024, 1, 22, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "advance by month",
			input:    time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			period:   "month",
			expected: time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "invalid period defaults to day",
			input:    time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			period:   "invalid",
			expected: time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := advanceTime(tt.input, tt.period)
			if !result.Equal(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFillGaps(t *testing.T) {
	t.Run("fills gaps with zero counts", func(t *testing.T) {
		from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

		data := []store.SignupsOverTimeDataPoint{
			{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Count: 5},
			{Date: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Count: 10},
		}

		result := fillGaps(data, from, to, "day")

		if len(result) != 3 {
			t.Errorf("expected 3 data points, got %d", len(result))
		}

		// Check that the gap (Jan 2) has count 0
		if result[1].Count != 0 {
			t.Errorf("expected gap to have count 0, got %d", result[1].Count)
		}
	})

	t.Run("empty data returns all zero counts", func(t *testing.T) {
		from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

		data := []store.SignupsOverTimeDataPoint{}

		result := fillGaps(data, from, to, "day")

		if len(result) != 3 {
			t.Errorf("expected 3 data points, got %d", len(result))
		}

		for _, dp := range result {
			if dp.Count != 0 {
				t.Errorf("expected all counts to be 0, got %d", dp.Count)
			}
		}
	})
}

func TestFormatSourceKey(t *testing.T) {
	tests := []struct {
		name      string
		unixTime  int64
		source    string
		expected  string
	}{
		{
			name:     "with source",
			unixTime: 1704067200,
			source:   "google",
			expected: "1704067200:google",
		},
		{
			name:     "empty source",
			unixTime: 1704067200,
			source:   "",
			expected: "1704067200:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSourceKey(tt.unixTime, tt.source)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// Test store error propagation
func TestGetAnalyticsOverview_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
	}

	storeErr := errors.New("database error")

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)
	mockStore.EXPECT().GetAnalyticsOverview(gomock.Any(), campaignID).Return(store.AnalyticsOverviewResult{}, storeErr)

	_, err := processor.GetAnalyticsOverview(ctx, accountID, campaignID)

	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGetConversionAnalytics_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAnalyticsStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	campaign := store.Campaign{
		ID:        campaignID,
		AccountID: accountID,
		Name:      "Test Campaign",
	}

	storeErr := errors.New("database error")

	mockStore.EXPECT().GetCampaignByID(gomock.Any(), campaignID).Return(campaign, nil)
	mockStore.EXPECT().GetConversionAnalytics(gomock.Any(), campaignID, nil, nil).Return(store.ConversionAnalyticsResult{}, storeErr)

	_, err := processor.GetConversionAnalytics(ctx, accountID, campaignID, nil, nil)

	if err == nil {
		t.Error("expected error, got nil")
	}
}
