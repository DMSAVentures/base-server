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
		return fmt.Errorf("failed to update price: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
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

		return nil, fmt.Errorf("failed to list prices: %w", err)
	}

	return prices, nil
}

const sqlGetPriceByID = `
SELECT id, product_id, stripe_id, description, created_at, updated_at
FROM prices
WHERE id = $1
`

func (s *Store) GetPriceByID(ctx context.Context, priceID string) (Price, error) {
	var price Price
	err := s.db.GetContext(ctx, &price, sqlGetPriceByID, priceID)
	if err != nil {
		return Price{}, fmt.Errorf("failed to get price: %w", err)
	}
	return price, nil
}

const sqlGetFreePriceStripeID = `
SELECT stripe_id
FROM prices
WHERE description = 'free' AND deleted_at IS NULL
LIMIT 1
`

func (s *Store) GetFreePriceStripeID(ctx context.Context) (string, error) {
	var stripeID string
	err := s.db.GetContext(ctx, &stripeID, sqlGetFreePriceStripeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("failed to get free price stripe id: %w", err)
	}
	return stripeID, nil
}
