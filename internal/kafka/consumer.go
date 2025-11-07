package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"base-server/internal/observability"

	"github.com/segmentio/kafka-go"
)

// MessageHandler is a function that processes a Kafka message
type MessageHandler func(ctx context.Context, message kafka.Message) error

// Consumer handles consuming messages from Kafka topics
type Consumer struct {
	reader  *kafka.Reader
	logger  *observability.Logger
	handler MessageHandler
	dlqProducer *Producer
}

// ConsumerConfig holds configuration for the Kafka consumer
type ConsumerConfig struct {
	Brokers       []string
	Topic         string
	GroupID       string
	MinBytes      int // Minimum bytes to fetch per request (default 1)
	MaxBytes      int // Maximum bytes to fetch per request (default 1MB)
	MaxWait       time.Duration // Max time to wait for MinBytes (default 10s)
	StartOffset   int64 // kafka.FirstOffset or kafka.LastOffset
	RetentionTime time.Duration // How long to keep consumer offset (default 24h)
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(config ConsumerConfig, handler MessageHandler, dlqProducer *Producer, logger *observability.Logger) *Consumer {
	minBytes := config.MinBytes
	if minBytes == 0 {
		minBytes = 1
	}

	maxBytes := config.MaxBytes
	if maxBytes == 0 {
		maxBytes = 10e6 // 10MB
	}

	maxWait := config.MaxWait
	if maxWait == 0 {
		maxWait = 10 * time.Second
	}

	startOffset := config.StartOffset
	if startOffset == 0 {
		startOffset = kafka.LastOffset // Start from latest by default
	}

	retentionTime := config.RetentionTime
	if retentionTime == 0 {
		retentionTime = 24 * time.Hour
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        config.Brokers,
		Topic:          config.Topic,
		GroupID:        config.GroupID,
		MinBytes:       minBytes,
		MaxBytes:       maxBytes,
		MaxWait:        maxWait,
		StartOffset:    startOffset,
		RetentionTime:  retentionTime,
		CommitInterval: time.Second, // Commit offsets every second
		// Session timeout - consumer is considered dead if no heartbeat in this time
		SessionTimeout: 30 * time.Second,
		// Rebalance timeout - max time for rebalance to complete
		RebalanceTimeout: 30 * time.Second,
		// Heartbeat interval
		HeartbeatInterval: 3 * time.Second,
		// Partition assignment strategy
		GroupBalancers: []kafka.GroupBalancer{
			kafka.RangeGroupBalancer{}, // Range partitioner for ordered processing
		},
	})

	return &Consumer{
		reader:  reader,
		logger:  logger,
		handler: handler,
		dlqProducer: dlqProducer,
	}
}

// Start begins consuming messages
func (c *Consumer) Start(ctx context.Context) error {
	c.logger.Info(ctx, fmt.Sprintf("starting consumer for topic %s with group %s", c.reader.Config().Topic, c.reader.Config().GroupID))

	for {
		select {
		case <-ctx.Done():
			c.logger.Info(ctx, "consumer context cancelled, shutting down")
			return ctx.Err()
		default:
			// Fetch message with context
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					c.logger.Info(ctx, "consumer stopped")
					return nil
				}
				c.logger.Error(ctx, "error fetching message", err)
				time.Sleep(time.Second) // Back off on error
				continue
			}

			// Process message
			if err := c.processMessage(ctx, msg); err != nil {
				c.logger.Error(ctx, fmt.Sprintf("error processing message from topic %s", c.reader.Config().Topic), err)

				// Send to DLQ if configured
				if c.dlqProducer != nil {
					if dlqErr := c.sendToDLQ(ctx, msg, err); dlqErr != nil {
						c.logger.Error(ctx, "failed to send message to DLQ", dlqErr)
					}
				}

				// Still commit the offset to avoid reprocessing
				if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
					c.logger.Error(ctx, "failed to commit offset after error", commitErr)
				}
				continue
			}

			// Commit offset on success
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.logger.Error(ctx, "failed to commit offset", err)
			}
		}
	}
}

// processMessage processes a single message with retry logic
func (c *Consumer) processMessage(ctx context.Context, msg kafka.Message) error {
	// Add message metadata to context
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "topic", Value: msg.Topic},
		observability.Field{Key: "partition", Value: msg.Partition},
		observability.Field{Key: "offset", Value: msg.Offset},
		observability.Field{Key: "key", Value: string(msg.Key)},
	)

	c.logger.Info(ctx, fmt.Sprintf("processing message from topic %s, partition %d, offset %d",
		msg.Topic, msg.Partition, msg.Offset))

	// Call the handler
	start := time.Now()
	err := c.handler(ctx, msg)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error(ctx, fmt.Sprintf("handler failed after %v", duration), err)
		return err
	}

	c.logger.Info(ctx, fmt.Sprintf("message processed successfully in %v", duration))
	return nil
}

// sendToDLQ sends a failed message to the dead letter queue
func (c *Consumer) sendToDLQ(ctx context.Context, msg kafka.Message, processingErr error) error {
	// Build DLQ message with original message and error details
	dlqMessage := map[string]interface{}{
		"original_topic":     msg.Topic,
		"original_partition": msg.Partition,
		"original_offset":    msg.Offset,
		"original_key":       string(msg.Key),
		"original_value":     string(msg.Value),
		"original_headers":   headersToMap(msg.Headers),
		"original_timestamp": msg.Time,
		"error":              processingErr.Error(),
		"failed_at":          time.Now(),
	}

	return c.dlqProducer.ProduceMessage(ctx, Message{
		Key:   string(msg.Key),
		Value: dlqMessage,
		Headers: map[string]string{
			"original_topic": msg.Topic,
			"error":          processingErr.Error(),
		},
		Timestamp: time.Now(),
	})
}

// Close closes the consumer
func (c *Consumer) Close() error {
	return c.reader.Close()
}

// Stats returns consumer statistics
func (c *Consumer) Stats() kafka.ReaderStats {
	return c.reader.Stats()
}

// headersToMap converts Kafka headers to a map
func headersToMap(headers []kafka.Header) map[string]string {
	result := make(map[string]string, len(headers))
	for _, h := range headers {
		result[h.Key] = string(h.Value)
	}
	return result
}

// UnmarshalMessage unmarshals a Kafka message value into a struct
func UnmarshalMessage(msg kafka.Message, v interface{}) error {
	return json.Unmarshal(msg.Value, v)
}
