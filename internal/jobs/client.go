package jobs

import (
	"context"
	"fmt"

	"base-server/internal/observability"

	"github.com/hibiken/asynq"
)

// Client handles enqueueing background jobs
type Client struct {
	client *asynq.Client
	logger *observability.Logger
}

// NewClient creates a new job client
func NewClient(redisAddr string, logger *observability.Logger) *Client {
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	return &Client{
		client: client,
		logger: logger,
	}
}

// Close closes the client connection
func (c *Client) Close() error {
	return c.client.Close()
}

// EnqueueEmailJob enqueues an email job
func (c *Client) EnqueueEmailJob(ctx context.Context, payload EmailJobPayload) error {
	task, err := NewEmailTask(payload, QueueHigh)
	if err != nil {
		c.logger.Error(ctx, "failed to create email task", err)
		return fmt.Errorf("failed to create email task: %w", err)
	}

	info, err := c.client.EnqueueContext(ctx, task)
	if err != nil {
		c.logger.Error(ctx, "failed to enqueue email task", err)
		return fmt.Errorf("failed to enqueue email task: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("enqueued email task: %s (queue: %s)", info.ID, info.Queue))
	return nil
}

// EnqueuePositionRecalcJob enqueues a position recalculation job
func (c *Client) EnqueuePositionRecalcJob(ctx context.Context, payload PositionRecalcJobPayload) error {
	task, err := NewPositionRecalcTask(payload)
	if err != nil {
		c.logger.Error(ctx, "failed to create position recalc task", err)
		return fmt.Errorf("failed to create position recalc task: %w", err)
	}

	info, err := c.client.EnqueueContext(ctx, task)
	if err != nil {
		c.logger.Error(ctx, "failed to enqueue position recalc task", err)
		return fmt.Errorf("failed to enqueue position recalc task: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("enqueued position recalc task: %s", info.ID))
	return nil
}

// EnqueueRewardDeliveryJob enqueues a reward delivery job
func (c *Client) EnqueueRewardDeliveryJob(ctx context.Context, payload RewardDeliveryJobPayload) error {
	task, err := NewRewardDeliveryTask(payload)
	if err != nil {
		c.logger.Error(ctx, "failed to create reward delivery task", err)
		return fmt.Errorf("failed to create reward delivery task: %w", err)
	}

	info, err := c.client.EnqueueContext(ctx, task)
	if err != nil {
		c.logger.Error(ctx, "failed to enqueue reward delivery task", err)
		return fmt.Errorf("failed to enqueue reward delivery task: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("enqueued reward delivery task: %s", info.ID))
	return nil
}

// EnqueueWebhookDeliveryJob enqueues a webhook delivery job
func (c *Client) EnqueueWebhookDeliveryJob(ctx context.Context, payload WebhookDeliveryJobPayload) error {
	task, err := NewWebhookDeliveryTask(payload)
	if err != nil {
		c.logger.Error(ctx, "failed to create webhook delivery task", err)
		return fmt.Errorf("failed to create webhook delivery task: %w", err)
	}

	info, err := c.client.EnqueueContext(ctx, task)
	if err != nil {
		c.logger.Error(ctx, "failed to enqueue webhook delivery task", err)
		return fmt.Errorf("failed to enqueue webhook delivery task: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("enqueued webhook delivery task: %s", info.ID))
	return nil
}

// EnqueueFraudDetectionJob enqueues a fraud detection job
func (c *Client) EnqueueFraudDetectionJob(ctx context.Context, payload FraudDetectionJobPayload) error {
	task, err := NewFraudDetectionTask(payload)
	if err != nil {
		c.logger.Error(ctx, "failed to create fraud detection task", err)
		return fmt.Errorf("failed to create fraud detection task: %w", err)
	}

	info, err := c.client.EnqueueContext(ctx, task)
	if err != nil {
		c.logger.Error(ctx, "failed to enqueue fraud detection task", err)
		return fmt.Errorf("failed to enqueue fraud detection task: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("enqueued fraud detection task: %s", info.ID))
	return nil
}

// EnqueueAnalyticsAggregationJob enqueues an analytics aggregation job
func (c *Client) EnqueueAnalyticsAggregationJob(ctx context.Context, payload AnalyticsAggregationJobPayload) error {
	task, err := NewAnalyticsAggregationTask(payload)
	if err != nil {
		c.logger.Error(ctx, "failed to create analytics aggregation task", err)
		return fmt.Errorf("failed to create analytics aggregation task: %w", err)
	}

	info, err := c.client.EnqueueContext(ctx, task)
	if err != nil {
		c.logger.Error(ctx, "failed to enqueue analytics aggregation task", err)
		return fmt.Errorf("failed to enqueue analytics aggregation task: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("enqueued analytics aggregation task: %s", info.ID))
	return nil
}
