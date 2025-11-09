package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"base-server/internal/observability"

	kafkago "github.com/segmentio/kafka-go"
)

// ConsumerConfig holds configuration for the Kafka event consumer.
type ConsumerConfig struct {
	// Brokers is the list of Kafka broker addresses.
	Brokers []string

	// ConsumerGroup is the Kafka consumer group ID.
	ConsumerGroup string

	// Topic is the Kafka topic to consume from.
	Topic string

	// WorkerPool configuration
	WorkerPoolConfig WorkerPoolConfig
}

// DefaultConsumerConfig returns sensible defaults for a consumer.
func DefaultConsumerConfig(brokers []string, consumerGroup, topic string) ConsumerConfig {
	return ConsumerConfig{
		Brokers:          brokers,
		ConsumerGroup:    consumerGroup,
		Topic:            topic,
		WorkerPoolConfig: DefaultWorkerPoolConfig(),
	}
}

// consumer implements the EventConsumer interface with worker pool and proper offset management.
type consumer struct {
	config    ConsumerConfig
	reader    *kafkago.Reader
	processor EventProcessor
	pool      WorkerPool
	logger    *observability.Logger

	// Offset management for safe commits
	mu              sync.Mutex
	pendingMessages map[string]*kafkago.Message // event_id -> kafka message (not yet processed)

	// Lifecycle
	stopOnce sync.Once
	stopped  bool
}

// NewConsumer creates a new Kafka event consumer with a worker pool.
// The consumer will:
// - Fetch messages from Kafka
// - Distribute them to a pool of N workers
// - Only commit offsets for successfully processed messages
// - On failure, messages will be redelivered when consumer restarts
func NewConsumer(
	config ConsumerConfig,
	processor EventProcessor,
	logger *observability.Logger,
) EventConsumer {
	c := &consumer{
		config:          config,
		processor:       processor,
		logger:          logger,
		pendingMessages: make(map[string]*kafkago.Message),
	}

	// Create Kafka reader with manual offset commits
	c.reader = kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  config.Brokers,
		Topic:    config.Topic,
		GroupID:  config.ConsumerGroup,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
		// Start from earliest message if no committed offset exists
		StartOffset: kafkago.FirstOffset,
		// Manual commit - we'll commit after successful processing
		CommitInterval: 0,
	})

	// Configure worker pool with result callback for offset management
	config.WorkerPoolConfig.OnResult = c.handleProcessingResult
	c.pool = NewWorkerPool(config.WorkerPoolConfig, processor, logger)

	return c
}

// Start begins consuming events from Kafka and processing them with the worker pool.
func (c *consumer) Start(ctx context.Context) error {
	if c.stopped {
		return fmt.Errorf("consumer already stopped")
	}

	consumerCtx := observability.WithFields(ctx,
		observability.Field{Key: "consumer_group", Value: c.config.ConsumerGroup},
		observability.Field{Key: "topic", Value: c.config.Topic},
		observability.Field{Key: "processor", Value: c.processor.Name()},
	)

	c.logger.Info(consumerCtx, fmt.Sprintf("Starting Kafka consumer for %s processor with %d workers",
		c.processor.Name(), c.config.WorkerPoolConfig.NumWorkers))

	// Start worker pool
	if err := c.pool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	// Main consumption loop
	for {
		select {
		case <-ctx.Done():
			c.logger.Info(consumerCtx, "Consumer context cancelled")
			return ctx.Err()

		default:
			// Fetch message from Kafka
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					// Context cancelled, exit gracefully
					return ctx.Err()
				}
				c.logger.Error(consumerCtx, "Failed to fetch message from Kafka", err)
				time.Sleep(1 * time.Second) // Back off on error
				continue
			}

			// Parse event message
			var event EventMessage
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				c.logger.Error(consumerCtx, "Failed to unmarshal event, skipping", err)
				// Commit bad messages to skip them
				if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
					c.logger.Error(consumerCtx, "Failed to commit bad message", commitErr)
				}
				continue
			}

			msgCtx := observability.WithFields(consumerCtx,
				observability.Field{Key: "event_type", Value: event.Type},
				observability.Field{Key: "event_id", Value: event.ID},
				observability.Field{Key: "partition", Value: msg.Partition},
				observability.Field{Key: "offset", Value: msg.Offset},
			)

			// Track message for offset management (must happen before Submit)
			c.trackMessage(event.ID, &msg)

			// Submit to worker pool (blocks if queue is full)
			// Worker will call handleProcessingResult when done
			if err := c.pool.Submit(msgCtx, event); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				c.logger.Error(msgCtx, "Failed to submit event to worker pool", err)
				// Clean up pending message since we won't process it
				c.mu.Lock()
				delete(c.pendingMessages, event.ID)
				c.mu.Unlock()
			}
		}
	}
}

// Stop gracefully shuts down the consumer, draining in-flight events.
func (c *consumer) Stop() {
	c.stopOnce.Do(func() {
		c.stopped = true

		ctx := context.Background()
		ctx = observability.WithFields(ctx,
			observability.Field{Key: "processor", Value: c.processor.Name()},
		)

		c.logger.Info(ctx, fmt.Sprintf("Stopping consumer for %s processor",
			c.processor.Name()))

		// Drain worker pool (wait for in-flight events)
		drainCtx, cancel := context.WithTimeout(ctx, c.config.WorkerPoolConfig.DrainTimeout)
		defer cancel()

		if err := c.pool.Drain(drainCtx); err != nil {
			c.logger.Error(ctx, "Failed to drain worker pool gracefully", err)
			c.pool.Stop() // Force stop
		}

		// Close Kafka reader
		if err := c.reader.Close(); err != nil {
			c.logger.Error(ctx, "Failed to close Kafka reader", err)
		}

		c.logger.Info(ctx, fmt.Sprintf("Consumer stopped for %s processor",
			c.processor.Name()))
	})
}

// trackMessage tracks a message that's been submitted for processing.
func (c *consumer) trackMessage(eventID string, msg *kafkago.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pendingMessages[eventID] = msg
}

// handleProcessingResult is called by workers after processing each event.
// This handles offset commits - only committing on success to enable replay on failure.
func (c *consumer) handleProcessingResult(result ProcessingResult) {
	c.mu.Lock()
	msg, exists := c.pendingMessages[result.Event.ID]
	c.mu.Unlock()

	if !exists {
		c.logger.Warn(context.Background(), fmt.Sprintf("Received result for unknown event %s", result.Event.ID))
		return
	}

	ctx := observability.WithFields(context.Background(),
		observability.Field{Key: "event_id", Value: result.Event.ID},
		observability.Field{Key: "event_type", Value: result.Event.Type},
		observability.Field{Key: "partition", Value: msg.Partition},
		observability.Field{Key: "offset", Value: msg.Offset},
	)

	if result.Error != nil {
		// Processing failed - DO NOT commit offset
		// Message will be redelivered when consumer restarts
		c.logger.Error(ctx, "Event processing failed, offset NOT committed (will replay on restart)", result.Error)
	} else {
		// Processing succeeded - commit offset
		if err := c.reader.CommitMessages(ctx, *msg); err != nil {
			c.logger.Error(ctx, "Failed to commit offset", err)
		} else {
			c.logger.Info(ctx, "Successfully processed event and committed offset")
		}
	}

	// Remove from pending
	c.mu.Lock()
	delete(c.pendingMessages, result.Event.ID)
	c.mu.Unlock()
}
