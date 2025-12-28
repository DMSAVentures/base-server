package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Feature represents a feature definition
type Feature struct {
	ID          uuid.UUID  `db:"id" json:"id"`
	Name        string     `db:"name" json:"name"`
	Description *string    `db:"description" json:"description"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at" json:"deleted_at"`
}

// Limit represents a limit definition
type Limit struct {
	ID         uuid.UUID  `db:"id" json:"id"`
	FeatureID  uuid.UUID  `db:"feature_id" json:"feature_id"`
	LimitName  string     `db:"limit_name" json:"limit_name"`
	LimitValue int        `db:"limit_value" json:"limit_value"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt  *time.Time `db:"deleted_at" json:"deleted_at"`
}

// PlanFeatureLimit represents a feature-limit mapping for a price/plan
type PlanFeatureLimit struct {
	PlanID    uuid.UUID  `db:"plan_id" json:"plan_id"`
	FeatureID uuid.UUID  `db:"feature_id" json:"feature_id"`
	LimitID   *uuid.UUID `db:"limit_id" json:"limit_id"`
	Enabled   bool       `db:"enabled" json:"enabled"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at"`
}

// PlanFeatureWithLimit combines plan_feature_limits with feature and limit info
type PlanFeatureWithLimit struct {
	FeatureName string `db:"feature_name"`
	Enabled     bool   `db:"enabled"`
	LimitValue  *int   `db:"limit_value"` // nil means unlimited or no limit
}

// TierInfo represents combined tier information for an account
type TierInfo struct {
	PriceID          uuid.UUID       `json:"price_id"`
	PriceDescription string          `json:"price_description"`
	Features         map[string]bool `json:"features"`
	Limits           map[string]*int `json:"limits"` // nil means unlimited
}

const sqlGetPlanFeaturesWithLimitsByPriceID = `
SELECT
    f.name as feature_name,
    pfl.enabled,
    l.limit_value
FROM plan_feature_limits pfl
JOIN features f ON f.id = pfl.feature_id
LEFT JOIN limits l ON l.id = pfl.limit_id
WHERE pfl.plan_id = $1
    AND pfl.deleted_at IS NULL
    AND f.deleted_at IS NULL
    AND (l.deleted_at IS NULL OR l.id IS NULL)
`

// GetPlanFeaturesWithLimitsByPriceID retrieves all features and their limits for a given price
func (s *Store) GetPlanFeaturesWithLimitsByPriceID(ctx context.Context, priceID uuid.UUID) ([]PlanFeatureWithLimit, error) {
	var results []PlanFeatureWithLimit
	err := s.db.SelectContext(ctx, &results, sqlGetPlanFeaturesWithLimitsByPriceID, priceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan features with limits: %w", err)
	}
	return results, nil
}

const sqlGetPriceDescriptionByID = `
SELECT description
FROM prices
WHERE id = $1 AND deleted_at IS NULL
`

// GetPriceDescriptionByID retrieves the price description for a given price ID
func (s *Store) GetPriceDescriptionByID(ctx context.Context, priceID uuid.UUID) (string, error) {
	var description string
	err := s.db.GetContext(ctx, &description, sqlGetPriceDescriptionByID, priceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("failed to get price description: %w", err)
	}
	return description, nil
}

// GetTierInfoByPriceID retrieves complete tier information for a price
func (s *Store) GetTierInfoByPriceID(ctx context.Context, priceID uuid.UUID) (TierInfo, error) {
	// Get price description
	priceDescription, err := s.GetPriceDescriptionByID(ctx, priceID)
	if err != nil {
		return TierInfo{}, fmt.Errorf("failed to get price description: %w", err)
	}

	// Get features and limits
	featuresWithLimits, err := s.GetPlanFeaturesWithLimitsByPriceID(ctx, priceID)
	if err != nil {
		return TierInfo{}, fmt.Errorf("failed to get features with limits: %w", err)
	}

	// Build the response
	tierInfo := TierInfo{
		PriceID:          priceID,
		PriceDescription: priceDescription,
		Features:         make(map[string]bool),
		Limits:           make(map[string]*int),
	}

	// Resource features that have limits
	resourceFeatures := map[string]bool{
		"campaigns":    true,
		"leads":        true,
		"team_members": true,
	}

	for _, f := range featuresWithLimits {
		if resourceFeatures[f.FeatureName] {
			// This is a resource with a limit
			tierInfo.Limits[f.FeatureName] = f.LimitValue
		} else {
			// This is a boolean feature
			tierInfo.Features[f.FeatureName] = f.Enabled
		}
	}

	return tierInfo, nil
}

// GetTierInfoByUserID retrieves tier information for a user via their subscription
func (s *Store) GetTierInfoByUserID(ctx context.Context, userID uuid.UUID) (TierInfo, error) {
	// Get user's active subscription
	subscription, err := s.GetSubscriptionByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// No subscription - return free tier info
			return s.GetFreeTierInfo(ctx)
		}
		return TierInfo{}, fmt.Errorf("failed to get subscription: %w", err)
	}

	// Only consider active subscriptions
	if subscription.Status != "active" && subscription.Status != "trialing" {
		return s.GetFreeTierInfo(ctx)
	}

	return s.GetTierInfoByPriceID(ctx, subscription.PriceID)
}

// GetTierInfoByAccountID retrieves tier information for an account
// Flow: Account → owner_user_id → User → Subscription → Price → Features/Limits
func (s *Store) GetTierInfoByAccountID(ctx context.Context, accountID uuid.UUID) (TierInfo, error) {
	// Get the account to find the owner
	account, err := s.GetAccountByID(ctx, accountID)
	if err != nil {
		return TierInfo{}, fmt.Errorf("failed to get account: %w", err)
	}

	// Get tier info via the owner's subscription
	return s.GetTierInfoByUserID(ctx, account.OwnerUserID)
}

const sqlGetFreePriceID = `
SELECT id
FROM prices
WHERE description = 'free' AND deleted_at IS NULL
LIMIT 1
`

// GetFreeTierInfo returns the free tier configuration
func (s *Store) GetFreeTierInfo(ctx context.Context) (TierInfo, error) {
	// Get the free price ID
	var freePriceID uuid.UUID
	err := s.db.GetContext(ctx, &freePriceID, sqlGetFreePriceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Return default free tier if not configured in DB
			return getDefaultFreeTier(), nil
		}
		return TierInfo{}, fmt.Errorf("failed to get free price: %w", err)
	}

	return s.GetTierInfoByPriceID(ctx, freePriceID)
}

// getDefaultFreeTier returns a hardcoded free tier as fallback
func getDefaultFreeTier() TierInfo {
	campaigns := 1
	leads := 200
	teamMembers := 1

	return TierInfo{
		PriceDescription: "free",
		Features: map[string]bool{
			"email_verification":   false,
			"referral_system":      false,
			"visual_form_builder":  true,
			"visual_email_builder": false,
			"all_widget_types":     false,
			"remove_branding":      false,
			"anti_spam_protection": false,
			"enhanced_lead_data":   false,
			"tracking_pixels":      false,
			"webhooks_zapier":      false,
			"email_blasts":         false,
			"json_export":          false,
		},
		Limits: map[string]*int{
			"campaigns":    &campaigns,
			"leads":        &leads,
			"team_members": &teamMembers,
		},
	}
}

// HasFeature checks if a price/plan has access to a specific feature
func (s *Store) HasFeature(ctx context.Context, priceID uuid.UUID, featureName string) (bool, error) {
	tierInfo, err := s.GetTierInfoByPriceID(ctx, priceID)
	if err != nil {
		return false, err
	}

	enabled, exists := tierInfo.Features[featureName]
	if !exists {
		return false, nil
	}
	return enabled, nil
}

// GetLimit returns the limit value for a price/plan (nil means unlimited)
func (s *Store) GetLimit(ctx context.Context, priceID uuid.UUID, limitName string) (*int, error) {
	tierInfo, err := s.GetTierInfoByPriceID(ctx, priceID)
	if err != nil {
		return nil, err
	}

	limit, exists := tierInfo.Limits[limitName]
	if !exists {
		return nil, nil
	}
	return limit, nil
}
