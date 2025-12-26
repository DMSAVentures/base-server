package processor

//go:generate go run go.uber.org/mock/mockgen@latest -source=processor.go -destination=mocks_test.go -package=processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AnalyticsStore defines the database operations required by AnalyticsProcessor
type AnalyticsStore interface {
	GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (store.Campaign, error)
	GetAnalyticsOverview(ctx context.Context, campaignID uuid.UUID) (store.AnalyticsOverviewResult, error)
	GetTopReferrers(ctx context.Context, campaignID uuid.UUID, limit int) ([]store.TopReferrerResult, error)
	GetSignupsOverTime(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo time.Time, period string) ([]store.SignupsOverTimeDataPoint, error)
	GetSignupsBySource(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo time.Time, period string) ([]store.SignupsBySourceDataPoint, error)
	GetReferralSourceBreakdown(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) ([]store.SourceBreakdownResult, error)
	GetConversionAnalytics(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) (store.ConversionAnalyticsResult, error)
	GetReferralAnalytics(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) (store.ReferralAnalyticsResult, error)
	GetSignupSourceBreakdown(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) ([]store.SourceBreakdownResult, error)
	GetUTMCampaignBreakdown(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) ([]map[string]interface{}, error)
	GetUTMSourceBreakdown(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) ([]map[string]interface{}, error)
	GetFunnelAnalytics(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) ([]store.FunnelStepResult, error)
}

var (
	ErrCampaignNotFound = errors.New("campaign not found")
	ErrUnauthorized     = errors.New("unauthorized access to campaign")
	ErrInvalidDateRange = errors.New("invalid date range")
	ErrInvalidGranularity = errors.New("invalid granularity")
)

type AnalyticsProcessor struct {
	store  AnalyticsStore
	logger *observability.Logger
}

func New(store AnalyticsStore, logger *observability.Logger) AnalyticsProcessor {
	return AnalyticsProcessor{
		store:  store,
		logger: logger,
	}
}

// AnalyticsOverviewResponse represents the overview analytics response
type AnalyticsOverviewResponse struct {
	TotalSignups      int                              `json:"total_signups"`
	TotalVerified     int                              `json:"total_verified"`
	TotalReferrals    int                              `json:"total_referrals"`
	VerificationRate  float64                          `json:"verification_rate"`
	AverageReferrals  float64                          `json:"average_referrals"`
	ViralCoefficient  float64                          `json:"viral_coefficient"`
	TopReferrers      []store.TopReferrerResult        `json:"top_referrers"`
	SignupsOverTime   []store.SignupsOverTimeDataPoint `json:"signups_over_time"`
	ReferralSources   []store.SourceBreakdownResult    `json:"referral_sources"`
}

// SignupsOverTimeResponse represents the response for signups over time chart
type SignupsOverTimeResponse struct {
	Data   []store.SignupsOverTimeDataPoint `json:"data"`
	Total  int                              `json:"total"`
	Period string                           `json:"period"`
}

// SignupsBySourceResponse represents the response for signups by source chart
type SignupsBySourceResponse struct {
	Data    []store.SignupsBySourceDataPoint `json:"data"`
	Sources []string                         `json:"sources"`
	Total   int                              `json:"total"`
	Period  string                           `json:"period"`
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
	dateTo := time.Now()
	signupsOverTime, err := p.store.GetSignupsOverTime(ctx, campaignID, dateFrom, dateTo, "day")
	if err != nil {
		p.logger.Error(ctx, "failed to get signups over time", err)
		return AnalyticsOverviewResponse{}, err
	}
	// Fill gaps for missing dates
	signupsOverTime = fillGaps(signupsOverTime, dateFrom, dateTo, "day")

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

// GetSignupsOverTime retrieves signup counts over time for charts
func (p *AnalyticsProcessor) GetSignupsOverTime(ctx context.Context, accountID, campaignID uuid.UUID, dateFrom, dateTo *time.Time, period string) (SignupsOverTimeResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "period", Value: period},
	)

	// Verify campaign belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return SignupsOverTimeResponse{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return SignupsOverTimeResponse{}, err
	}

	if campaign.AccountID != accountID {
		return SignupsOverTimeResponse{}, ErrUnauthorized
	}

	// Set default date range if not provided (use UTC)
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -30) // Default: last 30 days
	to := now
	if dateFrom != nil {
		from = dateFrom.UTC()
	}
	if dateTo != nil {
		to = dateTo.UTC()
	}

	// Validate date range
	if from.After(to) {
		return SignupsOverTimeResponse{}, ErrInvalidDateRange
	}

	// Validate period
	validPeriods := map[string]bool{
		"hour":  true,
		"day":   true,
		"week":  true,
		"month": true,
	}
	if !validPeriods[period] {
		period = "day"
	}

	// Get signups over time data
	data, err := p.store.GetSignupsOverTime(ctx, campaignID, from, to, period)
	if err != nil {
		p.logger.Error(ctx, "failed to get signups over time", err)
		return SignupsOverTimeResponse{}, err
	}

	// Fill gaps for missing time periods
	data = fillGaps(data, from, to, period)

	// Calculate total
	total := 0
	for _, d := range data {
		total += d.Count
	}

	return SignupsOverTimeResponse{
		Data:   data,
		Total:  total,
		Period: period,
	}, nil
}

// GetSignupsBySource retrieves signup counts over time broken down by UTM source
func (p *AnalyticsProcessor) GetSignupsBySource(ctx context.Context, accountID, campaignID uuid.UUID, dateFrom, dateTo *time.Time, period string) (SignupsBySourceResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "period", Value: period},
	)

	// Verify campaign belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return SignupsBySourceResponse{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return SignupsBySourceResponse{}, err
	}

	if campaign.AccountID != accountID {
		return SignupsBySourceResponse{}, ErrUnauthorized
	}

	// Set default date range if not provided (use UTC)
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -30) // Default: last 30 days
	to := now
	if dateFrom != nil {
		from = dateFrom.UTC()
	}
	if dateTo != nil {
		to = dateTo.UTC()
	}

	// Validate date range
	if from.After(to) {
		return SignupsBySourceResponse{}, ErrInvalidDateRange
	}

	// Validate period
	validPeriods := map[string]bool{
		"hour":  true,
		"day":   true,
		"week":  true,
		"month": true,
	}
	if !validPeriods[period] {
		period = "day"
	}

	// Get signups by source data
	data, err := p.store.GetSignupsBySource(ctx, campaignID, from, to, period)
	if err != nil {
		p.logger.Error(ctx, "failed to get signups by source", err)
		return SignupsBySourceResponse{}, err
	}

	// Extract unique sources and fill gaps
	data, sources := fillGapsBySource(data, from, to, period)

	// Calculate total
	total := 0
	for _, d := range data {
		total += d.Count
	}

	return SignupsBySourceResponse{
		Data:    data,
		Sources: sources,
		Total:   total,
		Period:  period,
	}, nil
}

// fillGapsBySource fills in missing time periods with zero counts for each source
func fillGapsBySource(data []store.SignupsBySourceDataPoint, from, to time.Time, period string) ([]store.SignupsBySourceDataPoint, []string) {
	// Normalize to UTC to ensure consistent comparison
	from = from.UTC()
	to = to.UTC()

	// Extract unique sources
	sourceSet := make(map[string]bool)
	for _, d := range data {
		if d.UTMSource != nil {
			sourceSet[*d.UTMSource] = true
		} else {
			sourceSet[""] = true // empty string for null utm_source
		}
	}

	// Convert to sorted slice
	sources := make([]string, 0, len(sourceSet))
	for s := range sourceSet {
		sources = append(sources, s)
	}
	// Sort sources with empty string (null) last
	for i := 0; i < len(sources)-1; i++ {
		for j := i + 1; j < len(sources); j++ {
			// Empty string should come last
			if sources[i] == "" || (sources[j] != "" && sources[i] > sources[j]) {
				sources[i], sources[j] = sources[j], sources[i]
			}
		}
	}

	if len(data) == 0 {
		// Return empty data with empty sources
		return []store.SignupsBySourceDataPoint{}, []string{}
	}

	// Create a map for quick lookup: key = "unix_timestamp:source"
	dataMap := make(map[string]int)
	for _, d := range data {
		key := truncateTime(d.Date.UTC(), period).Unix()
		source := ""
		if d.UTMSource != nil {
			source = *d.UTMSource
		}
		mapKey := formatSourceKey(key, source)
		dataMap[mapKey] = d.Count
	}

	// Generate complete list of periods for each source
	var result []store.SignupsBySourceDataPoint
	current := truncateTime(from, period)
	end := truncateTime(to, period)

	for !current.After(end) {
		key := current.Unix()
		for _, source := range sources {
			mapKey := formatSourceKey(key, source)
			count := dataMap[mapKey]
			var utmSource *string
			if source != "" {
				s := source
				utmSource = &s
			}
			result = append(result, store.SignupsBySourceDataPoint{
				Date:      current,
				UTMSource: utmSource,
				Count:     count,
			})
		}
		current = advanceTime(current, period)
	}

	return result, sources
}

// formatSourceKey creates a unique key for date+source combination
func formatSourceKey(unixTime int64, source string) string {
	return fmt.Sprintf("%d:%s", unixTime, source)
}

// fillGaps fills in missing time periods with zero counts
func fillGaps(data []store.SignupsOverTimeDataPoint, from, to time.Time, period string) []store.SignupsOverTimeDataPoint {
	// Normalize to UTC to ensure consistent comparison
	from = from.UTC()
	to = to.UTC()

	if len(data) == 0 {
		// Generate all periods with zero counts
		return generateEmptyPeriods(from, to, period)
	}

	// Create a map for quick lookup using Unix timestamp (timezone-agnostic)
	dataMap := make(map[int64]int)
	for _, d := range data {
		// Normalize DB date to UTC and truncate, then use Unix timestamp as key
		key := truncateTime(d.Date.UTC(), period).Unix()
		dataMap[key] = d.Count
	}

	// Generate complete list of periods
	var result []store.SignupsOverTimeDataPoint
	current := truncateTime(from, period)
	end := truncateTime(to, period)

	for !current.After(end) {
		key := current.Unix()
		count := dataMap[key]
		result = append(result, store.SignupsOverTimeDataPoint{
			Date:  current,
			Count: count,
		})
		current = advanceTime(current, period)
	}

	return result
}

// generateEmptyPeriods generates a list of periods with zero counts
func generateEmptyPeriods(from, to time.Time, period string) []store.SignupsOverTimeDataPoint {
	var result []store.SignupsOverTimeDataPoint
	current := truncateTime(from.UTC(), period)
	end := truncateTime(to.UTC(), period)

	for !current.After(end) {
		result = append(result, store.SignupsOverTimeDataPoint{
			Date:  current,
			Count: 0,
		})
		current = advanceTime(current, period)
	}

	return result
}

// truncateTime truncates a time to the start of the given period (in UTC)
func truncateTime(t time.Time, period string) time.Time {
	t = t.UTC()
	switch period {
	case "hour":
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC)
	case "day":
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	case "week":
		// Truncate to Monday of the week
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return time.Date(t.Year(), t.Month(), t.Day()-(weekday-1), 0, 0, 0, 0, time.UTC)
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
}

// advanceTime advances the time by one period
func advanceTime(t time.Time, period string) time.Time {
	switch period {
	case "hour":
		return t.Add(time.Hour)
	case "day":
		return t.AddDate(0, 0, 1)
	case "week":
		return t.AddDate(0, 0, 7)
	case "month":
		return t.AddDate(0, 1, 0)
	default:
		return t.AddDate(0, 0, 1)
	}
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
