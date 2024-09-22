package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

const sqlCreatePrice = `
INSERT INTO prices (product_id, stripe_id, description)
VALUES ($1, $2, $3)
RETURNING id, product_id, stripe_id, description, created_at, updated_at`

func (s *Store) CreatePrice(ctx context.Context, prices Price) error {
	_, err := s.db.ExecContext(ctx, sqlCreatePrice, prices.ProductID, prices.StripeID, prices.Description)
	if err != nil {
		return fmt.Errorf("failed to insert price: %w", err)
	}
	return nil
}

const sqlUpdatePriceByStripeID = `
UPDATE prices
SET product_id = $1, description = $2
WHERE stripe_id = $3
`

func (s *Store) UpdatePriceByStripeID(ctx context.Context, productID uuid.UUID, description, stripeID string) error {
	_, err := s.db.ExecContext(ctx, sqlUpdatePriceByStripeID, productID, description, stripeID)
	if err != nil {
		return fmt.Errorf("failed to update price: %w", err)
	}
	return nil
}

const sqlDeletePriceByStripeID = `
DELETE FROM prices
WHERE stripe_id = $1
`

func (s *Store) DeletePriceByStripeID(ctx context.Context, stripeID string) error {
	_, err := s.db.ExecContext(ctx, sqlDeletePriceByStripeID, stripeID)
	if err != nil {
		return fmt.Errorf("failed to delete price: %w", err)
	}
	return nil
}

const sqlGetPriceByStripeID = `
SELECT id, product_id, stripe_id, description, created_at, updated_at
FROM prices
WHERE stripe_id = $1
`

func (s *Store) GetPriceByStripeID(ctx context.Context, stripeID string) (Price, error) {
	var price Price
	err := s.db.GetContext(ctx, &price, sqlGetPriceByStripeID, stripeID)
	if err != nil {
		return Price{}, fmt.Errorf("failed to get price: %w", err)
	}
	return price, nil
}
