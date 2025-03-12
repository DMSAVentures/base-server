package subscriptions

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79"
)

type Subscription struct {
	ID              uuid.UUID `json:"id"`
	UserID          uuid.UUID `json:"user_id"`
	PriceID         uuid.UUID `json:"price_id"`
	StripeID        string    `json:"stripe_id"`
	Status          string    `json:"status"`
	StartDate       time.Time `json:"start_date"`
	EndDate         time.Time `json:"end_date"`
	NextBillingDate time.Time `json:"next_billing_date"`
}

var ErrNoSubscription = errors.New("no active subscription found")

func (p *SubscriptionService) CreateSubscription(ctx context.Context, subscriptionCreated stripe.Subscription) error {
	ctx = observability.WithFields(ctx, observability.Field{"subscription_id", subscriptionCreated.ID})

	user, err := p.store.GetUserByStripeCustomerID(ctx, subscriptionCreated.Customer.ID)
	if err != nil {
		p.logger.Error(ctx, "error getting user by stripe id", err)
		return fmt.Errorf("error getting user by stripe id: %w", err)
	}
	ctx = observability.WithFields(ctx, observability.Field{"user_id", user.ID})

	price, err := p.store.GetPriceByStripeID(ctx, subscriptionCreated.Items.Data[0].Price.ID)
	if err != nil {
		p.logger.Error(ctx, "error getting price by stripe id", err)
		return fmt.Errorf("error getting price by stripe id: %w", err)
	}
	ctx = observability.WithFields(ctx, observability.Field{"price_id", price.ID})

	params := store.CreateSubscriptionsParams{
		UserID:          user.ID,
		PriceID:         price.ID,
		StripeID:        subscriptionCreated.ID,
		Status:          string(subscriptionCreated.Status),
		StartDate:       time.Unix(subscriptionCreated.CurrentPeriodStart, 0),
		EndDate:         time.Unix(subscriptionCreated.CurrentPeriodEnd, 0),
		NextBillingDate: time.Unix(subscriptionCreated.CurrentPeriodEnd, 0),
	}
	err = p.store.CreateSubscription(ctx, params)
	if err != nil {
		p.logger.Error(ctx, "error creating subscription", err)
		return fmt.Errorf("error creating subscription: %w", err)
	}

	return nil
}

func (p *SubscriptionService) UpdateSubscription(ctx context.Context, subscriptionUpdated stripe.Subscription) error {
	ctx = observability.WithFields(ctx, observability.Field{"subscription_id", subscriptionUpdated.ID})

	params := store.UpdateSubscriptionParams{
		Status:  string(subscriptionUpdated.Status),
		EndDate: time.Unix(subscriptionUpdated.CurrentPeriodEnd, 0),
		// This we should only update when invoice is paid
		NextBillingDate: time.Unix(subscriptionUpdated.CurrentPeriodEnd, 0),
		StripeID:        subscriptionUpdated.ID,
		StripePriceID:   subscriptionUpdated.Items.Data[0].Price.ID,
	}
	err := p.store.UpdateSubscription(ctx, params)
	if err != nil {
		p.logger.Error(ctx, "error updating subscription", err)
		return fmt.Errorf("error updating subscription: %w", err)
	}

	return nil
}

func (p *SubscriptionService) CancelSubscription(ctx context.Context, subscriptionID string, cancelAt time.Time) error {
	ctx = observability.WithFields(ctx, observability.Field{"subscription_id", subscriptionID})

	err := p.store.CancelSubscription(ctx, subscriptionID, cancelAt)
	if err != nil {
		p.logger.Error(ctx, "error cancelling subscription", err)
		return fmt.Errorf("error cancelling subscription: %w", err)
	}

	return nil
}

func (p *SubscriptionService) GetSubscriptionByID(ctx context.Context, subscriptionID string) (Subscription,
	error) {
	ctx = observability.WithFields(ctx, observability.Field{"subscription_id", subscriptionID})
	sub, err := p.store.GetSubscription(ctx, subscriptionID)
	if err != nil {
		p.logger.Error(ctx, "error getting subscription", err)
		return Subscription{}, fmt.Errorf("error getting subscription: %w", err)
	}

	return Subscription(sub), nil
}

func (p *SubscriptionService) GetSubscriptionByUserID(ctx context.Context, userID uuid.UUID) (Subscription,
	error) {
	sub, err := p.store.GetSubscriptionByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return Subscription{}, ErrNoSubscription
		}

		p.logger.Error(ctx, "error getting subscription", err)
		return Subscription{}, fmt.Errorf("error getting subscription: %w", err)
	}

	return Subscription(sub), nil
}
