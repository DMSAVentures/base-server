package events

import (
	"base-server/internal/clients/kafka"
	"base-server/internal/observability"
	"context"
	"time"

	"github.com/google/uuid"
)

// Publisher handles publishing domain events to Kafka
type Publisher struct {
	kafkaProducer *kafka.Producer
	logger        *observability.Logger
}

// NewPublisher creates a new event publisher
func NewPublisher(kafkaProducer *kafka.Producer, logger *observability.Logger) *Publisher {
	return &Publisher{
		kafkaProducer: kafkaProducer,
		logger:        logger,
	}
}

// PublishUserSignedUp publishes a user.signed_up event
func (p *Publisher) PublishUserSignedUp(ctx context.Context, userID, campaignID uuid.UUID, email string) error {
	campaignIDStr := campaignID.String()
	event := kafka.EventMessage{
		ID:         uuid.New().String(),
		Type:       "user.signed_up",
		AccountID:  campaignID.String(),
		CampaignID: &campaignIDStr,
		Data: map[string]interface{}{
			"user_id":     userID.String(),
			"campaign_id": campaignID.String(),
			"email":       email,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return p.kafkaProducer.PublishEvent(ctx, event)
}

// PublishUserVerified publishes a user.verified event
func (p *Publisher) PublishUserVerified(ctx context.Context, userID, campaignID uuid.UUID, email string) error {
	campaignIDStr := campaignID.String()
	event := kafka.EventMessage{
		ID:         uuid.New().String(),
		Type:       "user.verified",
		AccountID:  campaignID.String(),
		CampaignID: &campaignIDStr,
		Data: map[string]interface{}{
			"user_id":     userID.String(),
			"campaign_id": campaignID.String(),
			"email":       email,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return p.kafkaProducer.PublishEvent(ctx, event)
}

// PublishReferralCreated publishes a referral.created event
func (p *Publisher) PublishReferralCreated(ctx context.Context, referralID, referrerID, referredID, campaignID uuid.UUID) error {
	campaignIDStr := campaignID.String()
	event := kafka.EventMessage{
		ID:         uuid.New().String(),
		Type:       "referral.created",
		AccountID:  campaignID.String(),
		CampaignID: &campaignIDStr,
		Data: map[string]interface{}{
			"referral_id": referralID.String(),
			"referrer_id": referrerID.String(),
			"referred_id": referredID.String(),
			"campaign_id": campaignID.String(),
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return p.kafkaProducer.PublishEvent(ctx, event)
}

// PublishReferralVerified publishes a referral.verified event
func (p *Publisher) PublishReferralVerified(ctx context.Context, referralID, referrerID, referredID, campaignID uuid.UUID, pointsPerReferral int) error {
	campaignIDStr := campaignID.String()
	event := kafka.EventMessage{
		ID:         uuid.New().String(),
		Type:       "referral.verified",
		AccountID:  campaignID.String(),
		CampaignID: &campaignIDStr,
		Data: map[string]interface{}{
			"referral_id":         referralID.String(),
			"referrer_id":         referrerID.String(),
			"referred_id":         referredID.String(),
			"campaign_id":         campaignID.String(),
			"points_per_referral": pointsPerReferral,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return p.kafkaProducer.PublishEvent(ctx, event)
}

// PublishRewardEarned publishes a reward.earned event
func (p *Publisher) PublishRewardEarned(ctx context.Context, userRewardID, userID, campaignID, rewardID uuid.UUID) error {
	campaignIDStr := campaignID.String()
	event := kafka.EventMessage{
		ID:         uuid.New().String(),
		Type:       "reward.earned",
		AccountID:  campaignID.String(),
		CampaignID: &campaignIDStr,
		Data: map[string]interface{}{
			"user_reward_id": userRewardID.String(),
			"user_id":        userID.String(),
			"campaign_id":    campaignID.String(),
			"reward_id":      rewardID.String(),
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return p.kafkaProducer.PublishEvent(ctx, event)
}

// PublishCampaignMilestoneReached publishes a campaign.milestone_reached event
func (p *Publisher) PublishCampaignMilestoneReached(ctx context.Context, campaignID uuid.UUID, milestone string, participantCount int) error {
	campaignIDStr := campaignID.String()
	event := kafka.EventMessage{
		ID:         uuid.New().String(),
		Type:       "campaign.milestone_reached",
		AccountID:  campaignID.String(),
		CampaignID: &campaignIDStr,
		Data: map[string]interface{}{
			"campaign_id":       campaignID.String(),
			"milestone":         milestone,
			"participant_count": participantCount,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return p.kafkaProducer.PublishEvent(ctx, event)
}
