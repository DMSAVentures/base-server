package tiers

import (
	"context"

	"base-server/internal/observability"
	"base-server/internal/store"

	"github.com/google/uuid"
)

// TierStore defines the database operations required by TierService
type TierStore interface {
	GetTierInfoByAccountID(ctx context.Context, accountID uuid.UUID) (store.TierInfo, error)
	GetTierInfoByUserID(ctx context.Context, userID uuid.UUID) (store.TierInfo, error)
	GetTierInfoByPriceID(ctx context.Context, priceID uuid.UUID) (store.TierInfo, error)
	GetFreeTierInfo(ctx context.Context) (store.TierInfo, error)
}

// TierService handles tier-related business logic
type TierService struct {
	store  TierStore
	logger *observability.Logger
}

// New creates a new TierService
func New(store TierStore, logger *observability.Logger) *TierService {
	return &TierService{
		store:  store,
		logger: logger,
	}
}

// TierInfo represents the complete tier information for a user/account
type TierInfo struct {
	TierName         string          `json:"tier_name"`
	DisplayName      string          `json:"display_name"`
	PriceDescription string          `json:"price_description"`
	Features         map[string]bool `json:"features"`
	Limits           map[string]*int `json:"limits"`
}

// GetTierInfoByAccountID retrieves complete tier information for an account
func (s *TierService) GetTierInfoByAccountID(ctx context.Context, accountID uuid.UUID) (TierInfo, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "operation", Value: "get_tier_info"},
		observability.Field{Key: "account_id", Value: accountID.String()},
	)

	storeTierInfo, err := s.store.GetTierInfoByAccountID(ctx, accountID)
	if err != nil {
		s.logger.Error(ctx, "failed to get tier info by account id", err)
		return TierInfo{}, err
	}

	return s.convertStoreTierInfo(storeTierInfo), nil
}

// GetTierInfoByUserID retrieves tier information for a user via their subscription
func (s *TierService) GetTierInfoByUserID(ctx context.Context, userID uuid.UUID) (TierInfo, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "operation", Value: "get_tier_info_by_user"},
		observability.Field{Key: "user_id", Value: userID.String()},
	)

	storeTierInfo, err := s.store.GetTierInfoByUserID(ctx, userID)
	if err != nil {
		s.logger.Error(ctx, "failed to get tier info by user id", err)
		return TierInfo{}, err
	}

	return s.convertStoreTierInfo(storeTierInfo), nil
}

// GetFreeTierInfo returns the free tier configuration
func (s *TierService) GetFreeTierInfo(ctx context.Context) (TierInfo, error) {
	storeTierInfo, err := s.store.GetFreeTierInfo(ctx)
	if err != nil {
		s.logger.Error(ctx, "failed to get free tier info", err)
		return TierInfo{}, err
	}

	return s.convertStoreTierInfo(storeTierInfo), nil
}

// convertStoreTierInfo converts store.TierInfo to the service's TierInfo struct
func (s *TierService) convertStoreTierInfo(storeTierInfo store.TierInfo) TierInfo {
	tierName := GetTierForPriceDescription(storeTierInfo.PriceDescription)

	return TierInfo{
		TierName:         string(tierName),
		DisplayName:      GetTierDisplayName(tierName),
		PriceDescription: storeTierInfo.PriceDescription,
		Features:         storeTierInfo.Features,
		Limits:           storeTierInfo.Limits,
	}
}

// HasFeatureByAccountID checks if an account has access to a specific feature
func (s *TierService) HasFeatureByAccountID(ctx context.Context, accountID uuid.UUID, featureName string) (bool, error) {
	tierInfo, err := s.GetTierInfoByAccountID(ctx, accountID)
	if err != nil {
		return false, err
	}

	return tierInfo.Features[featureName], nil
}

// GetLimitByAccountID returns the limit value for an account (nil means unlimited)
func (s *TierService) GetLimitByAccountID(ctx context.Context, accountID uuid.UUID, limitName string) (*int, error) {
	tierInfo, err := s.GetTierInfoByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}

	return tierInfo.Limits[limitName], nil
}
