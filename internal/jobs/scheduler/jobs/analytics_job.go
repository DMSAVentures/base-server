package jobs

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AnalyticsAggregationJob runs analytics aggregation on a schedule
type AnalyticsAggregationJob struct {
	store    *store.Store
	logger   *observability.Logger
	interval time.Duration
}

// NewAnalyticsAggregationJob creates a new analytics aggregation job
func NewAnalyticsAggregationJob(store *store.Store, logger *observability.Logger, interval time.Duration) *AnalyticsAggregationJob {
	if interval == 0 {
		interval = 1 * time.Hour // Default to hourly
	}

	return &AnalyticsAggregationJob{
		store:    store,
		logger:   logger,
		interval: interval,
	}
}

// Name returns the job name
func (j *AnalyticsAggregationJob) Name() string {
	return "analytics_aggregation"
}

// Schedule returns how often the job should run
func (j *AnalyticsAggregationJob) Schedule() time.Duration {
	return j.interval
}

// Run executes the analytics aggregation
func (j *AnalyticsAggregationJob) Run(ctx context.Context) error {
	j.logger.Info(ctx, "Running analytics aggregation job")

	// Get all active campaigns
	campaigns, err := j.store.GetAllActiveCampaigns(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active campaigns: %w", err)
	}

	j.logger.Info(ctx, fmt.Sprintf("Found %d active campaigns to aggregate", len(campaigns)))

	// Determine time window based on interval
	endTime := time.Now().Truncate(j.interval)
	startTime := endTime.Add(-j.interval)

	successCount := 0
	errorCount := 0

	// Aggregate metrics for each campaign
	for _, campaign := range campaigns {
		campaignCtx := observability.WithFields(ctx,
			observability.Field{Key: "campaign_id", Value: campaign.ID},
			observability.Field{Key: "campaign_name", Value: campaign.Name},
		)

		err := j.aggregateCampaignMetrics(campaignCtx, campaign.ID, startTime, endTime)
		if err != nil {
			j.logger.Error(campaignCtx, "Failed to aggregate metrics for campaign", err)
			errorCount++
			continue
		}

		successCount++
	}

	j.logger.Info(ctx, fmt.Sprintf("Analytics aggregation completed: %d succeeded, %d failed", successCount, errorCount))

	return nil
}

// aggregateCampaignMetrics aggregates metrics for a single campaign
func (j *AnalyticsAggregationJob) aggregateCampaignMetrics(ctx context.Context, campaignID uuid.UUID, startTime, endTime time.Time) error {
	// Count new signups
	signupsCount, err := j.store.CountNewSignupsSince(ctx, campaignID, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to count signups: %w", err)
	}

	// Count email verifications
	verificationsCount, err := j.store.CountEmailVerificationsSince(ctx, campaignID, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to count verifications: %w", err)
	}

	// Count new referrals
	referralsCount, err := j.store.CountNewReferralsSince(ctx, campaignID, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to count referrals: %w", err)
	}

	// Count emails sent
	emailsSentCount, err := j.store.CountEmailsSentSince(ctx, campaignID, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to count emails sent: %w", err)
	}

	// Count rewards delivered
	rewardsCount, err := j.store.CountRewardsDeliveredSince(ctx, campaignID, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to count rewards: %w", err)
	}

	// Create analytics record
	err = j.store.CreateCampaignAnalytics(ctx, store.CreateCampaignAnalyticsParams{
		CampaignID:         campaignID,
		PeriodStart:        startTime,
		PeriodEnd:          endTime,
		Granularity:        getGranularity(j.interval),
		SignupsCount:       signupsCount,
		VerificationsCount: verificationsCount,
		ReferralsCount:     referralsCount,
		EmailsSentCount:    emailsSentCount,
		RewardsCount:       rewardsCount,
	})

	if err != nil {
		return fmt.Errorf("failed to create analytics record: %w", err)
	}

	j.logger.Info(ctx, fmt.Sprintf("Aggregated metrics: signups=%d, verifications=%d, referrals=%d, emails=%d, rewards=%d",
		signupsCount, verificationsCount, referralsCount, emailsSentCount, rewardsCount))

	return nil
}

// getGranularity determines granularity based on interval
func getGranularity(interval time.Duration) string {
	switch {
	case interval <= time.Hour:
		return "hourly"
	case interval <= 24*time.Hour:
		return "daily"
	case interval <= 7*24*time.Hour:
		return "weekly"
	default:
		return "monthly"
	}
}
