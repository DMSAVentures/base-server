package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateWebhookParams represents parameters for creating a webhook
type CreateWebhookParams struct {
	AccountID    uuid.UUID
	CampaignID   *uuid.UUID
	URL          string
	Secret       string
	Events       []string
	RetryEnabled bool
	MaxRetries   int
}

const sqlCreateWebhook = `
INSERT INTO webhooks (account_id, campaign_id, url, secret, events, retry_enabled, max_retries)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, account_id, campaign_id, url, secret, events, status, retry_enabled, max_retries, total_sent, total_failed, last_success_at, last_failure_at, created_at, updated_at, deleted_at
`

// CreateWebhook creates a new webhook
func (s *Store) CreateWebhook(ctx context.Context, params CreateWebhookParams) (Webhook, error) {
	var webhook Webhook
	err := s.db.GetContext(ctx, &webhook, sqlCreateWebhook,
		params.AccountID,
		params.CampaignID,
		params.URL,
		params.Secret,
		StringArray(params.Events),
		params.RetryEnabled,
		params.MaxRetries)
	if err != nil {
		s.logger.Error(ctx, "failed to create webhook", err)
		return Webhook{}, fmt.Errorf("failed to create webhook: %w", err)
	}
	return webhook, nil
}

const sqlGetWebhookByID = `
SELECT id, account_id, campaign_id, url, secret, events, status, retry_enabled, max_retries, total_sent, total_failed, last_success_at, last_failure_at, created_at, updated_at, deleted_at
FROM webhooks
WHERE id = $1 AND deleted_at IS NULL
`

// GetWebhookByID retrieves a webhook by ID
func (s *Store) GetWebhookByID(ctx context.Context, webhookID uuid.UUID) (Webhook, error) {
	var webhook Webhook
	err := s.db.GetContext(ctx, &webhook, sqlGetWebhookByID, webhookID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Webhook{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get webhook", err)
		return Webhook{}, fmt.Errorf("failed to get webhook: %w", err)
	}
	return webhook, nil
}

const sqlGetWebhooksByAccount = `
SELECT id, account_id, campaign_id, url, secret, events, status, retry_enabled, max_retries, total_sent, total_failed, last_success_at, last_failure_at, created_at, updated_at, deleted_at
FROM webhooks
WHERE account_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
`

// GetWebhooksByAccount retrieves all webhooks for an account
func (s *Store) GetWebhooksByAccount(ctx context.Context, accountID uuid.UUID) ([]Webhook, error) {
	var webhooks []Webhook
	err := s.db.SelectContext(ctx, &webhooks, sqlGetWebhooksByAccount, accountID)
	if err != nil {
		s.logger.Error(ctx, "failed to get webhooks by account", err)
		return nil, fmt.Errorf("failed to get webhooks by account: %w", err)
	}
	return webhooks, nil
}

const sqlGetWebhooksByCampaign = `
SELECT id, account_id, campaign_id, url, secret, events, status, retry_enabled, max_retries, total_sent, total_failed, last_success_at, last_failure_at, created_at, updated_at, deleted_at
FROM webhooks
WHERE campaign_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
`

// GetWebhooksByCampaign retrieves all webhooks for a campaign
func (s *Store) GetWebhooksByCampaign(ctx context.Context, campaignID uuid.UUID) ([]Webhook, error) {
	var webhooks []Webhook
	err := s.db.SelectContext(ctx, &webhooks, sqlGetWebhooksByCampaign, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to get webhooks by campaign", err)
		return nil, fmt.Errorf("failed to get webhooks by campaign: %w", err)
	}
	return webhooks, nil
}

const sqlUpdateWebhook = `
UPDATE webhooks
SET url = COALESCE($2, url),
    events = COALESCE($3, events),
    status = COALESCE($4, status),
    retry_enabled = COALESCE($5, retry_enabled),
    max_retries = COALESCE($6, max_retries),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, account_id, campaign_id, url, secret, events, status, retry_enabled, max_retries, total_sent, total_failed, last_success_at, last_failure_at, created_at, updated_at, deleted_at
`

// UpdateWebhookParams represents parameters for updating a webhook
type UpdateWebhookParams struct {
	URL          *string
	Events       []string
	Status       *string
	RetryEnabled *bool
	MaxRetries   *int
}

// UpdateWebhook updates a webhook
func (s *Store) UpdateWebhook(ctx context.Context, webhookID uuid.UUID, params UpdateWebhookParams) (Webhook, error) {
	var webhook Webhook
	var eventsArray interface{}
	if params.Events != nil {
		eventsArray = StringArray(params.Events)
	}

	err := s.db.GetContext(ctx, &webhook, sqlUpdateWebhook,
		webhookID,
		params.URL,
		eventsArray,
		params.Status,
		params.RetryEnabled,
		params.MaxRetries)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Webhook{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to update webhook", err)
		return Webhook{}, fmt.Errorf("failed to update webhook: %w", err)
	}
	return webhook, nil
}

const sqlDeleteWebhook = `
UPDATE webhooks
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// DeleteWebhook soft deletes a webhook
func (s *Store) DeleteWebhook(ctx context.Context, webhookID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, sqlDeleteWebhook, webhookID)
	if err != nil {
		s.logger.Error(ctx, "failed to delete webhook", err)
		return fmt.Errorf("failed to delete webhook: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		s.logger.Error(ctx, "failed to get rows affected", err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

const sqlIncrementWebhookSent = `
UPDATE webhooks
SET total_sent = total_sent + 1,
    last_success_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// IncrementWebhookSent increments the total sent counter
func (s *Store) IncrementWebhookSent(ctx context.Context, webhookID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlIncrementWebhookSent, webhookID)
	if err != nil {
		s.logger.Error(ctx, "failed to increment webhook sent", err)
		return fmt.Errorf("failed to increment webhook sent: %w", err)
	}
	return nil
}

const sqlIncrementWebhookFailed = `
UPDATE webhooks
SET total_failed = total_failed + 1,
    last_failure_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// IncrementWebhookFailed increments the total failed counter
func (s *Store) IncrementWebhookFailed(ctx context.Context, webhookID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlIncrementWebhookFailed, webhookID)
	if err != nil {
		s.logger.Error(ctx, "failed to increment webhook failed", err)
		return fmt.Errorf("failed to increment webhook failed: %w", err)
	}
	return nil
}

// Webhook Delivery operations

// CreateWebhookDeliveryParams represents parameters for creating a webhook delivery
type CreateWebhookDeliveryParams struct {
	WebhookID   uuid.UUID
	EventType   string
	Payload     JSONB
	NextRetryAt *time.Time
}

const sqlCreateWebhookDelivery = `
INSERT INTO webhook_deliveries (webhook_id, event_type, payload, next_retry_at)
VALUES ($1, $2, $3, $4)
RETURNING id, webhook_id, event_type, payload, status, request_headers, response_status, response_body, response_headers, duration_ms, attempt_number, next_retry_at, error_message, created_at, delivered_at
`

// CreateWebhookDelivery creates a new webhook delivery record
func (s *Store) CreateWebhookDelivery(ctx context.Context, params CreateWebhookDeliveryParams) (WebhookDelivery, error) {
	var delivery WebhookDelivery
	err := s.db.GetContext(ctx, &delivery, sqlCreateWebhookDelivery,
		params.WebhookID,
		params.EventType,
		params.Payload,
		params.NextRetryAt)
	if err != nil {
		s.logger.Error(ctx, "failed to create webhook delivery", err)
		return WebhookDelivery{}, fmt.Errorf("failed to create webhook delivery: %w", err)
	}
	return delivery, nil
}

const sqlGetWebhookDeliveryByID = `
SELECT id, webhook_id, event_type, payload, status, request_headers, response_status, response_body, response_headers, duration_ms, attempt_number, next_retry_at, error_message, created_at, delivered_at
FROM webhook_deliveries
WHERE id = $1
`

// GetWebhookDeliveryByID retrieves a webhook delivery by ID
func (s *Store) GetWebhookDeliveryByID(ctx context.Context, deliveryID uuid.UUID) (WebhookDelivery, error) {
	var delivery WebhookDelivery
	err := s.db.GetContext(ctx, &delivery, sqlGetWebhookDeliveryByID, deliveryID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WebhookDelivery{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get webhook delivery", err)
		return WebhookDelivery{}, fmt.Errorf("failed to get webhook delivery: %w", err)
	}
	return delivery, nil
}

const sqlGetWebhookDeliveriesByWebhook = `
SELECT id, webhook_id, event_type, payload, status, request_headers, response_status, response_body, response_headers, duration_ms, attempt_number, next_retry_at, error_message, created_at, delivered_at
FROM webhook_deliveries
WHERE webhook_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
`

// GetWebhookDeliveriesByWebhook retrieves webhook deliveries for a webhook with pagination
func (s *Store) GetWebhookDeliveriesByWebhook(ctx context.Context, webhookID uuid.UUID, limit, offset int) ([]WebhookDelivery, error) {
	var deliveries []WebhookDelivery
	err := s.db.SelectContext(ctx, &deliveries, sqlGetWebhookDeliveriesByWebhook, webhookID, limit, offset)
	if err != nil {
		s.logger.Error(ctx, "failed to get webhook deliveries", err)
		return nil, fmt.Errorf("failed to get webhook deliveries: %w", err)
	}
	return deliveries, nil
}

const sqlUpdateWebhookDeliveryStatus = `
UPDATE webhook_deliveries
SET status = $2,
    response_status = $3,
    response_body = $4,
    duration_ms = $5,
    error_message = $6,
    delivered_at = CASE WHEN $2 = 'success' THEN CURRENT_TIMESTAMP ELSE delivered_at END
WHERE id = $1
`

// UpdateWebhookDeliveryStatusParams represents parameters for updating webhook delivery status
type UpdateWebhookDeliveryStatusParams struct {
	Status         string
	ResponseStatus *int
	ResponseBody   *string
	DurationMs     *int
	ErrorMessage   *string
}

// UpdateWebhookDeliveryStatus updates the status and response details of a webhook delivery
func (s *Store) UpdateWebhookDeliveryStatus(ctx context.Context, deliveryID uuid.UUID, params UpdateWebhookDeliveryStatusParams) error {
	res, err := s.db.ExecContext(ctx, sqlUpdateWebhookDeliveryStatus,
		deliveryID,
		params.Status,
		params.ResponseStatus,
		params.ResponseBody,
		params.DurationMs,
		params.ErrorMessage)
	if err != nil {
		s.logger.Error(ctx, "failed to update webhook delivery status", err)
		return fmt.Errorf("failed to update webhook delivery status: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		s.logger.Error(ctx, "failed to get rows affected", err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

const sqlGetPendingWebhookDeliveries = `
SELECT id, webhook_id, event_type, payload, status, request_headers, response_status, response_body, response_headers, duration_ms, attempt_number, next_retry_at, error_message, created_at, delivered_at
FROM webhook_deliveries
WHERE status = 'failed' AND next_retry_at IS NOT NULL AND next_retry_at <= CURRENT_TIMESTAMP
ORDER BY next_retry_at ASC
LIMIT $1
`

// GetPendingWebhookDeliveries retrieves webhook deliveries ready for retry
func (s *Store) GetPendingWebhookDeliveries(ctx context.Context, limit int) ([]WebhookDelivery, error) {
	var deliveries []WebhookDelivery
	err := s.db.SelectContext(ctx, &deliveries, sqlGetPendingWebhookDeliveries, limit)
	if err != nil {
		s.logger.Error(ctx, "failed to get pending webhook deliveries", err)
		return nil, fmt.Errorf("failed to get pending webhook deliveries: %w", err)
	}
	return deliveries, nil
}

const sqlIncrementDeliveryAttempt = `
UPDATE webhook_deliveries
SET attempt_number = attempt_number + 1,
    next_retry_at = $2
WHERE id = $1
`

// IncrementDeliveryAttempt increments the attempt number and sets next retry time
func (s *Store) IncrementDeliveryAttempt(ctx context.Context, deliveryID uuid.UUID, nextRetryAt *time.Time) error {
	_, err := s.db.ExecContext(ctx, sqlIncrementDeliveryAttempt, deliveryID, nextRetryAt)
	if err != nil {
		s.logger.Error(ctx, "failed to increment delivery attempt", err)
		return fmt.Errorf("failed to increment delivery attempt: %w", err)
	}
	return nil
}
