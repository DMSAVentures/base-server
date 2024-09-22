package products

import (
	"context"

	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/price"
	"github.com/stripe/stripe-go/v79/product"
)

func (p *ProductService) HandleProductCreationWebhookEvent(ctx context.Context, productCreated stripe.Product) error {
	// Handle product creation
	// 1. Create a new product in the system (pseudo-code)
	prod, err := product.Get(productCreated.ID, nil)
	if err != nil {
		p.logger.Error(ctx, "failed to get product", err)
		return err
	}

	// List prices for the product
	params := &stripe.PriceListParams{
		Product: stripe.String(prod.ID),
	}
	i := price.List(params)

	var prices []*stripe.Price
	for i.Next() {
		prices = append(prices, i.Price())
	}

	if err := i.Err(); err != nil {
		p.logger.Error(ctx, "failed to list prices", err)
		return err
	}

	err = p.store.CreateProduct(ctx, productCreated.ID, productCreated.Name, productCreated.Description)
	if err != nil {
		p.logger.Error(ctx, "failed to create product", err)
		return err
	}

	err = p.store.CreatePrices(ctx, prices)
	if err != nil {
		p.logger.Error(ctx, "failed to create prices", err)
		return err
	}
}
