package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"base-server/internal/observability"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// Producer handles producing messages to Kafka topics
type Producer struct {
	writer *kafka.Writer
	logger *observability.Logger
}

// ProducerConfig holds configuration for the Kafka producer
type ProducerConfig struct {
	Brokers []string
	Topic   string
	// Compression can be: none, gzip, snappy, lz4, zstd
	Compression string
	// BatchSize is the max number of messages to batch together
	BatchSize int
	// BatchTimeout is the max time to wait before sending a batch
	BatchTimeout time.Duration
	// RequiredAcks determines the durability guarantee
	// -1 = all replicas must acknowledge
	//  0 = no acknowledgment
	//  1 = only leader must acknowledge
	RequiredAcks int
}

// NewProducer creates a new Kafka producer
func NewProducer(config ProducerConfig, logger *observability.Logger) *Producer {
	compression := kafka.Compression(0) // no compression by default
	switch config.Compression {
	case "gzip":
		compression = kafka.Gzip
	case "snappy":
		compression = kafka.Snappy
	case "lz4":
		compression = kafka.Lz4
	case "zstd":
		compression = kafka.Zstd
	}

	batchSize := config.BatchSize
	if batchSize == 0 {
		batchSize = 100
	}

	batchTimeout := config.BatchTimeout
	if batchTimeout == 0 {
		batchTimeout = 10 * time.Millisecond
	}

	requiredAcks := config.RequiredAcks
	if requiredAcks == 0 {
		requiredAcks = -1 // All replicas by default for durability
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(config.Brokers...),
		Topic:        config.Topic,
		Balancer:     &kafka.Hash{}, // Use hash partitioner for message ordering by key
		Compression:  compression,
		BatchSize:    batchSize,
		BatchTimeout: batchTimeout,
		RequiredAcks: kafka.RequiredAcks(requiredAcks),
		// Async writes with error logging
		Async: false, // Synchronous for guaranteed delivery
	}

	return &Producer{
		writer: writer,
		logger: logger,
	}
}

// Message represents a Kafka message
type Message struct {
	Key       string                 // Used for partitioning
	Value     interface{}            // Will be JSON encoded
	Headers   map[string]string      // Message headers
	Timestamp time.Time              // Message timestamp
	Metadata  map[string]interface{} // Additional metadata
}

// ProduceMessage sends a message to the configured topic
func (p *Producer) ProduceMessage(ctx context.Context, msg Message) error {
	// JSON encode the value
	valueBytes, err := json.Marshal(msg.Value)
	if err != nil {
		p.logger.Error(ctx, "failed to marshal message value", err)
		return fmt.Errorf("failed to marshal message value: %w", err)
	}

	// Build headers
	headers := make([]kafka.Header, 0, len(msg.Headers)+3)
	for k, v := range msg.Headers {
		headers = append(headers, kafka.Header{
			Key:   k,
			Value: []byte(v),
		})
	}

	// Add standard headers
	headers = append(headers,
		kafka.Header{Key: "message_id", Value: []byte(uuid.New().String())},
		kafka.Header{Key: "produced_at", Value: []byte(time.Now().Format(time.RFC3339))},
		kafka.Header{Key: "producer", Value: []byte("base-server")},
	)

	// Create Kafka message
	kafkaMsg := kafka.Message{
		Key:     []byte(msg.Key),
		Value:   valueBytes,
		Headers: headers,
	}

	if !msg.Timestamp.IsZero() {
		kafkaMsg.Time = msg.Timestamp
	} else {
		kafkaMsg.Time = time.Now()
	}

	// Write message
	err = p.writer.WriteMessages(ctx, kafkaMsg)
	if err != nil {
		p.logger.Error(ctx, fmt.Sprintf("failed to write message to topic %s", p.writer.Topic), err)
		return fmt.Errorf("failed to write message to topic %s: %w", p.writer.Topic, err)
	}

	p.logger.Info(ctx, fmt.Sprintf("produced message to topic %s with key %s", p.writer.Topic, msg.Key))
	return nil
}

// ProduceBatch sends multiple messages in a single batch
func (p *Producer) ProduceBatch(ctx context.Context, messages []Message) error {
	if len(messages) == 0 {
		return nil
	}

	kafkaMessages := make([]kafka.Message, 0, len(messages))

	for _, msg := range messages {
		// JSON encode the value
		valueBytes, err := json.Marshal(msg.Value)
		if err != nil {
			p.logger.Error(ctx, "failed to marshal message value in batch", err)
			continue
		}

		// Build headers
		headers := make([]kafka.Header, 0, len(msg.Headers)+3)
		for k, v := range msg.Headers {
			headers = append(headers, kafka.Header{
				Key:   k,
				Value: []byte(v),
			})
		}

		headers = append(headers,
			kafka.Header{Key: "message_id", Value: []byte(uuid.New().String())},
			kafka.Header{Key: "produced_at", Value: []byte(time.Now().Format(time.RFC3339))},
			kafka.Header{Key: "producer", Value: []byte("base-server")},
		)

		kafkaMsg := kafka.Message{
			Key:     []byte(msg.Key),
			Value:   valueBytes,
			Headers: headers,
		}

		if !msg.Timestamp.IsZero() {
			kafkaMsg.Time = msg.Timestamp
		} else {
			kafkaMsg.Time = time.Now()
		}

		kafkaMessages = append(kafkaMessages, kafkaMsg)
	}

	// Write batch
	err := p.writer.WriteMessages(ctx, kafkaMessages...)
	if err != nil {
		p.logger.Error(ctx, fmt.Sprintf("failed to write batch to topic %s", p.writer.Topic), err)
		return fmt.Errorf("failed to write batch to topic %s: %w", p.writer.Topic, err)
	}

	p.logger.Info(ctx, fmt.Sprintf("produced batch of %d messages to topic %s", len(kafkaMessages), p.writer.Topic))
	return nil
}

// Close closes the producer
func (p *Producer) Close() error {
	return p.writer.Close()
}

// Stats returns writer statistics
func (p *Producer) Stats() kafka.WriterStats {
	return p.writer.Stats()
}
