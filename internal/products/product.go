package products

import (
	"base-server/internal/store"
	"context"

	"github.com/stripe/stripe-go/v79"
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
	product, err := p.store.GetProductByStripeID(ctx, priceCreated.Product.ID)
	if err != nil {
		p.logger.Error(ctx, "failed to get product", err)
		return err
	}

	err = p.store.CreatePrice(ctx, store.Price{
		ProductID:   product.ID,
		StripeID:    priceCreated.ID,
		Description: priceCreated.Nickname,
	})
	if err != nil {
		p.logger.Error(ctx, "failed to create prices", err)
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
