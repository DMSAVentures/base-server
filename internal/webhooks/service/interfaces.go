package service

import (
	"context"

	"github.com/google/uuid"
)

// WebhookServiceInterface defines the interface for webhook delivery and event dispatching
type WebhookServiceInterface interface {
	// DispatchEvent dispatches a webhook event to all subscribed webhooks for an account
	DispatchEvent(ctx context.Context, accountID uuid.UUID, campaignID *uuid.UUID, eventType string, data map[string]interface{}) error

	// RetryFailedDeliveries processes failed webhook deliveries that are ready for retry
	RetryFailedDeliveries(ctx context.Context, limit int) error

	// TestWebhook sends a test event to a specific webhook
	TestWebhook(ctx context.Context, webhookID uuid.UUID) error
}
