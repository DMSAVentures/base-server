package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"base-server/internal/integrations"

	"github.com/google/uuid"
)

// IntegrationSubscription represents a subscription in the database
type IntegrationSubscription struct {
	ID              uuid.UUID  `db:"id"`
	AccountID       uuid.UUID  `db:"account_id"`
	APIKeyID        *uuid.UUID `db:"api_key_id"`
	IntegrationType string     `db:"integration_type"`
	TargetURL       string     `db:"target_url"`
	EventType       string     `db:"event_type"`
	CampaignID      *uuid.UUID `db:"campaign_id"`
	Config          JSONB      `db:"config"`
	Status          string     `db:"status"`
	LastTriggeredAt *time.Time `db:"last_triggered_at"`
	TriggerCount    int        `db:"trigger_count"`
	ErrorCount      int        `db:"error_count"`
	LastError       *string    `db:"last_error"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
	DeletedAt       *time.Time `db:"deleted_at"`
}

// IntegrationDelivery represents a delivery record in the database
type IntegrationDelivery struct {
	ID             uuid.UUID `db:"id"`
	SubscriptionID uuid.UUID `db:"subscription_id"`
	EventType      string    `db:"event_type"`
	Status         string    `db:"status"`
	ResponseStatus *int      `db:"response_status"`
	DurationMs     *int      `db:"duration_ms"`
	ErrorMessage   *string   `db:"error_message"`
	CreatedAt      time.Time `db:"created_at"`
}

// ===== Integration Subscription Methods =====

const sqlCreateIntegrationSubscription = `
INSERT INTO integration_subscriptions (account_id, api_key_id, integration_type, target_url, event_type, campaign_id, config)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, account_id, api_key_id, integration_type, target_url, event_type, campaign_id, config, status, last_triggered_at, trigger_count, error_count, last_error, created_at, updated_at, deleted_at
`

// CreateIntegrationSubscription creates a new integration subscription
func (s *Store) CreateIntegrationSubscription(ctx context.Context, params integrations.CreateSubscriptionParams) (integrations.Subscription, error) {
	var sub IntegrationSubscription
	err := s.db.GetContext(ctx, &sub, sqlCreateIntegrationSubscription,
		params.AccountID,
		params.APIKeyID,
		string(params.IntegrationType),
		params.TargetURL,
		params.EventType,
		params.CampaignID,
		JSONB(params.Config),
	)
	if err != nil {
		return integrations.Subscription{}, fmt.Errorf("failed to create subscription: %w", err)
	}
	return toIntegrationSubscription(sub), nil
}

const sqlGetIntegrationSubscriptionByID = `
SELECT id, account_id, api_key_id, integration_type, target_url, event_type, campaign_id, config, status, last_triggered_at, trigger_count, error_count, last_error, created_at, updated_at, deleted_at
FROM integration_subscriptions
WHERE id = $1 AND deleted_at IS NULL
`

// GetIntegrationSubscriptionByID retrieves a subscription by ID
func (s *Store) GetIntegrationSubscriptionByID(ctx context.Context, subscriptionID uuid.UUID) (integrations.Subscription, error) {
	var sub IntegrationSubscription
	err := s.db.GetContext(ctx, &sub, sqlGetIntegrationSubscriptionByID, subscriptionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return integrations.Subscription{}, ErrNotFound
		}
		return integrations.Subscription{}, fmt.Errorf("failed to get subscription: %w", err)
	}
	return toIntegrationSubscription(sub), nil
}

const sqlGetIntegrationSubscriptionsByAccount = `
SELECT id, account_id, api_key_id, integration_type, target_url, event_type, campaign_id, config, status, last_triggered_at, trigger_count, error_count, last_error, created_at, updated_at, deleted_at
FROM integration_subscriptions
WHERE account_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
`

const sqlGetIntegrationSubscriptionsByAccountAndType = `
SELECT id, account_id, api_key_id, integration_type, target_url, event_type, campaign_id, config, status, last_triggered_at, trigger_count, error_count, last_error, created_at, updated_at, deleted_at
FROM integration_subscriptions
WHERE account_id = $1 AND integration_type = $2 AND deleted_at IS NULL
ORDER BY created_at DESC
`

// GetIntegrationSubscriptionsByAccount retrieves all subscriptions for an account
func (s *Store) GetIntegrationSubscriptionsByAccount(ctx context.Context, accountID uuid.UUID, integrationType *integrations.IntegrationType) ([]integrations.Subscription, error) {
	var subs []IntegrationSubscription
	var err error

	if integrationType != nil {
		err = s.db.SelectContext(ctx, &subs, sqlGetIntegrationSubscriptionsByAccountAndType, accountID, string(*integrationType))
	} else {
		err = s.db.SelectContext(ctx, &subs, sqlGetIntegrationSubscriptionsByAccount, accountID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get subscriptions: %w", err)
	}

	result := make([]integrations.Subscription, len(subs))
	for i, sub := range subs {
		result[i] = toIntegrationSubscription(sub)
	}
	return result, nil
}

const sqlGetActiveIntegrationSubscriptionsForEvent = `
SELECT id, account_id, api_key_id, integration_type, target_url, event_type, campaign_id, config, status, last_triggered_at, trigger_count, error_count, last_error, created_at, updated_at, deleted_at
FROM integration_subscriptions
WHERE account_id = $1
  AND event_type = $2
  AND status = 'active'
  AND deleted_at IS NULL
  AND (campaign_id IS NULL OR campaign_id = $3)
`

const sqlGetActiveIntegrationSubscriptionsForEventNoCampaign = `
SELECT id, account_id, api_key_id, integration_type, target_url, event_type, campaign_id, config, status, last_triggered_at, trigger_count, error_count, last_error, created_at, updated_at, deleted_at
FROM integration_subscriptions
WHERE account_id = $1
  AND event_type = $2
  AND status = 'active'
  AND deleted_at IS NULL
`

// GetActiveIntegrationSubscriptionsForEvent retrieves all active subscriptions for an event
func (s *Store) GetActiveIntegrationSubscriptionsForEvent(ctx context.Context, accountID uuid.UUID, eventType string, campaignID *uuid.UUID) ([]integrations.Subscription, error) {
	var subs []IntegrationSubscription
	var err error

	if campaignID != nil {
		err = s.db.SelectContext(ctx, &subs, sqlGetActiveIntegrationSubscriptionsForEvent, accountID, eventType, *campaignID)
	} else {
		err = s.db.SelectContext(ctx, &subs, sqlGetActiveIntegrationSubscriptionsForEventNoCampaign, accountID, eventType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get active subscriptions: %w", err)
	}

	result := make([]integrations.Subscription, len(subs))
	for i, sub := range subs {
		result[i] = toIntegrationSubscription(sub)
	}
	return result, nil
}

const sqlDeleteIntegrationSubscription = `
UPDATE integration_subscriptions
SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// DeleteIntegrationSubscription soft deletes a subscription
func (s *Store) DeleteIntegrationSubscription(ctx context.Context, subscriptionID uuid.UUID) error {
	result, err := s.db.ExecContext(ctx, sqlDeleteIntegrationSubscription, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

const sqlDeleteIntegrationSubscriptionsByAPIKey = `
UPDATE integration_subscriptions
SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE api_key_id = $1 AND deleted_at IS NULL
`

// DeleteIntegrationSubscriptionsByAPIKey soft deletes all subscriptions for an API key
func (s *Store) DeleteIntegrationSubscriptionsByAPIKey(ctx context.Context, apiKeyID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlDeleteIntegrationSubscriptionsByAPIKey, apiKeyID)
	if err != nil {
		return fmt.Errorf("failed to delete subscriptions by API key: %w", err)
	}
	return nil
}

const sqlUpdateIntegrationSubscriptionStatsSuccess = `
UPDATE integration_subscriptions
SET trigger_count = trigger_count + 1,
    last_triggered_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

const sqlUpdateIntegrationSubscriptionStatsFailed = `
UPDATE integration_subscriptions
SET error_count = error_count + 1,
    last_error = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// UpdateIntegrationSubscriptionStats updates subscription statistics after a delivery attempt
func (s *Store) UpdateIntegrationSubscriptionStats(ctx context.Context, subscriptionID uuid.UUID, success bool, errorMsg *string) error {
	var err error
	if success {
		_, err = s.db.ExecContext(ctx, sqlUpdateIntegrationSubscriptionStatsSuccess, subscriptionID)
	} else {
		_, err = s.db.ExecContext(ctx, sqlUpdateIntegrationSubscriptionStatsFailed, subscriptionID, errorMsg)
	}
	if err != nil {
		return fmt.Errorf("failed to update subscription stats: %w", err)
	}
	return nil
}

// ===== Delivery Methods =====

const sqlCreateDelivery = `
INSERT INTO integration_deliveries (subscription_id, event_type, status)
VALUES ($1, $2, $3)
RETURNING id, subscription_id, event_type, status, response_status, duration_ms, error_message, created_at
`

// CreateDelivery creates a new delivery record
func (s *Store) CreateDelivery(ctx context.Context, params integrations.CreateDeliveryParams) (integrations.Delivery, error) {
	var delivery IntegrationDelivery
	err := s.db.GetContext(ctx, &delivery, sqlCreateDelivery, params.SubscriptionID, params.EventType, params.Status)
	if err != nil {
		return integrations.Delivery{}, fmt.Errorf("failed to create delivery: %w", err)
	}
	return toIntegrationDelivery(delivery), nil
}

const sqlUpdateDeliveryStatus = `
UPDATE integration_deliveries
SET status = $2, response_status = $3, duration_ms = $4, error_message = $5
WHERE id = $1
`

// UpdateDeliveryStatus updates a delivery record with the result
func (s *Store) UpdateDeliveryStatus(ctx context.Context, deliveryID uuid.UUID, status string, responseStatus *int, durationMs *int, errorMsg *string) error {
	_, err := s.db.ExecContext(ctx, sqlUpdateDeliveryStatus, deliveryID, status, responseStatus, durationMs, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to update delivery status: %w", err)
	}
	return nil
}

const sqlGetDeliveriesBySubscription = `
SELECT id, subscription_id, event_type, status, response_status, duration_ms, error_message, created_at
FROM integration_deliveries
WHERE subscription_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
`

// GetDeliveriesBySubscription retrieves deliveries for a subscription
func (s *Store) GetDeliveriesBySubscription(ctx context.Context, subscriptionID uuid.UUID, limit, offset int) ([]integrations.Delivery, error) {
	var deliveries []IntegrationDelivery
	err := s.db.SelectContext(ctx, &deliveries, sqlGetDeliveriesBySubscription, subscriptionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get deliveries: %w", err)
	}
	result := make([]integrations.Delivery, len(deliveries))
	for i, d := range deliveries {
		result[i] = toIntegrationDelivery(d)
	}
	return result, nil
}

const sqlGetDeliveriesByAccount = `
SELECT d.id, d.subscription_id, d.event_type, d.status, d.response_status, d.duration_ms, d.error_message, d.created_at
FROM integration_deliveries d
JOIN integration_subscriptions s ON d.subscription_id = s.id
WHERE s.account_id = $1
ORDER BY d.created_at DESC
LIMIT $2 OFFSET $3
`

// GetDeliveriesByAccount retrieves recent deliveries for an account
func (s *Store) GetDeliveriesByAccount(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]integrations.Delivery, error) {
	var deliveries []IntegrationDelivery
	err := s.db.SelectContext(ctx, &deliveries, sqlGetDeliveriesByAccount, accountID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get deliveries: %w", err)
	}
	result := make([]integrations.Delivery, len(deliveries))
	for i, d := range deliveries {
		result[i] = toIntegrationDelivery(d)
	}
	return result, nil
}

// ===== Helper Functions =====

func toIntegrationSubscription(s IntegrationSubscription) integrations.Subscription {
	return integrations.Subscription{
		ID:              s.ID,
		AccountID:       s.AccountID,
		APIKeyID:        s.APIKeyID,
		IntegrationType: integrations.IntegrationType(s.IntegrationType),
		TargetURL:       s.TargetURL,
		EventType:       s.EventType,
		CampaignID:      s.CampaignID,
		Config:          map[string]interface{}(s.Config),
		Status:          s.Status,
		TriggerCount:    s.TriggerCount,
		ErrorCount:      s.ErrorCount,
		LastTriggeredAt: s.LastTriggeredAt,
		LastError:       s.LastError,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
		DeletedAt:       s.DeletedAt,
	}
}

func toIntegrationDelivery(d IntegrationDelivery) integrations.Delivery {
	return integrations.Delivery{
		ID:             d.ID,
		SubscriptionID: d.SubscriptionID,
		EventType:      d.EventType,
		Status:         d.Status,
		ResponseStatus: d.ResponseStatus,
		DurationMs:     d.DurationMs,
		ErrorMessage:   d.ErrorMessage,
		CreatedAt:      d.CreatedAt,
	}
}
