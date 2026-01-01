package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Product struct {
	ID          uuid.UUID `db:"id"`
	StripeID    string    `db:"stripe_id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	CreatedAt   string    `db:"created_at"`
	UpdatedAt   string    `db:"updated_at"`
}

type Price struct {
	ID          uuid.UUID `db:"id"`
	ProductID   uuid.UUID `db:"product_id"`
	StripeID    string    `db:"stripe_id"`
	Description string    `db:"description"`
	UnitAmount  *int64    `db:"unit_amount"`
	Currency    *string   `db:"currency"`
	Interval    *string   `db:"interval"`
	CreatedAt   string    `db:"created_at"`
	UpdatedAt   string    `db:"updated_at"`
}

const sqlCreateProduct = `
INSERT INTO products (stripe_id, name, description)
VALUES ($1, $2, $3)
RETURNING id, stripe_id, name, description, created_at, updated_at
`

func (s *Store) CreateProduct(ctx context.Context, productID, name, description string) (Product, error) {
	var product Product
	err := s.db.GetContext(ctx, &product, sqlCreateProduct, productID, name, description)
	if err != nil {
		return Product{}, fmt.Errorf("failed to insert product: %w", err)
	}

	return product, nil

}

const sqlGetProductByStripeID = `
SELECT id, stripe_id, name, description, created_at, updated_at
FROM products
WHERE stripe_id = $1
`

func (s *Store) GetProductByStripeID(ctx context.Context, stripeID string) (Product, error) {
	var product Product
	err := s.db.GetContext(ctx, &product, sqlGetProductByStripeID, stripeID)
	if err != nil {
		return Product{}, fmt.Errorf("failed to get product: %w", err)
	}
	return product, nil
}
