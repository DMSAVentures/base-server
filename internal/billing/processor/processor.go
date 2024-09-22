package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"

	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/customer"
)

type BillingProcessor struct {
	stripKey      string
	WebhookSecret string
	logger        *observability.Logger
	store         store.Store
}

type PaymentIntentItem struct {
	Id     string `json:"id" binding:"required"`
	Amount int64  `json:"amount" binding:"required"`
}

func New(stripKey string, webhookSecret string, store store.Store, logger *observability.Logger) BillingProcessor {
	stripe.Key = stripKey
	return BillingProcessor{
		stripKey:      stripKey,
		WebhookSecret: webhookSecret,
		store:         store,
		logger:        logger,
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
		return "", err
	}

	return customer.ID, nil

}
