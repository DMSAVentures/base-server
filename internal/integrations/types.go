package integrations

//go:generate mockgen -source=types.go -destination=mocks_test.go -package=integrations

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// IntegrationType represents the type of integration
type IntegrationType string

const (
	IntegrationZapier  IntegrationType = "zapier"
	IntegrationSlack   IntegrationType = "slack"
	IntegrationDiscord IntegrationType = "discord"
	IntegrationCustom  IntegrationType = "custom"
)

// Event represents an event to be delivered to integrations
type Event struct {
	ID         string
	Type       string
	AccountID  uuid.UUID
	CampaignID *uuid.UUID
	Data       map[string]interface{}
	Timestamp  time.Time
}

// Subscription represents a generic event subscription
type Subscription struct {
	ID              uuid.UUID
	AccountID       uuid.UUID
	APIKeyID        *uuid.UUID // Optional reference to API key that created this subscription
	IntegrationType IntegrationType
	TargetURL       string
	EventType       string
	CampaignID      *uuid.UUID
	Config          map[string]interface{}
	Status          string
	TriggerCount    int
	ErrorCount      int
	LastTriggeredAt *time.Time
	LastError       *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

// Delivery represents a delivery attempt to an integration
type Delivery struct {
	ID             uuid.UUID
	SubscriptionID uuid.UUID
	EventType      string
	Status         string
	ResponseStatus *int
	DurationMs     *int
	ErrorMessage   *string
	CreatedAt      time.Time
}

// DeliveryStatus constants
const (
	DeliveryStatusPending = "pending"
	DeliveryStatusSuccess = "success"
	DeliveryStatusFailed  = "failed"
)

// SubscriptionStatus constants
const (
	SubscriptionStatusActive  = "active"
	SubscriptionStatusPaused  = "paused"
	SubscriptionStatusDeleted = "deleted"
)

// Deliverer interface - each integration implements this
type Deliverer interface {
	// Type returns the integration type this deliverer handles
	Type() IntegrationType

	// Deliver sends an event to the subscription target
	Deliver(ctx context.Context, subscription Subscription, event Event) error

	// FormatPayload formats the event into the integration-specific format
	FormatPayload(event Event, config map[string]interface{}) ([]byte, error)
}

// IntegrationStore defines the interface for integration-related database operations
type IntegrationStore interface {
	// Integration Subscriptions
	CreateIntegrationSubscription(ctx context.Context, params CreateSubscriptionParams) (Subscription, error)
	GetIntegrationSubscriptionByID(ctx context.Context, subscriptionID uuid.UUID) (Subscription, error)
	GetIntegrationSubscriptionsByAccount(ctx context.Context, accountID uuid.UUID, integrationType *IntegrationType) ([]Subscription, error)
	GetActiveIntegrationSubscriptionsForEvent(ctx context.Context, accountID uuid.UUID, eventType string, campaignID *uuid.UUID) ([]Subscription, error)
	DeleteIntegrationSubscription(ctx context.Context, subscriptionID uuid.UUID) error
	DeleteIntegrationSubscriptionsByAPIKey(ctx context.Context, apiKeyID uuid.UUID) error
	UpdateIntegrationSubscriptionStats(ctx context.Context, subscriptionID uuid.UUID, success bool, errorMsg *string) error

	// Deliveries
	CreateDelivery(ctx context.Context, params CreateDeliveryParams) (Delivery, error)
	UpdateDeliveryStatus(ctx context.Context, deliveryID uuid.UUID, status string, responseStatus *int, durationMs *int, errorMsg *string) error
	GetDeliveriesBySubscription(ctx context.Context, subscriptionID uuid.UUID, limit, offset int) ([]Delivery, error)
	GetDeliveriesByAccount(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]Delivery, error)
}

// CreateSubscriptionParams contains parameters for creating a subscription
type CreateSubscriptionParams struct {
	AccountID       uuid.UUID
	APIKeyID        *uuid.UUID // Optional - tracks which API key created the subscription
	IntegrationType IntegrationType
	TargetURL       string
	EventType       string
	CampaignID      *uuid.UUID
	Config          map[string]interface{}
}

// CreateDeliveryParams contains parameters for creating a delivery record
type CreateDeliveryParams struct {
	SubscriptionID uuid.UUID
	EventType      string
	Status         string
}
