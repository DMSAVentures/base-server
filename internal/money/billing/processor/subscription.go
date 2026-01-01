package processor

import (
	"base-server/internal/money/subscriptions"
	"base-server/internal/observability"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/checkout/session"
	"github.com/stripe/stripe-go/v79/subscription"
)

var (
	// ErrNoActiveSubscription is returned when no active subscription is found for a user
	ErrNoActiveSubscription              = errors.New("no active subscription found")
	ErrFailedToCreateSubscriptionIntent  = errors.New("failed to create subscription intent")
	ErrFailedToCancelSubscription        = errors.New("failed to cancel subscription")
	ErrFailedToUpdateSubscription        = errors.New("failed to update subscription")
	ErrFailedToGetSubscription           = errors.New("failed to get subscription")
	ErrFailedToCreateFreeSubscription    = errors.New("failed to create free subscription")
)

func (p *BillingProcessor) CreateSubscriptionIntent(ctx context.Context, userID uuid.UUID, priceID string) (string, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "user_id", Value: userID},
		observability.Field{Key: "price_id", Value: priceID})

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
	ctx = observability.WithFields(ctx, observability.Field{Key: "subscription_id", Value: subscriptionInitialized.ID})

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

func (p *BillingProcessor) CreateFreeSubscription(ctx context.Context, stripeCustomerID string) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "stripe_customer_id", Value: stripeCustomerID})

	p.logger.Info(ctx, "Creating free subscription for customer")

	freePriceStripeID, err := p.store.GetFreePriceStripeID(ctx)
	if err != nil {
		p.logger.Error(ctx, "failed to get free price stripe id", err)
		return ErrFailedToCreateFreeSubscription
	}

	params := &stripe.SubscriptionParams{
		Customer: stripe.String(stripeCustomerID),
		Items: []*stripe.SubscriptionItemsParams{
			{
				Price: stripe.String(freePriceStripeID),
			},
		},
	}

	_, err = subscription.New(params)
	if err != nil {
		p.logger.Error(ctx, "failed to create free subscription", err)
		return ErrFailedToCreateFreeSubscription
	}

	return nil
}

func (p *BillingProcessor) CancelSubscription(ctx context.Context, userID uuid.UUID) error {
	ctx = observability.WithFields(ctx, observability.Field{Key: "user_id", Value: userID})

	activeSub, err := p.GetActiveSubscription(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get active stripe subscription", err)
		return ErrNoActiveSubscription
	}

	_, err = subscription.Cancel(activeSub.StripeID, nil)
	if err != nil {
		p.logger.Error(ctx, "failed to cancel subscription", err)
		return ErrFailedToCancelSubscription
	}

	return nil
}

func (p *BillingProcessor) CancelSubscriptionBySubscriptionExternalID(ctx context.Context, stripSubID string) error {
	ctx = observability.WithFields(ctx, observability.Field{Key: "stripe_subscription_id", Value: stripSubID})

	_, err := subscription.Cancel(stripSubID, nil)
	if err != nil {
		p.logger.Error(ctx, "failed to cancel subscription", err)
		return ErrFailedToCancelSubscription
	}

	return nil
}

func (p *BillingProcessor) UpdateSubscription(ctx context.Context, userID uuid.UUID, priceID string) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "user_id", Value: userID},
		observability.Field{Key: "price_id", Value: priceID})

	activeSub, err := p.GetActiveSubscription(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get active stripe subscription", err)
		return ErrNoActiveSubscription
	}

	params := &stripe.SubscriptionParams{
		Items: []*stripe.SubscriptionItemsParams{
			{
				ID:    stripe.String(activeSub.StripeID),
				Price: stripe.String(priceID),
			},
		},
		ProrationBehavior: stripe.String("create_prorations"),
	}

	_, err = subscription.Update(activeSub.StripeID, params)
	if err != nil {
		p.logger.Error(ctx, "failed to update subscription", err)
		return ErrFailedToUpdateSubscription
	}

	return nil
}

func (p *BillingProcessor) GetActiveSubscription(ctx context.Context, userID uuid.UUID) (subscriptions.Subscription,
	error) {
	ctx = observability.WithFields(ctx, observability.Field{Key: "user_id", Value: userID})

	sub, err := p.subscriptionService.GetSubscriptionByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, subscriptions.ErrNoSubscription) {
			p.logger.Info(ctx, "no active subscription found for user")
			return subscriptions.Subscription{}, ErrNoActiveSubscription
		}

		p.logger.Error(ctx, "failed to get subscription by user id", err)
		return subscriptions.Subscription{}, ErrFailedToGetSubscription
	}

	return sub, nil
}

func (p *BillingProcessor) CreateCheckoutSession(ctx context.Context, userID uuid.UUID, priceID string) (*stripe.CheckoutSession, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "user_id", Value: userID},
		observability.Field{Key: "price_id", Value: priceID})

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
		ReturnURL: stripe.String(fmt.Sprintf("%s/billing/payment_attempt?session_id={CHECKOUT_SESSION_ID}",
			p.webhostURL)),
		AutomaticTax: &stripe.CheckoutSessionAutomaticTaxParams{Enabled: stripe.Bool(true)},
	}

	session, err := session.New(params)
	if err != nil {
		p.logger.Error(ctx, "failed to create checkout session", err)
		return nil, errors.New("failed to create checkout session")
	}

	return session, nil

}

type CheckoutSessionInfo struct {
	SessionID     string `json:"session_id"`
	Status        string `json:"status"`
	PaymentStatus string `json:"payment_status"`
}

func (p *BillingProcessor) GetCheckoutSession(ctx context.Context, sessionID string) (CheckoutSessionInfo, error) {
	ctx = observability.WithFields(ctx, observability.Field{Key: "session_id", Value: sessionID})

	session, err := session.Get(sessionID, nil)
	if err != nil {
		p.logger.Error(ctx, "failed to get checkout session", err)
		return CheckoutSessionInfo{}, errors.New("failed to get checkout session")
	}

	return CheckoutSessionInfo{
		SessionID:     session.ID,
		Status:        string(session.Status),
		PaymentStatus: string(session.PaymentStatus),
	}, nil
}
