package processor

import (
	"base-server/internal/store"
	"context"

	"github.com/google/uuid"
)

// WebhookStore defines the database operations required by WebhookProcessor
type WebhookStore interface {
	CreateWebhook(ctx context.Context, params store.CreateWebhookParams) (store.Webhook, error)
	UpdateWebhook(ctx context.Context, webhookID uuid.UUID, params store.UpdateWebhookParams) (store.Webhook, error)
	GetWebhookByID(ctx context.Context, webhookID uuid.UUID) (store.Webhook, error)
	GetWebhooksByAccount(ctx context.Context, accountID uuid.UUID) ([]store.Webhook, error)
	GetWebhooksByCampaign(ctx context.Context, campaignID uuid.UUID) ([]store.Webhook, error)
	DeleteWebhook(ctx context.Context, webhookID uuid.UUID) error
	GetWebhookDeliveriesByWebhook(ctx context.Context, webhookID uuid.UUID, limit, offset int) ([]store.WebhookDelivery, error)
}

// WebhookService defines the webhook operations required by WebhookProcessor
type WebhookService interface {
	TestWebhook(ctx context.Context, webhookID uuid.UUID) error
}
