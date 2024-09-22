package store

import (
	"context"

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
		return User{}, err
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
		return err
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
		return "", err
	}
	return stripeCustomerID, nil
}
