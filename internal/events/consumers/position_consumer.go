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

// PositionConsumer handles consuming referral.verified events and recalculating positions
type PositionConsumer struct {
	kafkaConsumer  *kafka.Consumer
	positionWorker *workers.PositionWorker
	logger         *observability.Logger
	workerCount    int
}

// NewPositionConsumer creates a new PositionConsumer
func NewPositionConsumer(
	kafkaConsumer *kafka.Consumer,
	positionWorker *workers.PositionWorker,
	logger *observability.Logger,
	workerCount int,
) *PositionConsumer {
	if workerCount == 0 {
		workerCount = 10 // Default to 10 workers
	}

	return &PositionConsumer{
		kafkaConsumer:  kafkaConsumer,
		positionWorker: positionWorker,
		logger:         logger,
		workerCount:    workerCount,
	}
}

// Start starts consuming events from Kafka with multiple workers
func (c *PositionConsumer) Start(ctx context.Context) error {
	c.logger.Info(ctx, fmt.Sprintf("Starting position consumer with %d workers", c.workerCount))

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
func (c *PositionConsumer) worker(ctx context.Context, workerID int, eventChan <-chan kafka.EventMessage) {
	workerCtx := observability.WithFields(ctx, observability.Field{Key: "worker_id", Value: workerID})
	c.logger.Info(workerCtx, fmt.Sprintf("Position worker %d started", workerID))

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				c.logger.Info(workerCtx, fmt.Sprintf("Position worker %d stopping - channel closed", workerID))
				return
			}

			// Process event
			err := c.processEvent(workerCtx, event)
			if err != nil {
				c.logger.Error(workerCtx, fmt.Sprintf("Position worker %d failed to process event", workerID), err)
			}

		case <-ctx.Done():
			c.logger.Info(workerCtx, fmt.Sprintf("Position worker %d stopping - context cancelled", workerID))
			return
		}
	}
}

// processEvent processes a single referral.verified event
func (c *PositionConsumer) processEvent(ctx context.Context, event kafka.EventMessage) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "event_id", Value: event.ID},
		observability.Field{Key: "event_type", Value: event.Type},
		observability.Field{Key: "account_id", Value: event.AccountID},
	)

	// Only process referral.verified events
	if event.Type != "referral.verified" {
		c.logger.Info(ctx, fmt.Sprintf("Position consumer ignoring event type: %s", event.Type))
		return nil
	}

	c.logger.Info(ctx, "Processing referral.verified event for position recalculation")

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	campaignID, _ := uuid.Parse(data["campaign_id"].(string))
	pointsPerReferral := int(data["points_per_referral"].(float64))

	payload := jobs.PositionRecalcJobPayload{
		CampaignID:        campaignID,
		PointsPerReferral: pointsPerReferral,
	}

	err = c.positionWorker.ProcessPositionRecalc(ctx, payload)
	if err != nil {
		c.logger.Error(ctx, "Failed to recalculate positions", err)
		return err
	}

	c.logger.Info(ctx, "Successfully recalculated positions for referral verification")
	return nil
}

// Stop stops the consumer
func (c *PositionConsumer) Stop() error {
	c.logger.Info(context.Background(), "Stopping position consumer")
	return c.kafkaConsumer.Close()
}
