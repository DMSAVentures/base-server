package subscriptions

import (
	"base-server/internal/store"
	"context"
	"time"

	"github.com/stripe/stripe-go/v79"
)

func (p *SubscriptionService) CreateSubscription(ctx context.Context, subscriptionCreated stripe.Subscription) error {
	user, err := p.store.GetUserByStripeCustomerID(ctx, subscriptionCreated.Customer.ID)
	if err != nil {
		p.logger.Error(ctx, "error getting user by stripe id", err)
		return err
	}

	price, err := p.store.GetPriceByStripeID(ctx, subscriptionCreated.Items.Data[0].Price.ID)
	if err != nil {
		p.logger.Error(ctx, "error getting price by stripe id", err)
		return err
	}

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
		return err
	}
	return nil
}

func (p *SubscriptionService) UpdateSubscription(ctx context.Context, subscriptionUpdated stripe.Subscription) error {
	params := store.UpdateSubscriptionParams{
		Status:          string(subscriptionUpdated.Status),
		EndDate:         time.Unix(subscriptionUpdated.CurrentPeriodEnd, 0),
		NextBillingDate: time.Unix(subscriptionUpdated.CurrentPeriodEnd, 0),
		StripeID:        subscriptionUpdated.ID,
	}
	err := p.store.UpdateSubscription(ctx, params)
	if err != nil {
		p.logger.Error(ctx, "error updating subscription", err)
		return err
	}

	return nil
}

func (p *SubscriptionService) CancelSubscription(ctx context.Context, subscriptionID string) error {
	return p.store.CancelSubscription(ctx, subscriptionID)
}

func (p *SubscriptionService) GetSubscription(ctx context.Context, subscriptionID string) (store.Subscription,
	error) {
	return p.store.GetSubscription(ctx, subscriptionID)
}
