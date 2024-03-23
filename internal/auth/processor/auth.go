package processor

import "base-server/internal/store"

type AuthProcessor struct {
	store store.Store
}

func New(store store.Store) AuthProcessor {
	return AuthProcessor{
		store: store,
	}
}
