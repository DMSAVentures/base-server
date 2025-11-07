package producer

import (
	"base-server/internal/clients/kafka"
	"base-server/internal/observability"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EventProducer handles publishing webhook events to Kafka
type EventProducer struct {
	kafkaProducer *kafka.Producer
	logger        *observability.Logger
}

// New creates a new EventProducer
func New(kafkaProducer *kafka.Producer, logger *observability.Logger) *EventProducer {
	return &EventProducer{
		kafkaProducer: kafkaProducer,
		logger:        logger,
	}
}

// PublishEvent publishes a webhook event to Kafka
func (p *EventProducer) PublishEvent(ctx context.Context, accountID uuid.UUID, campaignID *uuid.UUID, eventType string, data map[string]interface{}) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID},
		observability.Field{Key: "event_type", Value: eventType},
	)

	var campaignIDStr *string
	if campaignID != nil {
		str := campaignID.String()
		campaignIDStr = &str
		ctx = observability.WithFields(ctx, observability.Field{Key: "campaign_id", Value: *campaignID})
	}

	event := kafka.EventMessage{
		ID:         uuid.New().String(),
		Type:       eventType,
		AccountID:  accountID.String(),
		CampaignID: campaignIDStr,
		Data:       data,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	err := p.kafkaProducer.PublishEvent(ctx, event)
	if err != nil {
		p.logger.Error(ctx, "failed to publish event to kafka", err)
		return fmt.Errorf("failed to publish event to kafka: %w", err)
	}

	p.logger.Info(ctx, fmt.Sprintf("published %s event to kafka", eventType))
	return nil
}

// PublishEvents publishes multiple events in batch
func (p *EventProducer) PublishEvents(ctx context.Context, events []EventData) error {
	if len(events) == 0 {
		return nil
	}

	kafkaEvents := make([]kafka.EventMessage, len(events))
	for i, event := range events {
		var campaignIDStr *string
		if event.CampaignID != nil {
			str := event.CampaignID.String()
			campaignIDStr = &str
		}

		kafkaEvents[i] = kafka.EventMessage{
			ID:         uuid.New().String(),
			Type:       event.EventType,
			AccountID:  event.AccountID.String(),
			CampaignID: campaignIDStr,
			Data:       event.Data,
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		}
	}

	err := p.kafkaProducer.PublishEvents(ctx, kafkaEvents)
	if err != nil {
		p.logger.Error(ctx, "failed to publish events to kafka", err)
		return fmt.Errorf("failed to publish events to kafka: %w", err)
	}

	p.logger.Info(ctx, fmt.Sprintf("published %d events to kafka", len(events)))
	return nil
}

// EventData represents event data for batch publishing
type EventData struct {
	AccountID  uuid.UUID
	CampaignID *uuid.UUID
	EventType  string
	Data       map[string]interface{}
}
