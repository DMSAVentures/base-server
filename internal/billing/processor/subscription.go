package processor

import (
	"base-server/internal/observability"
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/subscription"
)

func (p *BillingProcessor) CreateSubscriptionIntent(ctx context.Context, userID uuid.UUID, priceID string) (string, error) {
	ctx = observability.WithFields(ctx, observability.Field{"user_id", userID})
	p.logger.Info(ctx, "Creating subscription for user")

	stripeCustomerID, err := p.store.GetStripeCustomerIDByUserExternalID(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get stripe customer id from db", err)
		return "", err
	}

	// Create the subscription
	params := &stripe.SubscriptionParams{
		Customer: stripe.String(stripeCustomerID), // The customer ID
		Items: []*stripe.SubscriptionItemsParams{
			{
				Price: stripe.String(priceID), // The recurring price ID
			},
		},
		PaymentBehavior: stripe.String("default_incomplete"), // Handle payment confirmation manually
		Expand: []*string{
			stripe.String("customer"),                      // Expand the customer object to include email and other details
			stripe.String("latest_invoice.payment_intent"), // Expand the latest invoice object to include the payment intent
		},
	}

	subscriptionInitialized, err := subscription.New(params)
	if err != nil {
		p.logger.Error(ctx, "failed to create subscription", err)
		return "", err
	}

	// Check if a PaymentIntent exists on the latest invoice
	var clientSecret string
	if subscriptionInitialized.LatestInvoice != nil && subscriptionInitialized.LatestInvoice.PaymentIntent != nil {
		clientSecret = subscriptionInitialized.LatestInvoice.PaymentIntent.ClientSecret
	}

	if clientSecret == "" {
		p.logger.Error(ctx, "failed to get client secret for payment intent", nil)
		return "", errors.New("failed to create payment intent")
	}

	return clientSecret, nil
}

func (p *BillingProcessor) CancelSubscription(ctx context.Context, userID uuid.UUID) error {
	activeSub, err := p.GetActiveSubscription(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get active stripe subscription", err)
		return err
	}

	_, err = subscription.Cancel(activeSub.ID, nil)
	if err != nil {
		p.logger.Error(ctx, "failed to cancel subscription", err)
		return err
	}

	return nil
}

func (p *BillingProcessor) GetActiveSubscription(ctx context.Context, userID uuid.UUID) (*stripe.Subscription, error) {
	stripeCustomerID, err := p.store.GetStripeCustomerIDByUserExternalID(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get stripe customer id from db", err)
		return nil, err
	}

	params := &stripe.SubscriptionListParams{
		Customer: stripe.String(stripeCustomerID),
		Status:   stripe.String("active"),
	}
	i := subscription.List(params)

	for i.Next() {
		s := i.Subscription()
		return s, nil
	}

	if err := i.Err(); err != nil {
		p.logger.Error(ctx, "failed to list subscriptions", err)
		return nil, err
	}

	return nil, errors.New("no active subscription found")
}
