package jobs

import (
	"context"
	"fmt"

	"base-server/internal/kafka"
	"base-server/internal/observability"
)

// KafkaClient handles enqueueing background jobs to Kafka
type KafkaClient struct {
	producers map[string]*kafka.Producer
	logger    *observability.Logger
	brokers   []string
}

// NewKafkaClient creates a new Kafka-based job client
func NewKafkaClient(brokers []string, logger *observability.Logger) *KafkaClient {
	return &KafkaClient{
		producers: make(map[string]*kafka.Producer),
		logger:    logger,
		brokers:   brokers,
	}
}

// getOrCreateProducer gets or creates a producer for a specific topic
func (c *KafkaClient) getOrCreateProducer(topic string) *kafka.Producer {
	if producer, exists := c.producers[topic]; exists {
		return producer
	}

	producer := kafka.NewProducer(kafka.ProducerConfig{
		Brokers:      c.brokers,
		Topic:        topic,
		Compression:  "snappy", // Good balance of compression and speed
		BatchSize:    100,
		BatchTimeout: 10, // 10ms
		RequiredAcks: -1, // All replicas must acknowledge for durability
	}, c.logger)

	c.producers[topic] = producer
	return producer
}

// Close closes all producer connections
func (c *KafkaClient) Close() error {
	for _, producer := range c.producers {
		if err := producer.Close(); err != nil {
			c.logger.Error(context.Background(), "failed to close producer", err)
		}
	}
	return nil
}

// EnqueueEmailJob enqueues an email job to Kafka
func (c *KafkaClient) EnqueueEmailJob(ctx context.Context, payload EmailJobPayload) error {
	// Determine topic based on email type
	topic := c.getEmailTopic(payload.Type)
	producer := c.getOrCreateProducer(topic)

	// Use campaign_id + user_id as key for message ordering
	key := fmt.Sprintf("%s_%s", payload.CampaignID.String(), payload.UserID.String())

	err := producer.ProduceMessage(ctx, kafka.Message{
		Key:   key,
		Value: payload,
		Headers: map[string]string{
			"job_type":    "email",
			"email_type":  payload.Type,
			"campaign_id": payload.CampaignID.String(),
			"user_id":     payload.UserID.String(),
		},
	})

	if err != nil {
		c.logger.Error(ctx, "failed to enqueue email job", err)
		return fmt.Errorf("failed to enqueue email job: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("enqueued email job (%s) to topic %s", payload.Type, topic))
	return nil
}

// EnqueuePositionRecalcJob enqueues a position recalculation job
func (c *KafkaClient) EnqueuePositionRecalcJob(ctx context.Context, payload PositionRecalcJobPayload) error {
	producer := c.getOrCreateProducer(kafka.TopicPositionRecalc)

	// Use campaign_id as key for message ordering
	key := payload.CampaignID.String()

	err := producer.ProduceMessage(ctx, kafka.Message{
		Key:   key,
		Value: payload,
		Headers: map[string]string{
			"job_type":    "position_recalc",
			"campaign_id": payload.CampaignID.String(),
		},
	})

	if err != nil {
		c.logger.Error(ctx, "failed to enqueue position recalc job", err)
		return fmt.Errorf("failed to enqueue position recalc job: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("enqueued position recalc job to topic %s", kafka.TopicPositionRecalc))
	return nil
}

// EnqueueRewardDeliveryJob enqueues a reward delivery job
func (c *KafkaClient) EnqueueRewardDeliveryJob(ctx context.Context, payload RewardDeliveryJobPayload) error {
	producer := c.getOrCreateProducer(kafka.TopicRewardDelivery)

	// Use user_reward_id as key
	key := payload.UserRewardID.String()

	err := producer.ProduceMessage(ctx, kafka.Message{
		Key:   key,
		Value: payload,
		Headers: map[string]string{
			"job_type":       "reward_delivery",
			"user_reward_id": payload.UserRewardID.String(),
		},
	})

	if err != nil {
		c.logger.Error(ctx, "failed to enqueue reward delivery job", err)
		return fmt.Errorf("failed to enqueue reward delivery job: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("enqueued reward delivery job to topic %s", kafka.TopicRewardDelivery))
	return nil
}

// EnqueueWebhookDeliveryJob enqueues a webhook delivery job
func (c *KafkaClient) EnqueueWebhookDeliveryJob(ctx context.Context, payload WebhookDeliveryJobPayload) error {
	producer := c.getOrCreateProducer(kafka.TopicWebhookDelivery)

	// Use webhook_id as key
	key := payload.WebhookID.String()

	err := producer.ProduceMessage(ctx, kafka.Message{
		Key:   key,
		Value: payload,
		Headers: map[string]string{
			"job_type":    "webhook_delivery",
			"webhook_id":  payload.WebhookID.String(),
			"event_type":  payload.EventType,
		},
	})

	if err != nil {
		c.logger.Error(ctx, "failed to enqueue webhook delivery job", err)
		return fmt.Errorf("failed to enqueue webhook delivery job: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("enqueued webhook delivery job to topic %s", kafka.TopicWebhookDelivery))
	return nil
}

// EnqueueFraudDetectionJob enqueues a fraud detection job
func (c *KafkaClient) EnqueueFraudDetectionJob(ctx context.Context, payload FraudDetectionJobPayload) error {
	producer := c.getOrCreateProducer(kafka.TopicFraudDetection)

	// Use campaign_id + user_id as key
	key := fmt.Sprintf("%s_%s", payload.CampaignID.String(), payload.UserID.String())

	err := producer.ProduceMessage(ctx, kafka.Message{
		Key:   key,
		Value: payload,
		Headers: map[string]string{
			"job_type":    "fraud_detection",
			"campaign_id": payload.CampaignID.String(),
			"user_id":     payload.UserID.String(),
		},
	})

	if err != nil {
		c.logger.Error(ctx, "failed to enqueue fraud detection job", err)
		return fmt.Errorf("failed to enqueue fraud detection job: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("enqueued fraud detection job to topic %s", kafka.TopicFraudDetection))
	return nil
}

// EnqueueAnalyticsAggregationJob enqueues an analytics aggregation job
func (c *KafkaClient) EnqueueAnalyticsAggregationJob(ctx context.Context, payload AnalyticsAggregationJobPayload) error {
	producer := c.getOrCreateProducer(kafka.TopicAnalyticsAggregation)

	// Use campaign_id + start_time as key
	key := fmt.Sprintf("%s_%d", payload.CampaignID.String(), payload.StartTime.Unix())

	err := producer.ProduceMessage(ctx, kafka.Message{
		Key:   key,
		Value: payload,
		Headers: map[string]string{
			"job_type":    "analytics_aggregation",
			"campaign_id": payload.CampaignID.String(),
			"granularity": payload.Granularity,
		},
	})

	if err != nil {
		c.logger.Error(ctx, "failed to enqueue analytics aggregation job", err)
		return fmt.Errorf("failed to enqueue analytics aggregation job: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("enqueued analytics aggregation job to topic %s", kafka.TopicAnalyticsAggregation))
	return nil
}

// getEmailTopic returns the appropriate Kafka topic for an email type
func (c *KafkaClient) getEmailTopic(emailType string) string {
	switch emailType {
	case "verification":
		return kafka.TopicEmailVerification
	case "welcome":
		return kafka.TopicEmailWelcome
	case "position_update":
		return kafka.TopicEmailPositionUpdate
	case "reward_earned":
		return kafka.TopicEmailRewardEarned
	case "milestone":
		return kafka.TopicEmailMilestone
	default:
		return kafka.TopicEmailVerification
	}
}

// PublishEvent publishes an event to event sourcing topics for audit trail
func (c *KafkaClient) PublishEvent(ctx context.Context, topic string, key string, event interface{}) error {
	producer := c.getOrCreateProducer(topic)

	err := producer.ProduceMessage(ctx, kafka.Message{
		Key:   key,
		Value: event,
		Headers: map[string]string{
			"event_type": topic,
		},
	})

	if err != nil {
		c.logger.Error(ctx, "failed to publish event", err)
		return fmt.Errorf("failed to publish event: %w", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("published event to topic %s", topic))
	return nil
}
