package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"

	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/customer"
)

var ErrFailedToCreateCustomer = errors.New("failed to create customer")

type BillingProcessor struct {
	stripKey            string
	WebhookSecret       string
	webhostURL          string
	logger              *observability.Logger
	store               Store
	productService      ProductService
	subscriptionService SubscriptionService
	emailService        EmailService
}

type PaymentIntentItem struct {
	Id     string `json:"id" binding:"required"`
	Amount int64  `json:"amount" binding:"required"`
}

func New(stripKey string, webhookSecret string, webhostURL string, store Store,
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
	ctx = observability.WithFields(ctx, observability.Field{"email", email})

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
