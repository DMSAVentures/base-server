package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Subscription struct {
	ID              uuid.UUID `db:"id"`
	UserID          uuid.UUID `db:"user_id"`
	PriceID         uuid.UUID `db:"price_id"`
	StripeID        string    `db:"stripe_id"`
	Status          string    `db:"status"`
	StartDate       time.Time `db:"start_date"`
	EndDate         time.Time `db:"end_date"`
	NextBillingDate time.Time `db:"next_billing_date"`
}

type CreateSubscriptionsParams struct {
	UserID          uuid.UUID
	PriceID         uuid.UUID
	StripeID        string
	Status          string
	StartDate       time.Time
	EndDate         time.Time
	NextBillingDate time.Time
}

type UpdateSubscriptionParams struct {
	Status          string
	EndDate         time.Time
	NextBillingDate time.Time
	StripeID        string
}

const sqlCreateSubscription = `
INSERT INTO subscriptions (user_id, price_id, stripe_id, status, start_date, end_date, next_billing_date)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, price_id, stripe_id, status, start_date, end_date, next_billing_date
`

// CreateSubscription creates a new subscription in the database.
func (s *Store) CreateSubscription(ctx context.Context, subscriptionCreated CreateSubscriptionsParams) error {
	_, err := s.db.ExecContext(ctx, sqlCreateSubscription,
		subscriptionCreated.UserID,
		subscriptionCreated.PriceID,
		subscriptionCreated.StripeID,
		subscriptionCreated.Status,
		subscriptionCreated.StartDate,
		subscriptionCreated.EndDate,
		subscriptionCreated.NextBillingDate)
	if err != nil {
		s.logger.Error(ctx, "failed to insert subscription", err)
		return fmt.Errorf("failed to insert subscription: %w", err)
	}
	return nil
}

const sqlUpdateSubscription = `
UPDATE subscriptions
SET status = $1, end_date = $2, next_billing_date = $3
WHERE stripe_id = $4
RETURNING id, user_id, price_id, stripe_id, status, start_date, end_date, next_billing_date
`

// UpdateSubscription updates a subscription in the database.
func (s *Store) UpdateSubscription(ctx context.Context, subscriptionUpdated UpdateSubscriptionParams) error {
	res, err := s.db.ExecContext(ctx, sqlUpdateSubscription,
		subscriptionUpdated.Status,
		subscriptionUpdated.EndDate,
		subscriptionUpdated.NextBillingDate,
		subscriptionUpdated.StripeID)
	if err != nil {
		s.logger.Error(ctx, "failed to update subscription", err)
		return fmt.Errorf("failed to update subscription: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		s.logger.Error(ctx, "subscription not found", nil)
		return fmt.Errorf("subscription not found")
	}

	return nil
}

const sqlCancelSubscription = `
UPDATE subscriptions
SET status = 'canceled'
WHERE stripe_id = $1`

// CancelSubscription cancels a subscription in the database.
func (s *Store) CancelSubscription(ctx context.Context, subscriptionID string) error {
	_, err := s.db.ExecContext(ctx, sqlCancelSubscription, subscriptionID)
	if err != nil {
		s.logger.Error(ctx, "failed to cancel subscription", err)
		return fmt.Errorf("failed to cancel subscription: %w", err)
	}

	return nil
}

const sqlGetSubscriptionByStripeID = `
SELECT id, user_id, price_id, stripe_id, status, start_date, end_date, next_billing_date
FROM subscriptions
WHERE stripe_id = $1
`

// GetSubscription retrieves a subscription from the database.
func (s *Store) GetSubscription(ctx context.Context, subscriptionID string) (Subscription, error) {
	var subscription Subscription
	err := s.db.QueryRowxContext(ctx, sqlGetSubscriptionByStripeID, subscriptionID).StructScan(&subscription)
	if err != nil {
		s.logger.Error(ctx, "failed to get subscription", err)
		return Subscription{}, fmt.Errorf("failed to get subscription: %w", err)
	}

	return subscription, nil
}
