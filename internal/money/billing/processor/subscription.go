package processor

import (
	"base-server/internal/observability"
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/checkout/session"
	"github.com/stripe/stripe-go/v79/subscription"
)

var (
	// ErrNoActiveSubscription is returned when no active subscription is found for a user
	ErrNoActiveSubscription             = errors.New("no active subscription found")
	ErrFailedToCreateSubscriptionIntent = errors.New("failed to create subscription intent")
	ErrFailedToCancelSubscription       = errors.New("failed to cancel subscription")
	ErrFailedToUpdateSubscription       = errors.New("failed to update subscription")
)

func (p *BillingProcessor) CreateSubscriptionIntent(ctx context.Context, userID uuid.UUID, priceID string) (string, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{"user_id", userID},
		observability.Field{"price_id", priceID})

	p.logger.Info(ctx, "Creating subscription intent for user")

	stripeCustomerID, err := p.store.GetStripeCustomerIDByUserExternalID(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get stripe customer id from db", err)
		return "", ErrFailedToCreateSubscriptionIntent
	}

	price, err := p.store.GetPriceByID(ctx, priceID)
	if err != nil {
		p.logger.Error(ctx, "failed to get price by stripe id", err)
		return "", ErrFailedToCreateSubscriptionIntent
	}

	// Create the subscription
	params := &stripe.SubscriptionParams{
		Customer: stripe.String(stripeCustomerID), // The customer ID
		Items: []*stripe.SubscriptionItemsParams{
			{
				Price: stripe.String(price.StripeID), // The recurring price ID
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
		p.logger.Error(ctx, "failed to initialize incomplete subscription", err)
		return "", ErrFailedToCreateSubscriptionIntent
	}
	ctx = observability.WithFields(ctx, observability.Field{"subscription_id", subscriptionInitialized.ID})

	// Check if a PaymentIntent exists on the latest invoice
	var clientSecret string
	if subscriptionInitialized.LatestInvoice != nil && subscriptionInitialized.LatestInvoice.PaymentIntent != nil {
		clientSecret = subscriptionInitialized.LatestInvoice.PaymentIntent.ClientSecret
	}

	if clientSecret == "" {
		p.logger.Error(ctx, "failed to get client secret for payment intent", nil)
		return "", ErrFailedToCreateSubscriptionIntent
	}

	return clientSecret, nil
}

func (p *BillingProcessor) CancelSubscription(ctx context.Context, userID uuid.UUID) error {
	ctx = observability.WithFields(ctx, observability.Field{"user_id", userID})

	activeSub, err := p.GetActiveSubscription(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get active stripe subscription", err)
		return ErrNoActiveSubscription
	}

	_, err = subscription.Cancel(activeSub.ID, nil)
	if err != nil {
		p.logger.Error(ctx, "failed to cancel subscription", err)
		return ErrFailedToCancelSubscription
	}

	return nil
}

func (p *BillingProcessor) UpdateSubscription(ctx context.Context, userID uuid.UUID, priceID string) error {
	ctx = observability.WithFields(ctx,
		observability.Field{"user_id", userID},
		observability.Field{"price_id", priceID})

	activeSub, err := p.GetActiveSubscription(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get active stripe subscription", err)
		return ErrNoActiveSubscription
	}

	params := &stripe.SubscriptionParams{
		Items: []*stripe.SubscriptionItemsParams{
			{
				ID:    stripe.String(activeSub.Items.Data[0].ID),
				Price: stripe.String(priceID),
			},
		},
		ProrationBehavior: stripe.String("create_prorations"),
	}

	_, err = subscription.Update(activeSub.ID, params)
	if err != nil {
		p.logger.Error(ctx, "failed to update subscription", err)
		return ErrFailedToUpdateSubscription
	}

	return nil
}

func (p *BillingProcessor) GetActiveSubscription(ctx context.Context, userID uuid.UUID) (*stripe.Subscription, error) {
	stripeCustomerID, err := p.store.GetStripeCustomerIDByUserExternalID(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get stripe customer id from db", err)
		return nil, errors.New("failed to get stripe customer id")
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
		return nil, errors.New("failed to list subscriptions: %w")
	}

	return nil, errors.New("no active subscription found")
}

func (p *BillingProcessor) CreateCheckoutSession(ctx context.Context, userID uuid.UUID, priceID string) (*stripe.CheckoutSession, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{"user_id", userID},
		observability.Field{"price_id", priceID})

	stripeCustomerID, err := p.store.GetStripeCustomerIDByUserExternalID(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get stripe customer id from db", err)
		return nil, errors.New("failed to get stripe customer id")
	}

	price, err := p.store.GetPriceByID(ctx, priceID)
	if err != nil {
		p.logger.Error(ctx, "failed to get price by stripe id", err)
		return nil, errors.New("failed to get price by stripe id")
	}

	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(stripeCustomerID),
		PaymentMethodTypes: []*string{
			stripe.String("card"),
		},
		UIMode: stripe.String("embedded"),
		Mode:   stripe.String("subscription"),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(price.StripeID),
				Quantity: stripe.Int64(1),
			},
		},
		CustomerUpdate: &stripe.CheckoutSessionCustomerUpdateParams{
			Address: stripe.String("auto"),
		},
		ReturnURL:    stripe.String("http://localhost:3000/payment?session_id={CHECKOUT_SESSION_ID}"),
		AutomaticTax: &stripe.CheckoutSessionAutomaticTaxParams{Enabled: stripe.Bool(true)},
	}

	session, err := session.New(params)
	if err != nil {
		p.logger.Error(ctx, "failed to create checkout session", err)
		return nil, errors.New("failed to create checkout session")
	}

	return session, nil

}

func (p *BillingProcessor) GetCheckoutSession(ctx context.Context, sessionID string) (*stripe.CheckoutSession, error) {
	ctx = observability.WithFields(ctx, observability.Field{"session_id", sessionID})

	session, err := session.Get(sessionID, nil)
	if err != nil {
		p.logger.Error(ctx, "failed to get checkout session", err)
		return nil, errors.New("failed to get checkout session")
	}

	return session, nil
}
