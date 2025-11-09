package processor

import (
	"base-server/internal/money/products"
	"base-server/internal/money/subscriptions"
	"context"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79"
)

// BillingProcessorInterface defines the contract for billing operations
type BillingProcessorInterface interface {
	// Customer Management
	CreateStripeCustomer(ctx context.Context, email string) (string, error)

	// Subscription Management
	CreateSubscriptionIntent(ctx context.Context, userID uuid.UUID, priceID string) (string, error)
	CancelSubscription(ctx context.Context, userID uuid.UUID) error
	CancelSubscriptionBySubscriptionExternalID(ctx context.Context, stripSubID string) error
	UpdateSubscription(ctx context.Context, userID uuid.UUID, priceID string) error
	GetActiveSubscription(ctx context.Context, userID uuid.UUID) (subscriptions.Subscription, error)

	// Checkout Management
	CreateCheckoutSession(ctx context.Context, userID uuid.UUID, priceID string) (*stripe.CheckoutSession, error)
	GetCheckoutSession(ctx context.Context, sessionID string) (CheckoutSessionInfo, error)

	// Payment Processing
	SetupPaymentMethodUpdateIntent(ctx context.Context, userID uuid.UUID) (string, error)
	GetPaymentMethodForUser(ctx context.Context, userID uuid.UUID) (PaymentMethod, error)
	CreateStripePaymentIntent(ctx context.Context, items []PaymentIntentItem) (string, error)
	CreateCustomerPortal(ctx context.Context, user uuid.UUID) (string, error)

	// Pricing Management
	ListPrices(ctx context.Context) ([]products.Price, error)

	// Webhook Handlers
	PaymentIntentSucceeded(ctx context.Context, paymentIntent stripe.PaymentIntent)
	SubscriptionUpdated(ctx context.Context, subscription stripe.Subscription) error
	SubscriptionCreated(ctx context.Context, subscription stripe.Subscription) error
	InvoicePaymentFailed(ctx context.Context, invoice stripe.Invoice) error
	InvoicePaymentPaid(ctx context.Context, invoice stripe.Invoice) error
	ProductCreated(ctx context.Context, productCreated stripe.Product) error
	PriceCreated(ctx context.Context, priceCreated stripe.Price) error
	PriceUpdated(ctx context.Context, priceUpdated stripe.Price) error
	SubscriptionDeleted(ctx context.Context, subscriptionDeleted stripe.Subscription) error
	PriceDeleted(ctx context.Context, priceDeleted stripe.Price) error
	HandleWebhook(ctx context.Context, event stripe.Event) error
}
