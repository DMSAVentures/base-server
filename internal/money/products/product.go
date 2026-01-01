package products

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/product"
)

type Price struct {
	ProductID   uuid.UUID `json:"product_id"`
	PriceID     uuid.UUID `json:"price_id"`
	Description string    `json:"description"`
}

func (p *ProductService) CreateProduct(ctx context.Context, productCreated stripe.Product) error {
	ctx = observability.WithFields(ctx, observability.Field{Key: "product_id", Value: productCreated.ID})

	_, err := p.store.CreateProduct(ctx, productCreated.ID, productCreated.Name, productCreated.Description)
	if err != nil {
		p.logger.Error(ctx, "failed to create product", err)
		return fmt.Errorf("failed to create product: %w", err)
	}

	return nil
}

// HandlePriceCreationWebhookEvent handles the price creation webhook event
func (p *ProductService) CreatePrice(ctx context.Context, priceCreated stripe.Price) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "price_id", Value: priceCreated.ID},
		observability.Field{Key: "product_id", Value: priceCreated.Product.ID})

	// Try to get the product from the local database
	productInDB, err := p.store.GetProductByStripeID(ctx, priceCreated.Product.ID)
	if err != nil {
		// Log error but continue to fetch product from Stripe if it's not in the local database
		p.logger.Warn(ctx, "product not found in database, fetching from Stripe")

		// Fetch product from Stripe
		productGot, err := product.Get(priceCreated.Product.ID, nil)
		if err != nil {
			p.logger.Error(ctx, "failed to fetch product from Stripe", err)
			return fmt.Errorf("failed to fetch product from Stripe: %w", err)
		}

		// Create the product in the local database
		if err := p.CreateProduct(ctx, *productGot); err != nil {
			p.logger.Error(ctx, "failed to create product in local database", err)
			return fmt.Errorf("failed to create product in local database: %w", err)
		}

		// After creating the product, retrieve it from the local database
		productInDB, err = p.store.GetProductByStripeID(ctx, priceCreated.Product.ID)
		if err != nil {
			p.logger.Error(ctx, "failed to retrieve newly created product from local database", err)
			return fmt.Errorf("failed to retrieve newly created product from local database: %w", err)
		}
	}

	// Create the price in the local database
	price := store.Price{
		ProductID:   productInDB.ID,
		StripeID:    priceCreated.ID,
		Description: priceCreated.LookupKey,
	}

	if err := p.store.CreatePrice(ctx, price); err != nil {
		p.logger.Error(ctx, "failed to create price in database", err)
		return fmt.Errorf("failed to create price in database: %w", err)
	}

	return nil
}

// HandlePriceU handles the price update webhook event
func (p *ProductService) UpdatePrice(ctx context.Context, priceUpdated stripe.Price) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "price_id", Value: priceUpdated.ID},
		observability.Field{Key: "product_id", Value: priceUpdated.Product.ID})

	product, err := p.store.GetProductByStripeID(ctx, priceUpdated.Product.ID)
	if err != nil {
		p.logger.Error(ctx, "failed to find associated product", err)
		return fmt.Errorf("failed to find associalted product: %w", err)
	}

	err = p.store.UpdatePriceByStripeID(ctx, product.ID, priceUpdated.LookupKey, priceUpdated.ID)
	if err != nil {
		p.logger.Error(ctx, "failed to update price", err)
		return fmt.Errorf("failed to update price: %w", err)
	}

	return nil
}

// HandlePlanDeletionWebhookEvent handles the plan deletion webhook event
func (p *ProductService) DeletePrice(ctx context.Context, priceDeleted stripe.Price) error {
	ctx = observability.WithFields(ctx, observability.Field{Key: "price_id", Value: priceDeleted.ID})

	err := p.store.DeletePriceByStripeID(ctx, priceDeleted.ID)
	if err != nil {
		p.logger.Error(ctx, "failed to delete price", err)
		return fmt.Errorf("failed to delete price: %w", err)
	}

	return nil
}

func (p *ProductService) ListPrices(ctx context.Context) ([]Price, error) {
	prices, err := p.store.ListPrices(ctx)
	if err != nil {
		p.logger.Error(ctx, "failed to list prices", err)
		return nil, fmt.Errorf("failed to list prices: %w", err)
	}
	pricesTransformed := make([]Price, len(prices))
	for i, price := range prices {
		pricesTransformed[i] = Price{
			ProductID:   price.ProductID,
			PriceID:     price.ID,
			Description: price.Description,
		}
	}
	return pricesTransformed, nil
}
