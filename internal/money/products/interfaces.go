package products

import (
	"context"

	"github.com/stripe/stripe-go/v79"
)

// ProductServiceInterface defines the contract for product-related operations
type ProductServiceInterface interface {
	CreateProduct(ctx context.Context, productCreated stripe.Product) error
	CreatePrice(ctx context.Context, priceCreated stripe.Price) error
	UpdatePrice(ctx context.Context, priceUpdated stripe.Price) error
	DeletePrice(ctx context.Context, priceDeleted stripe.Price) error
	ListPrices(ctx context.Context) ([]Price, error)
}
