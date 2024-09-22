package products

import (
	"base-server/internal/store"
	"context"

	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/product"
)

func (p *ProductService) CreateProduct(ctx context.Context, productCreated stripe.Product) error {

	_, err := p.store.CreateProduct(ctx, productCreated.ID, productCreated.Name, productCreated.Description)
	if err != nil {
		p.logger.Error(ctx, "failed to create product", err)
		return err
	}
	return nil
}

// HandlePriceCreationWebhookEvent handles the price creation webhook event
func (p *ProductService) CreatePrice(ctx context.Context, priceCreated stripe.Price) error {
	// Try to get the product from the local database
	productInDB, err := p.store.GetProductByStripeID(ctx, priceCreated.Product.ID)
	if err != nil {
		// Log error but continue to fetch product from Stripe if it's not in the local database
		p.logger.Warn(ctx, "product not found in database, fetching from Stripe")

		// Fetch product from Stripe
		productGot, err := product.Get(priceCreated.Product.ID, nil)
		if err != nil {
			p.logger.Error(ctx, "failed to fetch product from Stripe", err)
			return err
		}

		// Create the product in the local database
		if err := p.CreateProduct(ctx, *productGot); err != nil {
			p.logger.Error(ctx, "failed to create product in local database", err)
			return err
		}

		// After creating the product, retrieve it from the local database
		productInDB, err = p.store.GetProductByStripeID(ctx, priceCreated.Product.ID)
		if err != nil {
			p.logger.Error(ctx, "failed to retrieve newly created product from local database", err)
			return err
		}
	}

	// Create the price in the local database
	price := store.Price{
		ProductID:   productInDB.ID,
		StripeID:    priceCreated.ID,
		Description: priceCreated.Nickname,
	}

	if err := p.store.CreatePrice(ctx, price); err != nil {
		p.logger.Error(ctx, "failed to create price in local database", err)
		return err
	}

	return nil
}

// HandlePriceU handles the price update webhook event
func (p *ProductService) UpdatePrice(ctx context.Context, priceUpdated stripe.Price) error {
	product, err := p.store.GetProductByStripeID(ctx, priceUpdated.Product.ID)
	if err != nil {
		p.logger.Error(ctx, "failed to get product", err)
	}

	err = p.store.UpdatePriceByStripeID(ctx, product.ID, priceUpdated.Nickname, priceUpdated.ID)
	if err != nil {
		p.logger.Error(ctx, "failed to update price", err)
		return err
	}
	return nil
}

// HandlePlanDeletionWebhookEvent handles the plan deletion webhook event
func (p *ProductService) DeletePrice(ctx context.Context, priceDeleted stripe.Price) error {
	err := p.store.DeletePriceByStripeID(ctx, priceDeleted.ID)
	if err != nil {
		p.logger.Error(ctx, "failed to delete price", err)
		return err
	}
	return nil
}
