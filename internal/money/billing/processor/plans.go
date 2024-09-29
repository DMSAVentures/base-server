package processor

import (
	"base-server/internal/money/products"
	"context"
)

func (p *BillingProcessor) ListPrices(ctx context.Context) ([]products.Price, error) {
	return p.productService.ListPrices(ctx)
}
