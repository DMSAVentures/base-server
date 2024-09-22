package products

import (
	"base-server/internal/observability"
	"base-server/internal/store"

	"github.com/stripe/stripe-go/v79"
)

type ProductService struct {
	logger    observability.Logger
	stripeKey string
	store     store.Store
}

func New(stripeKey string, store store.Store, logger observability.Logger) ProductService {
	stripe.Key = stripeKey
	return ProductService{
		logger:    logger,
		stripeKey: stripeKey,
		store:     store,
	}
}
