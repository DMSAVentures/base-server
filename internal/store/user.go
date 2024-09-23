package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

const sqlSelectUserByID = `
SELECT 
    id,
    first_name,
    last_name
FROM users
WHERE id = $1`

func (s *Store) GetUserByExternalID(ctx context.Context, externalID uuid.UUID) (User, error) {
	var user User
	err := s.db.GetContext(ctx, &user, sqlSelectUserByID, externalID)
	if err != nil {
		s.logger.Error(ctx, "failed to get user by external ID", err)
		return User{}, fmt.Errorf("failed to get user by external ID: %w", err)
	}

	return user, nil
}

const sqlUpdateStripeCustomerIDByUserID = `
UPDATE users
SET stripe_customer_id = $1
WHERE id = $2`

func (s *Store) UpdateStripeCustomerIDByUserID(ctx context.Context, userID uuid.UUID, stripeCustomerID string) error {
	_, err := s.db.ExecContext(ctx, sqlUpdateStripeCustomerIDByUserID, stripeCustomerID, userID)
	if err != nil {
		s.logger.Error(ctx, "failed to update stripe customer ID by user ID", err)
		return fmt.Errorf("failed to update stripe customer ID by user ID: %w", err)
	}
	return nil
}

const sqlGetStripeCustomerIDByUserID = `
SELECT stripe_customer_id
FROM users
WHERE id = $1`

func (s *Store) GetStripeCustomerIDByUserExternalID(ctx context.Context, ID uuid.UUID) (string, error) {
	var stripeCustomerID string
	err := s.db.GetContext(ctx, &stripeCustomerID, sqlGetStripeCustomerIDByUserID, ID)
	if err != nil {
		s.logger.Error(ctx, "failed to get stripe customer ID by user ID", err)
		return "", fmt.Errorf("failed to get stripe customer ID by user ID: %w", err)
	}

	return stripeCustomerID, nil
}

const sqlSelectUserByStripeCustomerID = `
SELECT 
    id,
    first_name,
    last_name
FROM users
WHERE stripe_customer_id = $1`

func (s *Store) GetUserByStripeCustomerID(ctx context.Context, stripeID string) (User, error) {
	var user User
	err := s.db.GetContext(ctx, &user, sqlSelectUserByStripeCustomerID, stripeID)
	if err != nil {
		s.logger.Error(ctx, "failed to get user by stripe customer ID", err)
		return User{}, fmt.Errorf("failed to get user by stripe customer ID: %w", err)
	}

	return user, nil
}
