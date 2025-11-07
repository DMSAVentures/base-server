package kafka

import (
	"base-server/internal/observability"
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
)

// Consumer handles consuming events from Kafka
type Consumer struct {
	reader *kafka.Reader
	logger *observability.Logger
}

// ConsumerConfig contains configuration for Kafka consumer
type ConsumerConfig struct {
	Brokers  []string
	Topic    string
	GroupID  string
	MinBytes int
	MaxBytes int
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(config ConsumerConfig, logger *observability.Logger) *Consumer {
	// Set defaults
	if config.MinBytes == 0 {
		config.MinBytes = 10e3 // 10KB
	}
	if config.MaxBytes == 0 {
		config.MaxBytes = 10e6 // 10MB
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  config.Brokers,
		Topic:    config.Topic,
		GroupID:  config.GroupID,
		MinBytes: config.MinBytes,
		MaxBytes: config.MaxBytes,
		// Start reading from the earliest message if no offset exists
		StartOffset: kafka.FirstOffset,
		// Commit interval
		CommitInterval: 0, // Manual commit
	})

	return &Consumer{
		reader: reader,
		logger: logger,
	}
}

// ConsumeEvents continuously consumes events and processes them
func (c *Consumer) ConsumeEvents(ctx context.Context, handler func(context.Context, EventMessage) error) error {
	c.logger.Info(ctx, "Starting Kafka consumer")

	for {
		select {
		case <-ctx.Done():
			c.logger.Info(ctx, "Stopping Kafka consumer")
			return ctx.Err()
		default:
			// Read message from Kafka
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				c.logger.Error(ctx, "failed to fetch message from kafka", err)
				continue
			}

			// Parse event
			var event EventMessage
			err = json.Unmarshal(msg.Value, &event)
			if err != nil {
				c.logger.Error(ctx, "failed to unmarshal event", err)
				// Commit even on error to skip bad messages
				c.reader.CommitMessages(ctx, msg)
				continue
			}

			msgCtx := observability.WithFields(ctx,
				observability.Field{Key: "event_type", Value: event.Type},
				observability.Field{Key: "event_id", Value: event.ID},
				observability.Field{Key: "partition", Value: msg.Partition},
				observability.Field{Key: "offset", Value: msg.Offset},
			)

			c.logger.Info(msgCtx, fmt.Sprintf("processing event %s", event.Type))

			// Process event
			err = handler(msgCtx, event)
			if err != nil {
				c.logger.Error(msgCtx, "failed to process event", err)
				// Don't commit on processing error - will retry
				continue
			}

			// Commit message after successful processing
			err = c.reader.CommitMessages(msgCtx, msg)
			if err != nil {
				c.logger.Error(msgCtx, "failed to commit message", err)
			}

			c.logger.Info(msgCtx, fmt.Sprintf("successfully processed event %s", event.Type))
		}
	}
}

// Close closes the Kafka consumer
func (c *Consumer) Close() error {
	return c.reader.Close()
}
