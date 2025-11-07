package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"base-server/internal/email"
	"base-server/internal/jobs"
	"base-server/internal/observability"
	"base-server/internal/store"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// RewardWorker handles reward delivery jobs
type RewardWorker struct {
	store        *store.Store
	emailService email.Service
	logger       *observability.Logger
	jobClient    *jobs.Client
}

// NewRewardWorker creates a new reward worker
func NewRewardWorker(store *store.Store, emailService email.Service, jobClient *jobs.Client, logger *observability.Logger) *RewardWorker {
	return &RewardWorker{
		store:        store,
		emailService: emailService,
		jobClient:    jobClient,
		logger:       logger,
	}
}

// ProcessRewardDelivery processes a reward delivery job (for Kafka)
func (w *RewardWorker) ProcessRewardDelivery(ctx context.Context, payload jobs.RewardDeliveryJobPayload) error {
	return w.processRewardDelivery(ctx, payload)
}

// ProcessRewardDeliveryTask processes a reward delivery task (for Asynq)
func (w *RewardWorker) ProcessRewardDeliveryTask(ctx context.Context, task *asynq.Task) error {
	var payload jobs.RewardDeliveryJobPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		w.logger.Error(ctx, "failed to unmarshal reward delivery job payload", err)
		return fmt.Errorf("failed to unmarshal reward delivery job payload: %w", err)
	}
	return w.processRewardDelivery(ctx, payload)
}

// processRewardDelivery contains the core reward delivery logic
func (w *RewardWorker) processRewardDelivery(ctx context.Context, payload jobs.RewardDeliveryJobPayload) error {
	// Get user reward
	var userReward store.UserReward
	userRewards, err := w.store.GetPendingUserRewards(ctx, 1000) // Get batch of pending rewards
	if err != nil {
		w.logger.Error(ctx, "failed to get pending user rewards", err)
		return fmt.Errorf("failed to get pending user rewards: %w", err)
	}

	// Find the specific reward
	found := false
	for _, ur := range userRewards {
		if ur.ID == payload.UserRewardID {
			userReward = ur
			found = true
			break
		}
	}

	if !found {
		w.logger.Error(ctx, "user reward not found", nil)
		return fmt.Errorf("user reward not found: %s", payload.UserRewardID)
	}

	// Get reward definition
	reward, err := w.store.GetRewardByID(ctx, userReward.RewardID)
	if err != nil {
		w.logger.Error(ctx, "failed to get reward", err)
		return fmt.Errorf("failed to get reward: %w", err)
	}

	// Get user
	user, err := w.store.GetWaitlistUserByID(ctx, userReward.UserID)
	if err != nil {
		w.logger.Error(ctx, "failed to get user", err)
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Deliver reward based on delivery method
	switch reward.DeliveryMethod {
	case "email":
		err = w.deliverRewardViaEmail(ctx, user, reward, userReward)
	case "webhook":
		err = w.deliverRewardViaWebhook(ctx, user, reward, userReward)
	case "manual":
		// Manual delivery - just mark as pending and send notification to admin
		w.logger.Info(ctx, fmt.Sprintf("reward %s requires manual delivery for user %s", reward.Name, user.Email))
		return nil
	default:
		err = fmt.Errorf("unsupported delivery method: %s", reward.DeliveryMethod)
	}

	if err != nil {
		// Increment delivery attempts
		if updateErr := w.store.IncrementDeliveryAttempts(ctx, userReward.ID, err.Error()); updateErr != nil {
			w.logger.Error(ctx, "failed to increment delivery attempts", updateErr)
		}

		// If max retries exceeded, mark as failed
		if payload.RetryAttempt >= 5 {
			w.logger.Error(ctx, "max delivery attempts exceeded for reward", err)
			return fmt.Errorf("max delivery attempts exceeded: %w", err)
		}

		return fmt.Errorf("failed to deliver reward: %w", err)
	}

	// Mark reward as delivered
	if err := w.store.UpdateUserRewardStatus(ctx, userReward.ID, "delivered"); err != nil {
		w.logger.Error(ctx, "failed to update reward status", err)
		return fmt.Errorf("failed to update reward status: %w", err)
	}

	w.logger.Info(ctx, fmt.Sprintf("successfully delivered reward %s to user %s", reward.Name, user.Email))
	return nil
}

// deliverRewardViaEmail delivers a reward via email
func (w *RewardWorker) deliverRewardViaEmail(ctx context.Context, user store.WaitlistUser, reward store.Reward, userReward store.UserReward) error {
	// Extract reward data
	rewardData := userReward.RewardData
	if rewardData == nil {
		rewardData = make(map[string]interface{})
	}

	// Get campaign
	campaign, err := w.store.GetCampaignByID(ctx, userReward.CampaignID)
	if err != nil {
		return fmt.Errorf("failed to get campaign: %w", err)
	}

	// Prepare template data
	templateData := map[string]interface{}{
		"reward_name":         reward.Name,
		"reward_description":  w.getStringPtr(reward.Description),
		"reward_type":         reward.Type,
		"reward_data":         rewardData,
	}

	// If there's a reward code, include it
	if code, ok := rewardData["code"].(string); ok {
		templateData["reward_code"] = code
	}

	// Enqueue email job
	err = w.jobClient.EnqueueEmailJob(ctx, jobs.EmailJobPayload{
		Type:         "reward_earned",
		CampaignID:   campaign.ID,
		UserID:       user.ID,
		TemplateData: templateData,
		Priority:     1,
	})

	if err != nil {
		return fmt.Errorf("failed to enqueue reward email: %w", err)
	}

	return nil
}

// deliverRewardViaWebhook delivers a reward via webhook
func (w *RewardWorker) deliverRewardViaWebhook(ctx context.Context, user store.WaitlistUser, reward store.Reward, userReward store.UserReward) error {
	// Get delivery config
	deliveryConfig, ok := reward.DeliveryConfig.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid delivery config for webhook")
	}

	webhookURL, ok := deliveryConfig["webhook_url"].(string)
	if !ok || webhookURL == "" {
		return fmt.Errorf("webhook URL not configured")
	}

	// Get all active webhooks for this campaign
	webhooks, err := w.store.GetWebhooksByCampaign(ctx, reward.CampaignID)
	if err != nil {
		return fmt.Errorf("failed to get webhooks: %w", err)
	}

	// Find webhook that matches the URL
	var webhook *store.Webhook
	for _, wh := range webhooks {
		if wh.URL == webhookURL {
			webhook = &wh
			break
		}
	}

	if webhook == nil {
		return fmt.Errorf("webhook not found for URL: %s", webhookURL)
	}

	// Prepare webhook payload
	payload := map[string]interface{}{
		"event": "reward.delivered",
		"data": map[string]interface{}{
			"user_id":     user.ID,
			"email":       user.Email,
			"reward_id":   reward.ID,
			"reward_name": reward.Name,
			"reward_type": reward.Type,
			"reward_data": userReward.RewardData,
		},
	}

	// Enqueue webhook delivery job
	err = w.jobClient.EnqueueWebhookDeliveryJob(ctx, jobs.WebhookDeliveryJobPayload{
		WebhookID:  webhook.ID,
		EventType:  "reward.delivered",
		Payload:    payload,
		Attempt:    1,
		MaxRetries: 5,
	})

	if err != nil {
		return fmt.Errorf("failed to enqueue webhook delivery: %w", err)
	}

	return nil
}

// Helper function
func (w *RewardWorker) getStringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
