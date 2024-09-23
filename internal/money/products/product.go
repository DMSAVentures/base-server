package products

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"fmt"

	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/product"
)

func (p *ProductService) CreateProduct(ctx context.Context, productCreated stripe.Product) error {
	ctx = observability.WithFields(ctx, observability.Field{"product_id", productCreated.ID})

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
		observability.Field{"price_id", priceCreated.ID},
		observability.Field{"product_id", priceCreated.Product.ID})

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
		Description: priceCreated.Nickname,
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
		observability.Field{"price_id", priceUpdated.ID},
		observability.Field{"product_id", priceUpdated.Product.ID})

	product, err := p.store.GetProductByStripeID(ctx, priceUpdated.Product.ID)
	if err != nil {
		p.logger.Error(ctx, "failed to find associated product", err)
		return fmt.Errorf("failed to find associalted product: %w", err)
	}

	err = p.store.UpdatePriceByStripeID(ctx, product.ID, priceUpdated.Nickname, priceUpdated.ID)
	if err != nil {
		p.logger.Error(ctx, "failed to update price", err)
		return fmt.Errorf("failed to update price: %w", err)
	}

	return nil
}

// HandlePlanDeletionWebhookEvent handles the plan deletion webhook event
func (p *ProductService) DeletePrice(ctx context.Context, priceDeleted stripe.Price) error {
	ctx = observability.WithFields(ctx, observability.Field{"price_id", priceDeleted.ID})

	err := p.store.DeletePriceByStripeID(ctx, priceDeleted.ID)
	if err != nil {
		p.logger.Error(ctx, "failed to delete price", err)
		return fmt.Errorf("failed to delete price: %w", err)
	}

	return nil
}
