package subscriptions

import (
	"base-server/internal/observability"
	"base-server/internal/store"
)

type SubscriptionService struct {
	logger    *observability.Logger
	stripeKey string
	store     store.Storer
}

func New(logger *observability.Logger, stripeKey string, store store.Storer) SubscriptionService {
	return SubscriptionService{
		logger:    logger,
		stripeKey: stripeKey,
		store:     store,
	}
}
