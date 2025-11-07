package consumer

import (
	"base-server/internal/clients/kafka"
	"base-server/internal/jobs"
	"base-server/internal/jobs/workers"
	"base-server/internal/observability"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// JobConsumer handles consuming job events from Kafka
type JobConsumer struct {
	kafkaConsumer  *kafka.Consumer
	emailWorker    *workers.EmailWorker
	positionWorker *workers.PositionWorker
	rewardWorker   *workers.RewardWorker
	analyticsWorker *workers.AnalyticsWorker
	fraudWorker    *workers.FraudWorker
	logger         *observability.Logger
	workerCount    int
}

// New creates a new JobConsumer
func New(
	kafkaConsumer *kafka.Consumer,
	emailWorker *workers.EmailWorker,
	positionWorker *workers.PositionWorker,
	rewardWorker *workers.RewardWorker,
	analyticsWorker *workers.AnalyticsWorker,
	fraudWorker *workers.FraudWorker,
	logger *observability.Logger,
	workerCount int,
) *JobConsumer {
	if workerCount == 0 {
		workerCount = 10 // Default to 10 workers
	}

	return &JobConsumer{
		kafkaConsumer:   kafkaConsumer,
		emailWorker:     emailWorker,
		positionWorker:  positionWorker,
		rewardWorker:    rewardWorker,
		analyticsWorker: analyticsWorker,
		fraudWorker:     fraudWorker,
		logger:          logger,
		workerCount:     workerCount,
	}
}

// Start starts consuming events from Kafka with multiple workers
func (c *JobConsumer) Start(ctx context.Context) error {
	c.logger.Info(ctx, fmt.Sprintf("Starting job consumer with %d workers", c.workerCount))

	// Create a channel for events
	eventChan := make(chan kafka.EventMessage, 100)
	errorChan := make(chan error, 1)

	// Start consumer in a goroutine
	go func() {
		err := c.kafkaConsumer.ConsumeEvents(ctx, func(msgCtx context.Context, event kafka.EventMessage) error {
			// Send event to worker pool
			select {
			case eventChan <- event:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if err != nil {
			errorChan <- err
		}
		close(eventChan)
	}()

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < c.workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			c.worker(ctx, workerID, eventChan)
		}(i)
	}

	// Wait for completion or error
	go func() {
		wg.Wait()
		close(errorChan)
	}()

	select {
	case err := <-errorChan:
		if err != nil {
			c.logger.Error(ctx, "consumer error", err)
			return err
		}
	case <-ctx.Done():
		c.logger.Info(ctx, "Consumer context cancelled")
		return ctx.Err()
	}

	return nil
}

// worker processes events from the event channel
func (c *JobConsumer) worker(ctx context.Context, workerID int, eventChan <-chan kafka.EventMessage) {
	workerCtx := observability.WithFields(ctx, observability.Field{Key: "worker_id", Value: workerID})
	c.logger.Info(workerCtx, fmt.Sprintf("Worker %d started", workerID))

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				c.logger.Info(workerCtx, fmt.Sprintf("Worker %d stopping - channel closed", workerID))
				return
			}

			// Process event
			err := c.processEvent(workerCtx, event)
			if err != nil {
				c.logger.Error(workerCtx, fmt.Sprintf("Worker %d failed to process event", workerID), err)
			}

		case <-ctx.Done():
			c.logger.Info(workerCtx, fmt.Sprintf("Worker %d stopping - context cancelled", workerID))
			return
		}
	}
}

// processEvent processes a single event by routing to the appropriate worker
func (c *JobConsumer) processEvent(ctx context.Context, event kafka.EventMessage) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "event_id", Value: event.ID},
		observability.Field{Key: "event_type", Value: event.Type},
		observability.Field{Key: "account_id", Value: event.AccountID},
	)

	c.logger.Info(ctx, fmt.Sprintf("Processing event %s", event.Type))

	// Route based on event type
	var err error
	switch {
	case strings.HasPrefix(event.Type, "job.email."):
		err = c.processEmailJob(ctx, event)
	case event.Type == "job.position.recalculate":
		err = c.processPositionJob(ctx, event)
	case event.Type == "job.reward.deliver":
		err = c.processRewardJob(ctx, event)
	case event.Type == "job.fraud.detect":
		err = c.processFraudJob(ctx, event)
	case event.Type == "job.analytics.aggregate":
		err = c.processAnalyticsJob(ctx, event)
	default:
		c.logger.Warn(ctx, fmt.Sprintf("Unknown event type: %s", event.Type))
		return nil
	}

	if err != nil {
		c.logger.Error(ctx, fmt.Sprintf("Failed to process %s", event.Type), err)
		return err
	}

	c.logger.Info(ctx, fmt.Sprintf("Successfully processed event %s", event.Type))
	return nil
}

// processEmailJob processes an email job
func (c *JobConsumer) processEmailJob(ctx context.Context, event kafka.EventMessage) error {
	var payload jobs.EmailJobPayload

	// Extract from event.Data
	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	// Parse payload
	payload.Type = data["type"].(string)
	payload.CampaignID, _ = uuid.Parse(data["campaign_id"].(string))
	payload.UserID, _ = uuid.Parse(data["user_id"].(string))

	if templateID, ok := data["template_id"].(string); ok && templateID != "" {
		tid, _ := uuid.Parse(templateID)
		payload.TemplateID = &tid
	}

	if templateData, ok := data["template_data"].(map[string]interface{}); ok {
		payload.TemplateData = templateData
	}

	if priority, ok := data["priority"].(float64); ok {
		payload.Priority = int(priority)
	}

	return c.emailWorker.ProcessEmailJob(ctx, payload)
}

// processPositionJob processes a position recalculation job
func (c *JobConsumer) processPositionJob(ctx context.Context, event kafka.EventMessage) error {
	var payload jobs.PositionRecalcJobPayload

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	payload.CampaignID, _ = uuid.Parse(data["campaign_id"].(string))

	if ppr, ok := data["points_per_referral"].(float64); ok {
		payload.PointsPerReferral = int(ppr)
	}

	return c.positionWorker.ProcessPositionRecalc(ctx, payload)
}

// processRewardJob processes a reward delivery job
func (c *JobConsumer) processRewardJob(ctx context.Context, event kafka.EventMessage) error {
	var payload jobs.RewardDeliveryJobPayload

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	payload.UserRewardID, _ = uuid.Parse(data["user_reward_id"].(string))

	return c.rewardWorker.ProcessRewardDelivery(ctx, payload)
}

// processFraudJob processes a fraud detection job
func (c *JobConsumer) processFraudJob(ctx context.Context, event kafka.EventMessage) error {
	var payload jobs.FraudDetectionJobPayload

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	payload.CampaignID, _ = uuid.Parse(data["campaign_id"].(string))
	payload.UserID, _ = uuid.Parse(data["user_id"].(string))

	return c.fraudWorker.ProcessFraudDetection(ctx, payload)
}

// processAnalyticsJob processes an analytics aggregation job
func (c *JobConsumer) processAnalyticsJob(ctx context.Context, event kafka.EventMessage) error {
	var payload jobs.AnalyticsAggregationJobPayload

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	payload.CampaignID, _ = uuid.Parse(data["campaign_id"].(string))
	payload.Granularity = data["granularity"].(string)

	if startTime, ok := data["start_time"].(string); ok {
		payload.StartTime, _ = parseRFC3339(startTime)
	}

	if endTime, ok := data["end_time"].(string); ok {
		payload.EndTime, _ = parseRFC3339(endTime)
	}

	return c.analyticsWorker.ProcessAnalyticsAggregation(ctx, payload)
}

// Stop stops the consumer
func (c *JobConsumer) Stop() error {
	c.logger.Info(context.Background(), "Stopping job consumer")
	return c.kafkaConsumer.Close()
}

// parseRFC3339 parses RFC3339 formatted time string
func parseRFC3339(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
