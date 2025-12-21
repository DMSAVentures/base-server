package processor

import (
	"base-server/internal/store"
	"context"
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
