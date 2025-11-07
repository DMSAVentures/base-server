package consumer

import (
	"base-server/internal/clients/kafka"
	"base-server/internal/observability"
	"base-server/internal/webhooks/service"
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// EventConsumer handles consuming webhook events from Kafka
type EventConsumer struct {
	kafkaConsumer  *kafka.Consumer
	webhookService *service.WebhookService
	logger         *observability.Logger
	workerCount    int
}

// New creates a new EventConsumer
func New(kafkaConsumer *kafka.Consumer, webhookService *service.WebhookService, logger *observability.Logger, workerCount int) *EventConsumer {
	if workerCount == 0 {
		workerCount = 10 // Default to 10 workers
	}

	return &EventConsumer{
		kafkaConsumer:  kafkaConsumer,
		webhookService: webhookService,
		logger:         logger,
		workerCount:    workerCount,
	}
}

// Start starts consuming events from Kafka with multiple workers
func (c *EventConsumer) Start(ctx context.Context) error {
	c.logger.Info(ctx, fmt.Sprintf("Starting webhook event consumer with %d workers", c.workerCount))

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
func (c *EventConsumer) worker(ctx context.Context, workerID int, eventChan <-chan kafka.EventMessage) {
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

// processEvent processes a single event by dispatching webhooks
func (c *EventConsumer) processEvent(ctx context.Context, event kafka.EventMessage) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "event_id", Value: event.ID},
		observability.Field{Key: "event_type", Value: event.Type},
		observability.Field{Key: "account_id", Value: event.AccountID},
	)

	c.logger.Info(ctx, fmt.Sprintf("Processing event %s", event.Type))

	// Parse account ID
	accountID, err := uuid.Parse(event.AccountID)
	if err != nil {
		c.logger.Error(ctx, "invalid account_id", err)
		return fmt.Errorf("invalid account_id: %w", err)
	}

	// Parse campaign ID if present
	var campaignID *uuid.UUID
	if event.CampaignID != nil {
		parsed, err := uuid.Parse(*event.CampaignID)
		if err != nil {
			c.logger.Error(ctx, "invalid campaign_id", err)
			return fmt.Errorf("invalid campaign_id: %w", err)
		}
		campaignID = &parsed
	}

	// Dispatch to webhooks
	err = c.webhookService.DispatchEvent(ctx, accountID, campaignID, event.Type, event.Data)
	if err != nil {
		c.logger.Error(ctx, "failed to dispatch event to webhooks", err)
		return fmt.Errorf("failed to dispatch event to webhooks: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("Successfully processed event %s", event.Type))
	return nil
}

// Stop stops the consumer
func (c *EventConsumer) Stop() error {
	c.logger.Info(context.Background(), "Stopping webhook event consumer")
	return c.kafkaConsumer.Close()
}
