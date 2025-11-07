package jobs

import (
	"base-server/internal/jobs"
	"base-server/internal/jobs/workers"
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// FraudDetectionJob runs fraud detection checks on a schedule
type FraudDetectionJob struct {
	store       *store.Store
	fraudWorker *workers.FraudWorker
	logger      *observability.Logger
	interval    time.Duration
	lookbackWindow time.Duration
}

// NewFraudDetectionJob creates a new fraud detection job
func NewFraudDetectionJob(
	store *store.Store,
	fraudWorker *workers.FraudWorker,
	logger *observability.Logger,
	interval time.Duration,
) *FraudDetectionJob {
	if interval == 0 {
		interval = 15 * time.Minute // Default to every 15 minutes
	}

	return &FraudDetectionJob{
		store:          store,
		fraudWorker:    fraudWorker,
		logger:         logger,
		interval:       interval,
		lookbackWindow: interval * 2, // Check users from the last 2 intervals
	}
}

// Name returns the job name
func (j *FraudDetectionJob) Name() string {
	return "fraud_detection"
}

// Schedule returns how often the job should run
func (j *FraudDetectionJob) Schedule() time.Duration {
	return j.interval
}

// Run executes fraud detection checks
func (j *FraudDetectionJob) Run(ctx context.Context) error {
	j.logger.Info(ctx, "Running fraud detection job")

	// Get all active campaigns
	campaigns, err := j.store.GetAllActiveCampaigns(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active campaigns: %w", err)
	}

	j.logger.Info(ctx, fmt.Sprintf("Found %d active campaigns for fraud detection", len(campaigns)))

	totalChecked := 0
	totalFlagged := 0

	// Run fraud checks for each campaign
	for _, campaign := range campaigns {
		campaignCtx := observability.WithFields(ctx,
			observability.Field{Key: "campaign_id", Value: campaign.ID},
			observability.Field{Key: "campaign_name", Value: campaign.Name},
		)

		checked, flagged, err := j.checkCampaignForFraud(campaignCtx, campaign.ID)
		if err != nil {
			j.logger.Error(campaignCtx, "Failed to run fraud checks for campaign", err)
			continue
		}

		totalChecked += checked
		totalFlagged += flagged
	}

	j.logger.Info(ctx, fmt.Sprintf("Fraud detection completed: checked %d users, flagged %d", totalChecked, totalFlagged))

	return nil
}

// checkCampaignForFraud runs fraud checks for a single campaign
func (j *FraudDetectionJob) checkCampaignForFraud(ctx context.Context, campaignID uuid.UUID) (int, int, error) {
	// Get recently joined users (within lookback window)
	cutoffTime := time.Now().Add(-j.lookbackWindow)
	recentUsers, err := j.store.GetWaitlistUsersSince(ctx, campaignID, cutoffTime)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get recent users: %w", err)
	}

	if len(recentUsers) == 0 {
		j.logger.Info(ctx, "No recent users to check for fraud")
		return 0, 0, nil
	}

	j.logger.Info(ctx, fmt.Sprintf("Checking %d recent users for fraud", len(recentUsers)))

	checkedCount := 0
	flaggedCount := 0

	// Run fraud checks on each user
	for _, user := range recentUsers {
		userCtx := observability.WithFields(ctx,
			observability.Field{Key: "user_id", Value: user.ID},
			observability.Field{Key: "user_email", Value: user.Email},
		)

		// Run all fraud check types
		payload := jobs.FraudDetectionJobPayload{
			CampaignID: campaignID,
			UserID:     user.ID,
			CheckTypes: []string{
				"self_referral",
				"velocity",
				"fake_email",
				"bot_detection",
				"duplicate_ip",
				"duplicate_device",
			},
		}

		err := j.fraudWorker.ProcessFraudDetection(userCtx, payload)
		if err != nil {
			j.logger.Error(userCtx, "Failed to process fraud detection", err)
			continue
		}

		checkedCount++

		// Check if user has any fraud detections
		detections, err := j.store.GetFraudDetectionsByUser(ctx, user.ID)
		if err != nil {
			j.logger.Error(userCtx, "Failed to get fraud detections", err)
			continue
		}

		// Count suspicious detections
		suspiciousCount := 0
		for _, detection := range detections {
			if detection.RiskLevel == "high" || detection.RiskLevel == "critical" {
				suspiciousCount++
			}
		}

		if suspiciousCount > 0 {
			flaggedCount++
			j.logger.Warn(ctx, fmt.Sprintf("User %s flagged with %d suspicious fraud detections",
				user.Email, suspiciousCount))
		}
	}

	j.logger.Info(ctx, fmt.Sprintf("Campaign fraud check complete: checked %d users, flagged %d",
		checkedCount, flaggedCount))

	return checkedCount, flaggedCount, nil
}
