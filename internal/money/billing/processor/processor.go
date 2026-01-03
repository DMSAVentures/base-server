package processor

//go:generate go run go.uber.org/mock/mockgen@latest -source=processor.go -destination=mocks_test.go -package=processor

import (
	"base-server/internal/money/products"
	"base-server/internal/money/subscriptions"
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/customer"
)

// BillingProcessorInterface defines the contract for billing operations
type BillingProcessorInterface interface {
	// Customer Management
	CreateStripeCustomer(ctx context.Context, email string) (string, error)

	// Subscription Management
	CreateFreeSubscription(ctx context.Context, stripeCustomerID string) error
	CancelSubscription(ctx context.Context, userID uuid.UUID) error
	CancelSubscriptionBySubscriptionExternalID(ctx context.Context, stripSubID string) error
	GetActiveSubscription(ctx context.Context, userID uuid.UUID) (subscriptions.Subscription, error)

	// Checkout & Portal Management
	CreateCheckoutSession(ctx context.Context, userID uuid.UUID, priceID string) (*stripe.CheckoutSession, error)
	GetCheckoutSession(ctx context.Context, sessionID string) (CheckoutSessionInfo, error)
	CreateCustomerPortal(ctx context.Context, userID uuid.UUID) (string, error)

	// Payment Method Management
	SetupPaymentMethodUpdateIntent(ctx context.Context, userID uuid.UUID) (string, error)
	GetPaymentMethodForUser(ctx context.Context, userID uuid.UUID) (PaymentMethod, error)

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

// BillingStore defines the database operations required by BillingProcessor
type BillingStore interface {
	GetStripeCustomerIDByUserExternalID(ctx context.Context, ID uuid.UUID) (string, error)
	GetPaymentMethodByUserID(ctx context.Context, userID uuid.UUID) (*store.PaymentMethod, error)
	GetPriceByID(ctx context.Context, priceID string) (store.Price, error)
	GetFreePriceStripeID(ctx context.Context) (string, error)
}

// ProductService defines the product operations required by BillingProcessor
type ProductService interface {
	ListPrices(ctx context.Context) ([]products.Price, error)
	CreateProduct(ctx context.Context, productCreated stripe.Product) error
	CreatePrice(ctx context.Context, priceCreated stripe.Price) error
	UpdatePrice(ctx context.Context, priceUpdated stripe.Price) error
	DeletePrice(ctx context.Context, priceDeleted stripe.Price) error
}

// SubscriptionService defines the subscription operations required by BillingProcessor
type SubscriptionService interface {
	GetSubscriptionByUserID(ctx context.Context, userID uuid.UUID) (subscriptions.Subscription, error)
	CancelSubscription(ctx context.Context, subscriptionID string, cancelAt time.Time) error
	UpdateSubscription(ctx context.Context, subscriptionUpdated stripe.Subscription) error
	CreateSubscription(ctx context.Context, subscriptionCreated stripe.Subscription) error
}

// EmailService defines the email operations required by BillingProcessor
type EmailService interface {
	SendEmail(ctx context.Context, to, subject, htmlContent string) error
}

var ErrFailedToCreateCustomer = errors.New("failed to create customer")

type BillingProcessor struct {
	stripKey            string
	WebhookSecret       string
	webhostURL          string
	logger              *observability.Logger
	store               BillingStore
	productService      ProductService
	subscriptionService SubscriptionService
	emailService        EmailService
}

func New(stripKey string, webhookSecret string, webhostURL string, store BillingStore,
	productService ProductService, subService SubscriptionService,
	emailService EmailService, logger *observability.Logger) BillingProcessor {
	stripe.Key = stripKey
	return BillingProcessor{
		stripKey:            stripKey,
		WebhookSecret:       webhookSecret,
		webhostURL:          webhostURL,
		store:               store,
		productService:      productService,
		subscriptionService: subService,
		emailService:        emailService,
		logger:              logger,
	}
}

func (p *BillingProcessor) CreateStripeCustomer(ctx context.Context, email string) (string, error) {
	ctx = observability.WithFields(ctx, observability.Field{Key: "email", Value: email})

	params := &stripe.CustomerParams{
		Email: stripe.String(email),
	}

	customer, err := customer.New(params)
	if err != nil {
		p.logger.Error(ctx, "failed to create customer", err)
		return "", ErrFailedToCreateCustomer
	}

	return customer.ID, nil
}
