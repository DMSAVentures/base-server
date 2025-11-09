package subscriptions

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79"
)

// SubscriptionServiceInterface defines the contract for subscription-related operations
type SubscriptionServiceInterface interface {
	CreateSubscription(ctx context.Context, subscriptionCreated stripe.Subscription) error
	UpdateSubscription(ctx context.Context, subscriptionUpdated stripe.Subscription) error
	CancelSubscription(ctx context.Context, subscriptionID string, cancelAt time.Time) error
	GetSubscriptionByID(ctx context.Context, subscriptionID string) (Subscription, error)
	GetSubscriptionByUserID(ctx context.Context, userID uuid.UUID) (Subscription, error)
}
