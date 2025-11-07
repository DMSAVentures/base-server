package consumers

import (
	"base-server/internal/clients/kafka"
	"base-server/internal/jobs"
	"base-server/internal/jobs/workers"
	"base-server/internal/observability"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// RewardConsumer handles consuming reward.earned events and delivering rewards
type RewardConsumer struct {
	kafkaConsumer *kafka.Consumer
	rewardWorker  *workers.RewardWorker
	logger        *observability.Logger
	workerCount   int
}

// NewRewardConsumer creates a new RewardConsumer
func NewRewardConsumer(
	kafkaConsumer *kafka.Consumer,
	rewardWorker *workers.RewardWorker,
	logger *observability.Logger,
	workerCount int,
) *RewardConsumer {
	if workerCount == 0 {
		workerCount = 10 // Default to 10 workers
	}

	return &RewardConsumer{
		kafkaConsumer: kafkaConsumer,
		rewardWorker:  rewardWorker,
		logger:        logger,
		workerCount:   workerCount,
	}
}

// Start starts consuming events from Kafka with multiple workers
func (c *RewardConsumer) Start(ctx context.Context) error {
	c.logger.Info(ctx, fmt.Sprintf("Starting reward consumer with %d workers", c.workerCount))

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
func (c *RewardConsumer) worker(ctx context.Context, workerID int, eventChan <-chan kafka.EventMessage) {
	workerCtx := observability.WithFields(ctx, observability.Field{Key: "worker_id", Value: workerID})
	c.logger.Info(workerCtx, fmt.Sprintf("Reward worker %d started", workerID))

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				c.logger.Info(workerCtx, fmt.Sprintf("Reward worker %d stopping - channel closed", workerID))
				return
			}

			// Process event
			err := c.processEvent(workerCtx, event)
			if err != nil {
				c.logger.Error(workerCtx, fmt.Sprintf("Reward worker %d failed to process event", workerID), err)
			}

		case <-ctx.Done():
			c.logger.Info(workerCtx, fmt.Sprintf("Reward worker %d stopping - context cancelled", workerID))
			return
		}
	}
}

// processEvent processes a single reward.earned event
func (c *RewardConsumer) processEvent(ctx context.Context, event kafka.EventMessage) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "event_id", Value: event.ID},
		observability.Field{Key: "event_type", Value: event.Type},
		observability.Field{Key: "account_id", Value: event.AccountID},
	)

	// Only process reward.earned events
	if event.Type != "reward.earned" {
		c.logger.Info(ctx, fmt.Sprintf("Reward consumer ignoring event type: %s", event.Type))
		return nil
	}

	c.logger.Info(ctx, "Processing reward.earned event for reward delivery")

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	userRewardID, _ := uuid.Parse(data["user_reward_id"].(string))

	payload := jobs.RewardDeliveryJobPayload{
		UserRewardID: userRewardID,
	}

	err = c.rewardWorker.ProcessRewardDelivery(ctx, payload)
	if err != nil {
		c.logger.Error(ctx, "Failed to deliver reward", err)
		return err
	}

	c.logger.Info(ctx, "Successfully delivered reward")
	return nil
}

// Stop stops the consumer
func (c *RewardConsumer) Stop() error {
	c.logger.Info(context.Background(), "Stopping reward consumer")
	return c.kafkaConsumer.Close()
}
