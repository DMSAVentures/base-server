package events

import (
	"context"

	"github.com/google/uuid"
)

// EventDispatcher defines the interface for dispatching domain-specific webhook events
type EventDispatcher interface {
	// DispatchUserCreated dispatches a user.created event
	DispatchUserCreated(ctx context.Context, accountID, campaignID uuid.UUID, userData map[string]interface{})

	// DispatchUserVerified dispatches a user.verified event
	DispatchUserVerified(ctx context.Context, accountID, campaignID uuid.UUID, userData map[string]interface{})

	// DispatchUserPositionChanged dispatches a user.position_changed event
	DispatchUserPositionChanged(ctx context.Context, accountID, campaignID uuid.UUID, userData map[string]interface{}, oldPosition, newPosition int)

	// DispatchReferralCreated dispatches a referral.created event
	DispatchReferralCreated(ctx context.Context, accountID, campaignID uuid.UUID, referralData map[string]interface{})

	// DispatchReferralVerified dispatches a referral.verified event
	DispatchReferralVerified(ctx context.Context, accountID, campaignID uuid.UUID, referralData map[string]interface{})

	// DispatchRewardEarned dispatches a reward.earned event
	DispatchRewardEarned(ctx context.Context, accountID, campaignID uuid.UUID, rewardData map[string]interface{})

	// DispatchCampaignMilestone dispatches a campaign.milestone event
	DispatchCampaignMilestone(ctx context.Context, accountID, campaignID uuid.UUID, milestone int, totalSignups int)

	// DispatchEmailSent dispatches an email.sent event
	DispatchEmailSent(ctx context.Context, accountID, campaignID uuid.UUID, emailData map[string]interface{})

	// DispatchEmailDelivered dispatches an email.delivered event
	DispatchEmailDelivered(ctx context.Context, accountID, campaignID uuid.UUID, emailData map[string]interface{})
}
