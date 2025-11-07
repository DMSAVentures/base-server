package producer

import (
	"base-server/internal/clients/kafka"
	"base-server/internal/jobs"
	"base-server/internal/observability"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// JobProducer handles publishing background job events to Kafka
type JobProducer struct {
	kafkaProducer *kafka.Producer
	logger        *observability.Logger
}

// New creates a new JobProducer
func New(kafkaProducer *kafka.Producer, logger *observability.Logger) *JobProducer {
	return &JobProducer{
		kafkaProducer: kafkaProducer,
		logger:        logger,
	}
}

// EnqueueEmailJob enqueues an email job to Kafka
func (p *JobProducer) EnqueueEmailJob(ctx context.Context, payload jobs.EmailJobPayload) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "job_type", Value: "email"},
		observability.Field{Key: "email_type", Value: payload.Type},
		observability.Field{Key: "campaign_id", Value: payload.CampaignID},
		observability.Field{Key: "user_id", Value: payload.UserID},
	)

	event := kafka.EventMessage{
		ID:         uuid.New().String(),
		Type:       fmt.Sprintf("job.email.%s", payload.Type),
		AccountID:  payload.CampaignID.String(), // Use campaign_id as account context
		CampaignID: &payload.CampaignID.String(),
		Data: map[string]interface{}{
			"type":          payload.Type,
			"campaign_id":   payload.CampaignID.String(),
			"user_id":       payload.UserID.String(),
			"template_id":   payload.TemplateID,
			"template_data": payload.TemplateData,
			"priority":      payload.Priority,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	err := p.kafkaProducer.PublishEvent(ctx, event)
	if err != nil {
		p.logger.Error(ctx, "failed to enqueue email job", err)
		return fmt.Errorf("failed to enqueue email job: %w", err)
	}

	p.logger.Info(ctx, fmt.Sprintf("enqueued email job (%s)", payload.Type))
	return nil
}

// EnqueuePositionRecalcJob enqueues a position recalculation job
func (p *JobProducer) EnqueuePositionRecalcJob(ctx context.Context, payload jobs.PositionRecalcJobPayload) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "job_type", Value: "position_recalc"},
		observability.Field{Key: "campaign_id", Value: payload.CampaignID},
	)

	campaignIDStr := payload.CampaignID.String()
	event := kafka.EventMessage{
		ID:         uuid.New().String(),
		Type:       "job.position.recalculate",
		AccountID:  payload.CampaignID.String(),
		CampaignID: &campaignIDStr,
		Data: map[string]interface{}{
			"campaign_id":       payload.CampaignID.String(),
			"points_per_referral": payload.PointsPerReferral,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	err := p.kafkaProducer.PublishEvent(ctx, event)
	if err != nil {
		p.logger.Error(ctx, "failed to enqueue position recalc job", err)
		return fmt.Errorf("failed to enqueue position recalc job: %w", err)
	}

	p.logger.Info(ctx, "enqueued position recalc job")
	return nil
}

// EnqueueRewardDeliveryJob enqueues a reward delivery job
func (p *JobProducer) EnqueueRewardDeliveryJob(ctx context.Context, payload jobs.RewardDeliveryJobPayload) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "job_type", Value: "reward_delivery"},
		observability.Field{Key: "user_reward_id", Value: payload.UserRewardID},
	)

	event := kafka.EventMessage{
		ID:        uuid.New().String(),
		Type:      "job.reward.deliver",
		AccountID: payload.UserRewardID.String(), // Use reward ID as key
		Data: map[string]interface{}{
			"user_reward_id": payload.UserRewardID.String(),
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	err := p.kafkaProducer.PublishEvent(ctx, event)
	if err != nil {
		p.logger.Error(ctx, "failed to enqueue reward delivery job", err)
		return fmt.Errorf("failed to enqueue reward delivery job: %w", err)
	}

	p.logger.Info(ctx, "enqueued reward delivery job")
	return nil
}

// EnqueueFraudDetectionJob enqueues a fraud detection job
func (p *JobProducer) EnqueueFraudDetectionJob(ctx context.Context, payload jobs.FraudDetectionJobPayload) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "job_type", Value: "fraud_detection"},
		observability.Field{Key: "campaign_id", Value: payload.CampaignID},
		observability.Field{Key: "user_id", Value: payload.UserID},
	)

	campaignIDStr := payload.CampaignID.String()
	event := kafka.EventMessage{
		ID:         uuid.New().String(),
		Type:       "job.fraud.detect",
		AccountID:  payload.CampaignID.String(),
		CampaignID: &campaignIDStr,
		Data: map[string]interface{}{
			"campaign_id": payload.CampaignID.String(),
			"user_id":     payload.UserID.String(),
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	err := p.kafkaProducer.PublishEvent(ctx, event)
	if err != nil {
		p.logger.Error(ctx, "failed to enqueue fraud detection job", err)
		return fmt.Errorf("failed to enqueue fraud detection job: %w", err)
	}

	p.logger.Info(ctx, "enqueued fraud detection job")
	return nil
}

// EnqueueAnalyticsAggregationJob enqueues an analytics aggregation job
func (p *JobProducer) EnqueueAnalyticsAggregationJob(ctx context.Context, payload jobs.AnalyticsAggregationJobPayload) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "job_type", Value: "analytics_aggregation"},
		observability.Field{Key: "campaign_id", Value: payload.CampaignID},
		observability.Field{Key: "granularity", Value: payload.Granularity},
	)

	campaignIDStr := payload.CampaignID.String()
	event := kafka.EventMessage{
		ID:         uuid.New().String(),
		Type:       "job.analytics.aggregate",
		AccountID:  payload.CampaignID.String(),
		CampaignID: &campaignIDStr,
		Data: map[string]interface{}{
			"campaign_id": payload.CampaignID.String(),
			"granularity": payload.Granularity,
			"start_time":  payload.StartTime.Format(time.RFC3339),
			"end_time":    payload.EndTime.Format(time.RFC3339),
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	err := p.kafkaProducer.PublishEvent(ctx, event)
	if err != nil {
		p.logger.Error(ctx, "failed to enqueue analytics aggregation job", err)
		return fmt.Errorf("failed to enqueue analytics aggregation job: %w", err)
	}

	p.logger.Info(ctx, "enqueued analytics aggregation job")
	return nil
}

