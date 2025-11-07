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
	ErrCampaignNotFound = errors.New("campaign not found")
	ErrUnauthorized     = errors.New("unauthorized access to campaign")
	ErrInvalidDateRange = errors.New("invalid date range")
	ErrInvalidGranularity = errors.New("invalid granularity")
)

type AnalyticsProcessor struct {
	store  store.Store
	logger *observability.Logger
}

func New(store store.Store, logger *observability.Logger) AnalyticsProcessor {
	return AnalyticsProcessor{
		store:  store,
		logger: logger,
	}
}

// AnalyticsOverviewResponse represents the overview analytics response
type AnalyticsOverviewResponse struct {
	TotalSignups      int                         `json:"total_signups"`
	TotalVerified     int                         `json:"total_verified"`
	TotalReferrals    int                         `json:"total_referrals"`
	VerificationRate  float64                     `json:"verification_rate"`
	AverageReferrals  float64                     `json:"average_referrals"`
	ViralCoefficient  float64                     `json:"viral_coefficient"`
	TopReferrers      []store.TopReferrerResult   `json:"top_referrers"`
	SignupsOverTime   []store.TimeSeriesDataPoint `json:"signups_over_time"`
	ReferralSources   []store.SourceBreakdownResult `json:"referral_sources"`
}

// ConversionAnalyticsResponse represents conversion analytics response
type ConversionAnalyticsResponse struct {
	TotalSignups      int                        `json:"total_signups"`
	TotalVerified     int                        `json:"total_verified"`
	TotalConverted    int                        `json:"total_converted"`
	VerificationRate  float64                    `json:"verification_rate"`
	ConversionRate    float64                    `json:"conversion_rate"`
	FunnelData        []FunnelDataPoint          `json:"funnel_data"`
}

// FunnelDataPoint represents a single point in the conversion funnel
type FunnelDataPoint struct {
	Stage      string  `json:"stage"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// ReferralAnalyticsResponse represents referral analytics response
type ReferralAnalyticsResponse struct {
	TotalReferrals          int                           `json:"total_referrals"`
	VerifiedReferrals       int                           `json:"verified_referrals"`
	AverageReferralsPerUser float64                       `json:"average_referrals_per_user"`
	ViralCoefficient        float64                       `json:"viral_coefficient"`
	TopReferrers            []store.TopReferrerResult     `json:"top_referrers"`
	ReferralSources         []store.SourceBreakdownResult `json:"referral_sources"`
}

// SourceAnalyticsResponse represents source analytics response
type SourceAnalyticsResponse struct {
	Sources      []store.SourceBreakdownResult `json:"sources"`
	UTMCampaigns []map[string]interface{}      `json:"utm_campaigns"`
	UTMSources   []map[string]interface{}      `json:"utm_sources"`
}

// GetAnalyticsOverview retrieves high-level analytics for a campaign
func (p *AnalyticsProcessor) GetAnalyticsOverview(ctx context.Context, accountID, campaignID uuid.UUID) (AnalyticsOverviewResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	// Verify campaign belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return AnalyticsOverviewResponse{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return AnalyticsOverviewResponse{}, err
	}

	if campaign.AccountID != accountID {
		return AnalyticsOverviewResponse{}, ErrUnauthorized
	}

	// Get overview analytics
	overview, err := p.store.GetAnalyticsOverview(ctx, campaignID)
	if err != nil {
		p.logger.Error(ctx, "failed to get analytics overview", err)
		return AnalyticsOverviewResponse{}, err
	}

	// Get top referrers
	topReferrers, err := p.store.GetTopReferrers(ctx, campaignID, 10)
	if err != nil {
		p.logger.Error(ctx, "failed to get top referrers", err)
		return AnalyticsOverviewResponse{}, err
	}

	// Get time series data for last 30 days
	dateFrom := time.Now().AddDate(0, 0, -30)
	signupsOverTime, err := p.store.GetTimeSeriesAnalytics(ctx, campaignID, &dateFrom, nil, "day")
	if err != nil {
		p.logger.Error(ctx, "failed to get time series analytics", err)
		return AnalyticsOverviewResponse{}, err
	}

	// Get referral sources
	referralSources, err := p.store.GetReferralSourceBreakdown(ctx, campaignID, nil, nil)
	if err != nil {
		p.logger.Error(ctx, "failed to get referral sources", err)
		return AnalyticsOverviewResponse{}, err
	}

	response := AnalyticsOverviewResponse{
		TotalSignups:      overview.TotalSignups,
		TotalVerified:     overview.TotalVerified,
		TotalReferrals:    overview.TotalReferrals,
		VerificationRate:  overview.VerificationRate,
		AverageReferrals:  overview.AverageReferrals,
		ViralCoefficient:  overview.ViralCoefficient,
		TopReferrers:      topReferrers,
		SignupsOverTime:   signupsOverTime,
		ReferralSources:   referralSources,
	}

	return response, nil
}

// GetConversionAnalytics retrieves conversion funnel analytics
func (p *AnalyticsProcessor) GetConversionAnalytics(ctx context.Context, accountID, campaignID uuid.UUID, dateFrom, dateTo *time.Time) (ConversionAnalyticsResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	// Verify campaign belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ConversionAnalyticsResponse{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return ConversionAnalyticsResponse{}, err
	}

	if campaign.AccountID != accountID {
		return ConversionAnalyticsResponse{}, ErrUnauthorized
	}

	// Validate date range
	if dateFrom != nil && dateTo != nil && dateFrom.After(*dateTo) {
		return ConversionAnalyticsResponse{}, ErrInvalidDateRange
	}

	// Get conversion analytics
	conversionData, err := p.store.GetConversionAnalytics(ctx, campaignID, dateFrom, dateTo)
	if err != nil {
		p.logger.Error(ctx, "failed to get conversion analytics", err)
		return ConversionAnalyticsResponse{}, err
	}

	// Build funnel data
	funnelData := []FunnelDataPoint{
		{
			Stage:      "signup",
			Count:      conversionData.TotalSignups,
			Percentage: 100.0,
		},
		{
			Stage:      "verified",
			Count:      conversionData.TotalVerified,
			Percentage: conversionData.VerificationRate * 100,
		},
		{
			Stage:      "converted",
			Count:      conversionData.TotalConverted,
			Percentage: conversionData.ConversionRate * 100,
		},
	}

	response := ConversionAnalyticsResponse{
		TotalSignups:     conversionData.TotalSignups,
		TotalVerified:    conversionData.TotalVerified,
		TotalConverted:   conversionData.TotalConverted,
		VerificationRate: conversionData.VerificationRate,
		ConversionRate:   conversionData.ConversionRate,
		FunnelData:       funnelData,
	}

	return response, nil
}

// GetReferralAnalytics retrieves referral performance analytics
func (p *AnalyticsProcessor) GetReferralAnalytics(ctx context.Context, accountID, campaignID uuid.UUID, dateFrom, dateTo *time.Time) (ReferralAnalyticsResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	// Verify campaign belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ReferralAnalyticsResponse{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return ReferralAnalyticsResponse{}, err
	}

	if campaign.AccountID != accountID {
		return ReferralAnalyticsResponse{}, ErrUnauthorized
	}

	// Validate date range
	if dateFrom != nil && dateTo != nil && dateFrom.After(*dateTo) {
		return ReferralAnalyticsResponse{}, ErrInvalidDateRange
	}

	// Get referral analytics
	referralData, err := p.store.GetReferralAnalytics(ctx, campaignID, dateFrom, dateTo)
	if err != nil {
		p.logger.Error(ctx, "failed to get referral analytics", err)
		return ReferralAnalyticsResponse{}, err
	}

	// Get top referrers
	topReferrers, err := p.store.GetTopReferrers(ctx, campaignID, 10)
	if err != nil {
		p.logger.Error(ctx, "failed to get top referrers", err)
		return ReferralAnalyticsResponse{}, err
	}

	// Get referral sources
	referralSources, err := p.store.GetReferralSourceBreakdown(ctx, campaignID, dateFrom, dateTo)
	if err != nil {
		p.logger.Error(ctx, "failed to get referral sources", err)
		return ReferralAnalyticsResponse{}, err
	}

	response := ReferralAnalyticsResponse{
		TotalReferrals:          referralData.TotalReferrals,
		VerifiedReferrals:       referralData.VerifiedReferrals,
		AverageReferralsPerUser: referralData.AverageReferralsPerUser,
		ViralCoefficient:        referralData.ViralCoefficient,
		TopReferrers:            topReferrers,
		ReferralSources:         referralSources,
	}

	return response, nil
}

// GetTimeSeriesAnalytics retrieves time-series analytics data
func (p *AnalyticsProcessor) GetTimeSeriesAnalytics(ctx context.Context, accountID, campaignID uuid.UUID, dateFrom, dateTo *time.Time, granularity string) ([]store.TimeSeriesDataPoint, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "granularity", Value: granularity},
	)

	// Verify campaign belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return nil, err
	}

	if campaign.AccountID != accountID {
		return nil, ErrUnauthorized
	}

	// Validate date range
	if dateFrom != nil && dateTo != nil && dateFrom.After(*dateTo) {
		return nil, ErrInvalidDateRange
	}

	// Validate granularity
	validGranularities := map[string]bool{
		"hour":  true,
		"day":   true,
		"week":  true,
		"month": true,
	}
	if !validGranularities[granularity] {
		return nil, ErrInvalidGranularity
	}

	// Get time series data
	timeSeriesData, err := p.store.GetTimeSeriesAnalytics(ctx, campaignID, dateFrom, dateTo, granularity)
	if err != nil {
		p.logger.Error(ctx, "failed to get time series analytics", err)
		return nil, err
	}

	return timeSeriesData, nil
}

// GetSourceAnalytics retrieves traffic source breakdown
func (p *AnalyticsProcessor) GetSourceAnalytics(ctx context.Context, accountID, campaignID uuid.UUID, dateFrom, dateTo *time.Time) (SourceAnalyticsResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	// Verify campaign belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return SourceAnalyticsResponse{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return SourceAnalyticsResponse{}, err
	}

	if campaign.AccountID != accountID {
		return SourceAnalyticsResponse{}, ErrUnauthorized
	}

	// Validate date range
	if dateFrom != nil && dateTo != nil && dateFrom.After(*dateTo) {
		return SourceAnalyticsResponse{}, ErrInvalidDateRange
	}

	// Get signup sources
	sources, err := p.store.GetSignupSourceBreakdown(ctx, campaignID, dateFrom, dateTo)
	if err != nil {
		p.logger.Error(ctx, "failed to get signup sources", err)
		return SourceAnalyticsResponse{}, err
	}

	// Get UTM campaign breakdown
	utmCampaigns, err := p.store.GetUTMCampaignBreakdown(ctx, campaignID, dateFrom, dateTo)
	if err != nil {
		p.logger.Error(ctx, "failed to get utm campaigns", err)
		return SourceAnalyticsResponse{}, err
	}

	// Get UTM source breakdown
	utmSources, err := p.store.GetUTMSourceBreakdown(ctx, campaignID, dateFrom, dateTo)
	if err != nil {
		p.logger.Error(ctx, "failed to get utm sources", err)
		return SourceAnalyticsResponse{}, err
	}

	response := SourceAnalyticsResponse{
		Sources:      sources,
		UTMCampaigns: utmCampaigns,
		UTMSources:   utmSources,
	}

	return response, nil
}

// GetFunnelAnalytics retrieves conversion funnel visualization data
func (p *AnalyticsProcessor) GetFunnelAnalytics(ctx context.Context, accountID, campaignID uuid.UUID, dateFrom, dateTo *time.Time) (map[string]interface{}, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	// Verify campaign belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return nil, err
	}

	if campaign.AccountID != accountID {
		return nil, ErrUnauthorized
	}

	// Validate date range
	if dateFrom != nil && dateTo != nil && dateFrom.After(*dateTo) {
		return nil, ErrInvalidDateRange
	}

	// Get funnel analytics
	funnelSteps, err := p.store.GetFunnelAnalytics(ctx, campaignID, dateFrom, dateTo)
	if err != nil {
		p.logger.Error(ctx, "failed to get funnel analytics", err)
		return nil, err
	}

	response := map[string]interface{}{
		"funnel_steps": funnelSteps,
	}

	return response, nil
}
