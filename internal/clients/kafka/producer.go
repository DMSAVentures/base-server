package kafka

import (
	"base-server/internal/observability"
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
)

// Producer handles publishing events to Kafka
type Producer struct {
	writer *kafka.Writer
	logger *observability.Logger
}

// ProducerConfig contains configuration for Kafka producer
type ProducerConfig struct {
	Brokers []string
	Topic   string
}

// NewProducer creates a new Kafka producer
func NewProducer(config ProducerConfig, logger *observability.Logger) *Producer {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(config.Brokers...),
		Topic:    config.Topic,
		Balancer: &kafka.LeastBytes{},
		// Async writes for better performance
		Async: false,
		// Compression for better throughput
		Compression: kafka.Snappy,
		// Batching for efficiency
		BatchSize: 100,
	}

	return &Producer{
		writer: writer,
		logger: logger,
	}
}

// EventMessage represents an event message structure
type EventMessage struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	AccountID  string                 `json:"account_id"`
	CampaignID *string                `json:"campaign_id,omitempty"`
	Data       map[string]interface{} `json:"data"`
	Timestamp  string                 `json:"timestamp"`
}

// PublishEvent publishes an event to Kafka
func (p *Producer) PublishEvent(ctx context.Context, event EventMessage) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "event_type", Value: event.Type},
		observability.Field{Key: "event_id", Value: event.ID},
	)

	// Serialize event to JSON
	eventBytes, err := json.Marshal(event)
	if err != nil {
		p.logger.Error(ctx, "failed to marshal event", err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create Kafka message
	msg := kafka.Message{
		Key:   []byte(event.AccountID), // Partition by account ID for ordering
		Value: eventBytes,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.Type)},
			{Key: "account_id", Value: []byte(event.AccountID)},
		},
	}

	// Write to Kafka
	err = p.writer.WriteMessages(ctx, msg)
	if err != nil {
		p.logger.Error(ctx, "failed to write message to kafka", err)
		return fmt.Errorf("failed to write message to kafka: %w", err)
	}

	p.logger.Info(ctx, fmt.Sprintf("published event %s to kafka", event.Type))
	return nil
}

// PublishEvents publishes multiple events in batch
func (p *Producer) PublishEvents(ctx context.Context, events []EventMessage) error {
	if len(events) == 0 {
		return nil
	}

	messages := make([]kafka.Message, len(events))
	for i, event := range events {
		eventBytes, err := json.Marshal(event)
		if err != nil {
			p.logger.Error(ctx, fmt.Sprintf("failed to marshal event %s", event.ID), err)
			continue
		}

		messages[i] = kafka.Message{
			Key:   []byte(event.AccountID),
			Value: eventBytes,
			Headers: []kafka.Header{
				{Key: "event_type", Value: []byte(event.Type)},
				{Key: "account_id", Value: []byte(event.AccountID)},
			},
		}
	}

	err := p.writer.WriteMessages(ctx, messages...)
	if err != nil {
		p.logger.Error(ctx, "failed to write messages to kafka", err)
		return fmt.Errorf("failed to write messages to kafka: %w", err)
	}

	p.logger.Info(ctx, fmt.Sprintf("published %d events to kafka", len(events)))
	return nil
}

// Close closes the Kafka producer
func (p *Producer) Close() error {
	return p.writer.Close()
}
