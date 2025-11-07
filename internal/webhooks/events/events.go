package events

import (
	"base-server/internal/observability"
	"base-server/internal/webhooks/producer"
	"context"

	"github.com/google/uuid"
)

// Event types
const (
	// User events
	EventUserCreated         = "user.created"
	EventUserUpdated         = "user.updated"
	EventUserVerified        = "user.verified"
	EventUserDeleted         = "user.deleted"
	EventUserPositionChanged = "user.position_changed"
	EventUserConverted       = "user.converted"

	// Referral events
	EventReferralCreated   = "referral.created"
	EventReferralVerified  = "referral.verified"
	EventReferralConverted = "referral.converted"

	// Reward events
	EventRewardEarned    = "reward.earned"
	EventRewardDelivered = "reward.delivered"
	EventRewardRedeemed  = "reward.redeemed"

	// Campaign events
	EventCampaignMilestone = "campaign.milestone"
	EventCampaignLaunched  = "campaign.launched"
	EventCampaignCompleted = "campaign.completed"

	// Email events
	EventEmailSent      = "email.sent"
	EventEmailDelivered = "email.delivered"
	EventEmailOpened    = "email.opened"
	EventEmailClicked   = "email.clicked"
	EventEmailBounced   = "email.bounced"
)

// EventDispatcher provides convenience methods for dispatching webhook events
type EventDispatcher struct {
	eventProducer *producer.EventProducer
	logger        *observability.Logger
}

// NewEventDispatcher creates a new EventDispatcher
func NewEventDispatcher(eventProducer *producer.EventProducer, logger *observability.Logger) *EventDispatcher {
	return &EventDispatcher{
		eventProducer: eventProducer,
		logger:        logger,
	}
}

// DispatchUserCreated dispatches a user.created event
func (d *EventDispatcher) DispatchUserCreated(ctx context.Context, accountID, campaignID uuid.UUID, userData map[string]interface{}) {
	data := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"user":        userData,
	}

	err := d.eventProducer.PublishEvent(ctx, accountID, &campaignID, EventUserCreated, data)
	if err != nil {
		d.logger.Error(ctx, "failed to dispatch user.created event", err)
	}
}

// DispatchUserVerified dispatches a user.verified event
func (d *EventDispatcher) DispatchUserVerified(ctx context.Context, accountID, campaignID uuid.UUID, userData map[string]interface{}) {
	data := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"user":        userData,
	}

	err := d.eventProducer.PublishEvent(ctx, accountID, &campaignID, EventUserVerified, data)
	if err != nil {
		d.logger.Error(ctx, "failed to dispatch user.verified event", err)
	}
}

// DispatchUserPositionChanged dispatches a user.position_changed event
func (d *EventDispatcher) DispatchUserPositionChanged(ctx context.Context, accountID, campaignID uuid.UUID, userData map[string]interface{}, oldPosition, newPosition int) {
	data := map[string]interface{}{
		"campaign_id":  campaignID.String(),
		"user":         userData,
		"old_position": oldPosition,
		"new_position": newPosition,
	}

	err := d.eventProducer.PublishEvent(ctx, accountID, &campaignID, EventUserPositionChanged, data)
	if err != nil {
		d.logger.Error(ctx, "failed to dispatch user.position_changed event", err)
	}
}

// DispatchReferralCreated dispatches a referral.created event
func (d *EventDispatcher) DispatchReferralCreated(ctx context.Context, accountID, campaignID uuid.UUID, referralData map[string]interface{}) {
	data := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"referral":    referralData,
	}

	err := d.eventProducer.PublishEvent(ctx, accountID, &campaignID, EventReferralCreated, data)
	if err != nil {
		d.logger.Error(ctx, "failed to dispatch referral.created event", err)
	}
}

// DispatchReferralVerified dispatches a referral.verified event
func (d *EventDispatcher) DispatchReferralVerified(ctx context.Context, accountID, campaignID uuid.UUID, referralData map[string]interface{}) {
	data := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"referral":    referralData,
	}

	err := d.eventProducer.PublishEvent(ctx, accountID, &campaignID, EventReferralVerified, data)
	if err != nil {
		d.logger.Error(ctx, "failed to dispatch referral.verified event", err)
	}
}

// DispatchRewardEarned dispatches a reward.earned event
func (d *EventDispatcher) DispatchRewardEarned(ctx context.Context, accountID, campaignID uuid.UUID, rewardData map[string]interface{}) {
	data := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"reward":      rewardData,
	}

	err := d.eventProducer.PublishEvent(ctx, accountID, &campaignID, EventRewardEarned, data)
	if err != nil {
		d.logger.Error(ctx, "failed to dispatch reward.earned event", err)
	}
}

// DispatchCampaignMilestone dispatches a campaign.milestone event
func (d *EventDispatcher) DispatchCampaignMilestone(ctx context.Context, accountID, campaignID uuid.UUID, milestone int, totalSignups int) {
	data := map[string]interface{}{
		"campaign_id":   campaignID.String(),
		"milestone":     milestone,
		"total_signups": totalSignups,
	}

	err := d.eventProducer.PublishEvent(ctx, accountID, &campaignID, EventCampaignMilestone, data)
	if err != nil {
		d.logger.Error(ctx, "failed to dispatch campaign.milestone event", err)
	}
}

// DispatchEmailSent dispatches an email.sent event
func (d *EventDispatcher) DispatchEmailSent(ctx context.Context, accountID, campaignID uuid.UUID, emailData map[string]interface{}) {
	data := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"email":       emailData,
	}

	err := d.eventProducer.PublishEvent(ctx, accountID, &campaignID, EventEmailSent, data)
	if err != nil {
		d.logger.Error(ctx, "failed to dispatch email.sent event", err)
	}
}

// DispatchEmailDelivered dispatches an email.delivered event
func (d *EventDispatcher) DispatchEmailDelivered(ctx context.Context, accountID, campaignID uuid.UUID, emailData map[string]interface{}) {
	data := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"email":       emailData,
	}

	err := d.eventProducer.PublishEvent(ctx, accountID, &campaignID, EventEmailDelivered, data)
	if err != nil {
		d.logger.Error(ctx, "failed to dispatch email.delivered event", err)
	}
}
