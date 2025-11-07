package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TopReferrerResult represents a top referrer in analytics
type TopReferrerResult struct {
	UserID        uuid.UUID `db:"user_id" json:"user_id"`
	Email         string    `db:"email" json:"email"`
	Name          string    `db:"name" json:"name"`
	ReferralCount int       `db:"referral_count" json:"referral_count"`
}

// SourceBreakdownResult represents traffic source breakdown
type SourceBreakdownResult struct {
	Source string `db:"source" json:"source"`
	Count  int    `db:"count" json:"count"`
}

// TimeSeriesDataPoint represents a point in time series
type TimeSeriesDataPoint struct {
	Date          time.Time `db:"date" json:"date"`
	Signups       int       `db:"signups" json:"signups"`
	Verified      int       `db:"verified" json:"verified"`
	Referrals     int       `db:"referrals" json:"referrals"`
	EmailsSent    int       `db:"emails_sent" json:"emails_sent"`
	EmailsOpened  int       `db:"emails_opened" json:"emails_opened"`
	EmailsClicked int       `db:"emails_clicked" json:"emails_clicked"`
}

// FunnelStepResult represents a funnel step
type FunnelStepResult struct {
	Step        string  `db:"step" json:"step"`
	Count       int     `db:"count" json:"count"`
	Percentage  float64 `db:"percentage" json:"percentage"`
	DropOffRate float64 `db:"drop_off_rate" json:"drop_off_rate"`
}

// AnalyticsOverviewResult represents the overview analytics
type AnalyticsOverviewResult struct {
	TotalSignups      int     `db:"total_signups" json:"total_signups"`
	TotalVerified     int     `db:"total_verified" json:"total_verified"`
	TotalReferrals    int     `db:"total_referrals" json:"total_referrals"`
	VerificationRate  float64 `json:"verification_rate"`
	AverageReferrals  float64 `json:"average_referrals"`
	ViralCoefficient  float64 `json:"viral_coefficient"`
}

// ConversionAnalyticsResult represents conversion funnel analytics
type ConversionAnalyticsResult struct {
	TotalSignups      int     `json:"total_signups"`
	TotalVerified     int     `json:"total_verified"`
	TotalConverted    int     `json:"total_converted"`
	VerificationRate  float64 `json:"verification_rate"`
	ConversionRate    float64 `json:"conversion_rate"`
}

// ReferralAnalyticsResult represents referral performance analytics
type ReferralAnalyticsResult struct {
	TotalReferrals           int     `json:"total_referrals"`
	VerifiedReferrals        int     `json:"verified_referrals"`
	AverageReferralsPerUser  float64 `json:"average_referrals_per_user"`
	ViralCoefficient         float64 `json:"viral_coefficient"`
}

const sqlGetAnalyticsOverview = `
SELECT
    COALESCE(SUM(new_signups), 0)::int as total_signups,
    COALESCE(SUM(new_verified), 0)::int as total_verified,
    COALESCE(SUM(new_referrals), 0)::int as total_referrals
FROM campaign_analytics
WHERE campaign_id = $1
`

// GetAnalyticsOverview retrieves overview analytics for a campaign
func (s *Store) GetAnalyticsOverview(ctx context.Context, campaignID uuid.UUID) (AnalyticsOverviewResult, error) {
	var result AnalyticsOverviewResult
	err := s.db.GetContext(ctx, &result, sqlGetAnalyticsOverview, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to get analytics overview", err)
		return AnalyticsOverviewResult{}, fmt.Errorf("failed to get analytics overview: %w", err)
	}

	// Calculate rates
	if result.TotalSignups > 0 {
		result.VerificationRate = float64(result.TotalVerified) / float64(result.TotalSignups)
		result.AverageReferrals = float64(result.TotalReferrals) / float64(result.TotalSignups)
		// Viral coefficient is average referrals that convert (verified)
		if result.TotalReferrals > 0 {
			result.ViralCoefficient = float64(result.TotalVerified) / float64(result.TotalSignups)
		}
	}

	return result, nil
}

const sqlGetTopReferrers = `
SELECT
    wu.id as user_id,
    wu.email,
    COALESCE(wu.first_name || ' ' || wu.last_name, wu.email) as name,
    wu.verified_referral_count as referral_count
FROM waitlist_users wu
WHERE wu.campaign_id = $1
    AND wu.deleted_at IS NULL
    AND wu.verified_referral_count > 0
ORDER BY wu.verified_referral_count DESC
LIMIT $2
`

// GetTopReferrers retrieves top referrers for a campaign
func (s *Store) GetTopReferrers(ctx context.Context, campaignID uuid.UUID, limit int) ([]TopReferrerResult, error) {
	var results []TopReferrerResult
	err := s.db.SelectContext(ctx, &results, sqlGetTopReferrers, campaignID, limit)
	if err != nil {
		s.logger.Error(ctx, "failed to get top referrers", err)
		return nil, fmt.Errorf("failed to get top referrers: %w", err)
	}
	return results, nil
}

const sqlGetConversionAnalytics = `
SELECT
    COALESCE(COUNT(*), 0)::int as total_signups,
    COALESCE(COUNT(*) FILTER (WHERE email_verified = true), 0)::int as total_verified,
    COALESCE(COUNT(*) FILTER (WHERE status = 'converted'), 0)::int as total_converted
FROM waitlist_users
WHERE campaign_id = $1
    AND deleted_at IS NULL
    AND ($2::timestamptz IS NULL OR created_at >= $2)
    AND ($3::timestamptz IS NULL OR created_at <= $3)
`

// GetConversionAnalytics retrieves conversion funnel analytics
func (s *Store) GetConversionAnalytics(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) (ConversionAnalyticsResult, error) {
	var result ConversionAnalyticsResult
	err := s.db.GetContext(ctx, &result, sqlGetConversionAnalytics, campaignID, dateFrom, dateTo)
	if err != nil {
		s.logger.Error(ctx, "failed to get conversion analytics", err)
		return ConversionAnalyticsResult{}, fmt.Errorf("failed to get conversion analytics: %w", err)
	}

	// Calculate rates
	if result.TotalSignups > 0 {
		result.VerificationRate = float64(result.TotalVerified) / float64(result.TotalSignups)
		result.ConversionRate = float64(result.TotalConverted) / float64(result.TotalSignups)
	}

	return result, nil
}

const sqlGetReferralAnalytics = `
SELECT
    COALESCE(COUNT(*), 0)::int as total_referrals,
    COALESCE(COUNT(*) FILTER (WHERE status = 'verified'), 0)::int as verified_referrals
FROM referrals
WHERE campaign_id = $1
    AND ($2::timestamptz IS NULL OR created_at >= $2)
    AND ($3::timestamptz IS NULL OR created_at <= $3)
`

const sqlGetReferralStats = `
SELECT
    COALESCE(COUNT(*), 0)::int as total_users,
    COALESCE(SUM(referral_count), 0)::int as total_referrals
FROM waitlist_users
WHERE campaign_id = $1
    AND deleted_at IS NULL
`

// GetReferralAnalytics retrieves referral performance analytics
func (s *Store) GetReferralAnalytics(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) (ReferralAnalyticsResult, error) {
	var result ReferralAnalyticsResult
	err := s.db.GetContext(ctx, &result, sqlGetReferralAnalytics, campaignID, dateFrom, dateTo)
	if err != nil {
		s.logger.Error(ctx, "failed to get referral analytics", err)
		return ReferralAnalyticsResult{}, fmt.Errorf("failed to get referral analytics: %w", err)
	}

	// Get average referrals per user
	var stats struct {
		TotalUsers     int `db:"total_users"`
		TotalReferrals int `db:"total_referrals"`
	}
	err = s.db.GetContext(ctx, &stats, sqlGetReferralStats, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to get referral stats", err)
		return ReferralAnalyticsResult{}, fmt.Errorf("failed to get referral stats: %w", err)
	}

	if stats.TotalUsers > 0 {
		result.AverageReferralsPerUser = float64(stats.TotalReferrals) / float64(stats.TotalUsers)
		// Viral coefficient: average referrals that result in verified signups
		if result.TotalReferrals > 0 {
			result.ViralCoefficient = float64(result.VerifiedReferrals) / float64(stats.TotalUsers)
		}
	}

	return result, nil
}

const sqlGetReferralSourceBreakdown = `
SELECT
    COALESCE(source, 'direct') as source,
    COUNT(*)::int as count
FROM referrals
WHERE campaign_id = $1
    AND ($2::timestamptz IS NULL OR created_at >= $2)
    AND ($3::timestamptz IS NULL OR created_at <= $3)
GROUP BY source
ORDER BY count DESC
`

// GetReferralSourceBreakdown retrieves referral source breakdown
func (s *Store) GetReferralSourceBreakdown(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) ([]SourceBreakdownResult, error) {
	var results []SourceBreakdownResult
	err := s.db.SelectContext(ctx, &results, sqlGetReferralSourceBreakdown, campaignID, dateFrom, dateTo)
	if err != nil {
		s.logger.Error(ctx, "failed to get referral source breakdown", err)
		return nil, fmt.Errorf("failed to get referral source breakdown: %w", err)
	}
	return results, nil
}

// GetTimeSeriesAnalytics retrieves time-series analytics data
func (s *Store) GetTimeSeriesAnalytics(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time, granularity string) ([]TimeSeriesDataPoint, error) {
	// Determine time bucket based on granularity
	var timeBucket string
	switch granularity {
	case "hour":
		timeBucket = "1 hour"
	case "day":
		timeBucket = "1 day"
	case "week":
		timeBucket = "1 week"
	case "month":
		timeBucket = "1 month"
	default:
		timeBucket = "1 day"
	}

	query := fmt.Sprintf(`
SELECT
    time_bucket('%s', time) as date,
    COALESCE(SUM(new_signups), 0)::int as signups,
    COALESCE(SUM(new_verified), 0)::int as verified,
    COALESCE(SUM(new_referrals), 0)::int as referrals,
    COALESCE(SUM(emails_sent), 0)::int as emails_sent,
    COALESCE(SUM(emails_opened), 0)::int as emails_opened,
    COALESCE(SUM(emails_clicked), 0)::int as emails_clicked
FROM campaign_analytics
WHERE campaign_id = $1
    AND ($2::timestamptz IS NULL OR time >= $2)
    AND ($3::timestamptz IS NULL OR time <= $3)
GROUP BY date
ORDER BY date ASC
`, timeBucket)

	var results []TimeSeriesDataPoint
	err := s.db.SelectContext(ctx, &results, query, campaignID, dateFrom, dateTo)
	if err != nil {
		s.logger.Error(ctx, "failed to get time series analytics", err)
		return nil, fmt.Errorf("failed to get time series analytics: %w", err)
	}
	return results, nil
}

const sqlGetSignupSourceBreakdown = `
SELECT
    COALESCE(source, 'direct') as source,
    COUNT(*)::int as count
FROM waitlist_users
WHERE campaign_id = $1
    AND deleted_at IS NULL
    AND ($2::timestamptz IS NULL OR created_at >= $2)
    AND ($3::timestamptz IS NULL OR created_at <= $3)
GROUP BY source
ORDER BY count DESC
`

// GetSignupSourceBreakdown retrieves signup source breakdown
func (s *Store) GetSignupSourceBreakdown(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) ([]SourceBreakdownResult, error) {
	var results []SourceBreakdownResult
	err := s.db.SelectContext(ctx, &results, sqlGetSignupSourceBreakdown, campaignID, dateFrom, dateTo)
	if err != nil {
		s.logger.Error(ctx, "failed to get signup source breakdown", err)
		return nil, fmt.Errorf("failed to get signup source breakdown: %w", err)
	}
	return results, nil
}

const sqlGetUTMCampaignBreakdown = `
SELECT
    COALESCE(utm_campaign, 'none') as utm_campaign,
    COUNT(*)::int as count
FROM waitlist_users
WHERE campaign_id = $1
    AND deleted_at IS NULL
    AND ($2::timestamptz IS NULL OR created_at >= $2)
    AND ($3::timestamptz IS NULL OR created_at <= $3)
GROUP BY utm_campaign
ORDER BY count DESC
LIMIT 20
`

// GetUTMCampaignBreakdown retrieves UTM campaign breakdown
func (s *Store) GetUTMCampaignBreakdown(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) ([]map[string]interface{}, error) {
	rows, err := s.db.QueryxContext(ctx, sqlGetUTMCampaignBreakdown, campaignID, dateFrom, dateTo)
	if err != nil {
		s.logger.Error(ctx, "failed to get utm campaign breakdown", err)
		return nil, fmt.Errorf("failed to get utm campaign breakdown: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		result := make(map[string]interface{})
		err := rows.MapScan(result)
		if err != nil {
			s.logger.Error(ctx, "failed to scan utm campaign row", err)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

const sqlGetUTMSourceBreakdown = `
SELECT
    COALESCE(utm_source, 'none') as utm_source,
    COUNT(*)::int as count
FROM waitlist_users
WHERE campaign_id = $1
    AND deleted_at IS NULL
    AND ($2::timestamptz IS NULL OR created_at >= $2)
    AND ($3::timestamptz IS NULL OR created_at <= $3)
GROUP BY utm_source
ORDER BY count DESC
LIMIT 20
`

// GetUTMSourceBreakdown retrieves UTM source breakdown
func (s *Store) GetUTMSourceBreakdown(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) ([]map[string]interface{}, error) {
	rows, err := s.db.QueryxContext(ctx, sqlGetUTMSourceBreakdown, campaignID, dateFrom, dateTo)
	if err != nil {
		s.logger.Error(ctx, "failed to get utm source breakdown", err)
		return nil, fmt.Errorf("failed to get utm source breakdown: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		result := make(map[string]interface{})
		err := rows.MapScan(result)
		if err != nil {
			s.logger.Error(ctx, "failed to scan utm source row", err)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

const sqlGetFunnelAnalytics = `
WITH funnel_counts AS (
    SELECT
        COUNT(*)::int as signed_up,
        COUNT(*) FILTER (WHERE email_verified = true)::int as verified,
        COUNT(*) FILTER (WHERE referral_count > 0)::int as referred,
        COUNT(*) FILTER (WHERE status = 'converted')::int as converted
    FROM waitlist_users
    WHERE campaign_id = $1
        AND deleted_at IS NULL
        AND ($2::timestamptz IS NULL OR created_at >= $2)
        AND ($3::timestamptz IS NULL OR created_at <= $3)
)
SELECT
    signed_up,
    verified,
    referred,
    converted
FROM funnel_counts
`

// GetFunnelAnalytics retrieves conversion funnel data
func (s *Store) GetFunnelAnalytics(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) ([]FunnelStepResult, error) {
	var counts struct {
		SignedUp  int `db:"signed_up"`
		Verified  int `db:"verified"`
		Referred  int `db:"referred"`
		Converted int `db:"converted"`
	}

	err := s.db.GetContext(ctx, &counts, sqlGetFunnelAnalytics, campaignID, dateFrom, dateTo)
	if err != nil {
		s.logger.Error(ctx, "failed to get funnel analytics", err)
		return nil, fmt.Errorf("failed to get funnel analytics: %w", err)
	}

	// Calculate funnel steps with percentages and drop-off rates
	results := []FunnelStepResult{}

	if counts.SignedUp > 0 {
		results = append(results, FunnelStepResult{
			Step:        "signed_up",
			Count:       counts.SignedUp,
			Percentage:  100.0,
			DropOffRate: 0.0,
		})

		verifiedPct := float64(counts.Verified) / float64(counts.SignedUp) * 100
		results = append(results, FunnelStepResult{
			Step:        "verified",
			Count:       counts.Verified,
			Percentage:  verifiedPct,
			DropOffRate: 100 - verifiedPct,
		})

		referredPct := float64(counts.Referred) / float64(counts.SignedUp) * 100
		results = append(results, FunnelStepResult{
			Step:        "referred",
			Count:       counts.Referred,
			Percentage:  referredPct,
			DropOffRate: 100 - referredPct,
		})

		convertedPct := float64(counts.Converted) / float64(counts.SignedUp) * 100
		results = append(results, FunnelStepResult{
			Step:        "converted",
			Count:       counts.Converted,
			Percentage:  convertedPct,
			DropOffRate: 100 - convertedPct,
		})
	}

	return results, nil
}
