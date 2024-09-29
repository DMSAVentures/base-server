package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

const sqlCreatePrice = `
INSERT INTO prices (product_id, stripe_id, description)
VALUES ($1, $2, $3)
RETURNING id, product_id, stripe_id, description, created_at, updated_at`

func (s *Store) CreatePrice(ctx context.Context, price Price) error {
	_, err := s.db.ExecContext(ctx, sqlCreatePrice, price.ProductID, price.StripeID, price.Description)
	if err != nil {
		s.logger.Error(ctx, "failed to insert price", err)
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
	result, err := s.db.ExecContext(ctx, sqlUpdatePriceByStripeID, productID, description, stripeID)
	if err != nil {
		s.logger.Error(ctx, "failed to update price", err)
		return fmt.Errorf("failed to update price: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		s.logger.Error(ctx, "price not found", nil)
		return fmt.Errorf("price not found")
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
		s.logger.Error(ctx, "failed to delete price", err)
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
		s.logger.Error(ctx, "failed to get price", err)
		return Price{}, fmt.Errorf("failed to get price: %w", err)
	}
	return price, nil
}

const sqlGetAllPrices = `
SELECT id, product_id, stripe_id, description, created_at, updated_at
FROM prices
`

func (s *Store) ListPrices(ctx context.Context) ([]Price, error) {
	var prices []Price
	err := s.db.SelectContext(ctx, &prices, sqlGetAllPrices)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		s.logger.Error(ctx, "failed to list prices", err)
		return nil, fmt.Errorf("failed to list prices: %w", err)
	}

	return prices, nil
}
