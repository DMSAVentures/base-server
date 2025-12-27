package service

import (
	"base-server/internal/integrations"
	"base-server/internal/observability"
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// IntegrationService orchestrates event delivery to different integration types
type IntegrationService struct {
	deliverers map[integrations.IntegrationType]integrations.Deliverer
	store      integrations.IntegrationStore
	logger     *observability.Logger
	mu         sync.RWMutex
}

// New creates a new IntegrationService
func New(store integrations.IntegrationStore, logger *observability.Logger) *IntegrationService {
	return &IntegrationService{
		deliverers: make(map[integrations.IntegrationType]integrations.Deliverer),
		store:      store,
		logger:     logger,
	}
}

// Register adds a deliverer for a specific integration type
func (s *IntegrationService) Register(d integrations.Deliverer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliverers[d.Type()] = d
	ctx := observability.WithFields(context.Background(),
		observability.Field{Key: "integration_type", Value: string(d.Type())})
	s.logger.Info(ctx, "registered integration deliverer")
}

// DeliverEvent delivers an event to all matching subscriptions
func (s *IntegrationService) DeliverEvent(ctx context.Context, event integrations.Event) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "event_id", Value: event.ID},
		observability.Field{Key: "event_type", Value: event.Type},
		observability.Field{Key: "account_id", Value: event.AccountID.String()},
	)

	// Get all active subscriptions for this event
	subs, err := s.store.GetActiveIntegrationSubscriptionsForEvent(ctx, event.AccountID, event.Type, event.CampaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to get active subscriptions for event", err)
		return err
	}

	if len(subs) == 0 {
		return nil
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "subscription_count", Value: len(subs)})
	s.logger.Info(ctx, "delivering event to integrations")

	// Deliver to each subscription asynchronously
	for _, sub := range subs {
		go s.deliverToSubscription(ctx, sub, event)
	}

	return nil
}

// deliverToSubscription delivers an event to a single subscription
func (s *IntegrationService) deliverToSubscription(ctx context.Context, sub integrations.Subscription, event integrations.Event) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "subscription_id", Value: sub.ID.String()},
		observability.Field{Key: "integration_type", Value: string(sub.IntegrationType)},
	)

	s.mu.RLock()
	deliverer, ok := s.deliverers[sub.IntegrationType]
	s.mu.RUnlock()

	if !ok {
		s.logger.Warn(ctx, "no deliverer registered for integration type")
		return
	}

	// Create delivery record
	delivery, err := s.store.CreateDelivery(ctx, integrations.CreateDeliveryParams{
		SubscriptionID: sub.ID,
		EventType:      event.Type,
		Status:         integrations.DeliveryStatusPending,
	})
	if err != nil {
		s.logger.Error(ctx, "failed to create delivery record", err)
		return
	}

	// Measure delivery duration
	startTime := time.Now()

	// Attempt delivery
	err = deliverer.Deliver(ctx, sub, event)
	durationMs := int(time.Since(startTime).Milliseconds())

	if err != nil {
		ctx = observability.WithFields(ctx,
			observability.Field{Key: "duration_ms", Value: durationMs})
		s.logger.Error(ctx, "failed to deliver event to integration", err)

		errorMsg := err.Error()
		s.store.UpdateDeliveryStatus(ctx, delivery.ID, integrations.DeliveryStatusFailed, nil, &durationMs, &errorMsg)
		s.store.UpdateIntegrationSubscriptionStats(ctx, sub.ID, false, &errorMsg)
		return
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "duration_ms", Value: durationMs})
	s.logger.Info(ctx, "successfully delivered event to integration")

	successStatus := 200
	s.store.UpdateDeliveryStatus(ctx, delivery.ID, integrations.DeliveryStatusSuccess, &successStatus, &durationMs, nil)
	s.store.UpdateIntegrationSubscriptionStats(ctx, sub.ID, true, nil)
}

// GetDeliverer returns the deliverer for a specific integration type
func (s *IntegrationService) GetDeliverer(integrationType integrations.IntegrationType) (integrations.Deliverer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.deliverers[integrationType]
	return d, ok
}

// Deliver delivers an event to a specific subscription (used by consumer)
func (s *IntegrationService) Deliver(ctx context.Context, sub integrations.Subscription, event integrations.Event) {
	s.deliverToSubscription(ctx, sub, event)
}

// GetSubscriptionsByAccount returns all subscriptions for an account
func (s *IntegrationService) GetSubscriptionsByAccount(ctx context.Context, accountID uuid.UUID, integrationType *integrations.IntegrationType) ([]integrations.Subscription, error) {
	return s.store.GetIntegrationSubscriptionsByAccount(ctx, accountID, integrationType)
}

// GetDeliveriesByAccount returns recent deliveries for an account
func (s *IntegrationService) GetDeliveriesByAccount(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]integrations.Delivery, error) {
	return s.store.GetDeliveriesByAccount(ctx, accountID, limit, offset)
}
