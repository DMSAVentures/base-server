package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"base-server/internal/jobs"
	"base-server/internal/observability"
	"base-server/internal/store"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// AnalyticsWorker handles analytics aggregation jobs
type AnalyticsWorker struct {
	store  *store.Store
	logger *observability.Logger
}

// NewAnalyticsWorker creates a new analytics worker
func NewAnalyticsWorker(store *store.Store, logger *observability.Logger) *AnalyticsWorker {
	return &AnalyticsWorker{
		store:  store,
		logger: logger,
	}
}

// ProcessAnalyticsAggregation processes an analytics aggregation job (for Kafka)
func (w *AnalyticsWorker) ProcessAnalyticsAggregation(ctx context.Context, payload jobs.AnalyticsAggregationJobPayload) error {
	return w.processAnalyticsAggregation(ctx, payload)
}

// ProcessAnalyticsAggregationTask processes an analytics aggregation task (for Asynq)
func (w *AnalyticsWorker) ProcessAnalyticsAggregationTask(ctx context.Context, task *asynq.Task) error {
	var payload jobs.AnalyticsAggregationJobPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		w.logger.Error(ctx, "failed to unmarshal analytics aggregation job payload", err)
		return fmt.Errorf("failed to unmarshal analytics aggregation job payload: %w", err)
	}
	return w.processAnalyticsAggregation(ctx, payload)
}

// processAnalyticsAggregation contains the core analytics aggregation logic
func (w *AnalyticsWorker) processAnalyticsAggregation(ctx context.Context, payload jobs.AnalyticsAggregationJobPayload) error {
	// Get campaign to ensure it exists
	campaign, err := w.store.GetCampaignByID(ctx, payload.CampaignID)
	if err != nil {
		w.logger.Error(ctx, "failed to get campaign", err)
		return fmt.Errorf("failed to get campaign: %w", err)
	}

	// Aggregate metrics for the time range
	err = w.aggregateMetrics(ctx, campaign.ID, payload.StartTime, payload.EndTime)
	if err != nil {
		w.logger.Error(ctx, "failed to aggregate metrics", err)
		return fmt.Errorf("failed to aggregate metrics: %w", err)
	}

	w.logger.Info(ctx, fmt.Sprintf("successfully aggregated analytics for campaign %s from %s to %s",
		campaign.Slug, payload.StartTime.Format(time.RFC3339), payload.EndTime.Format(time.RFC3339)))
	return nil
}

// aggregateMetrics aggregates all metrics for a campaign in a time range
func (w *AnalyticsWorker) aggregateMetrics(ctx context.Context, campaignID uuid.UUID, startTime, endTime time.Time) error {
	// Count new signups
	newSignups, err := w.store.CountNewSignupsSince(ctx, campaignID, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to count new signups: %w", err)
	}

	// Count new verified users
	newVerified, err := w.store.CountNewVerifiedSince(ctx, campaignID, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to count new verified: %w", err)
	}

	// Count new referrals
	newReferrals, err := w.store.CountNewReferralsSince(ctx, campaignID, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to count new referrals: %w", err)
	}

	// Count emails sent
	emailsSent, err := w.store.CountEmailsSentSince(ctx, campaignID, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to count emails sent: %w", err)
	}

	// Count emails opened
	emailsOpened, err := w.store.CountEmailsOpenedSince(ctx, campaignID, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to count emails opened: %w", err)
	}

	// Count emails clicked
	emailsClicked, err := w.store.CountEmailsClickedSince(ctx, campaignID, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to count emails clicked: %w", err)
	}

	// Count rewards earned in this period
	rewardsEarned, err := w.countRewardsEarnedSince(ctx, campaignID, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to count rewards earned: %w", err)
	}

	// Count rewards delivered in this period
	rewardsDelivered, err := w.countRewardsDeliveredSince(ctx, campaignID, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to count rewards delivered: %w", err)
	}

	// Get current totals
	campaign, err := w.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("failed to get campaign: %w", err)
	}

	// Create or update analytics record
	err = w.store.CreateCampaignAnalytics(ctx, store.CreateCampaignAnalyticsParams{
		Time:             startTime,
		CampaignID:       campaignID,
		NewSignups:       newSignups,
		NewVerified:      newVerified,
		NewReferrals:     newReferrals,
		NewConversions:   0, // TODO: Implement conversion tracking
		EmailsSent:       emailsSent,
		EmailsOpened:     emailsOpened,
		EmailsClicked:    emailsClicked,
		RewardsEarned:    rewardsEarned,
		RewardsDelivered: rewardsDelivered,
		TotalSignups:     campaign.TotalSignups,
		TotalVerified:    campaign.TotalVerified,
		TotalReferrals:   campaign.TotalReferrals,
	})

	if err != nil {
		return fmt.Errorf("failed to create campaign analytics: %w", err)
	}

	return nil
}

// countRewardsEarnedSince counts rewards earned in a time range
func (w *AnalyticsWorker) countRewardsEarnedSince(ctx context.Context, campaignID uuid.UUID, startTime, endTime time.Time) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM user_rewards
		WHERE campaign_id = $1 AND earned_at >= $2 AND earned_at < $3
	`

	var count int
	err := w.store.DB().GetContext(ctx, &count, query, campaignID, startTime, endTime)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// countRewardsDeliveredSince counts rewards delivered in a time range
func (w *AnalyticsWorker) countRewardsDeliveredSince(ctx context.Context, campaignID uuid.UUID, startTime, endTime time.Time) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM user_rewards
		WHERE campaign_id = $1 AND delivered_at >= $2 AND delivered_at < $3
	`

	var count int
	err := w.store.DB().GetContext(ctx, &count, query, campaignID, startTime, endTime)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ProcessHourlyAggregation is a periodic task that aggregates metrics every hour
func (w *AnalyticsWorker) ProcessHourlyAggregation(ctx context.Context) error {
	// Get all active campaigns
	campaigns, err := w.store.GetAllActiveCampaigns(ctx)
	if err != nil {
		w.logger.Error(ctx, "failed to get active campaigns", err)
		return fmt.Errorf("failed to get active campaigns: %w", err)
	}

	// Calculate time range for the last hour
	endTime := time.Now().Truncate(time.Hour)
	startTime := endTime.Add(-time.Hour)

	// Aggregate metrics for each campaign
	for _, campaign := range campaigns {
		if err := w.aggregateMetrics(ctx, campaign.ID, startTime, endTime); err != nil {
			w.logger.Error(ctx, fmt.Sprintf("failed to aggregate metrics for campaign %s", campaign.Slug), err)
			// Continue with other campaigns even if one fails
			continue
		}
	}

	w.logger.Info(ctx, fmt.Sprintf("successfully aggregated hourly analytics for %d campaigns", len(campaigns)))
	return nil
}
