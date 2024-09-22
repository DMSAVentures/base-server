package subscriptions

import (
	"base-server/internal/observability"
	"base-server/internal/store"
)

type SubscriptionService struct {
	logger    *observability.Logger
	stripeKey string
	store     *store.Store
}

func New(logger *observability.Logger, stripeKey string, store *store.Store) *SubscriptionService {
	return &SubscriptionService{
		logger:    logger,
		stripeKey: stripeKey,
		store:     store,
	}
}
