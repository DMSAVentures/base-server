package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// API key environment constants
const (
	APIKeyEnvironmentProduction  = "production"
	APIKeyEnvironmentStaging     = "staging"
	APIKeyEnvironmentDevelopment = "development"
)

// Usage operation constants for billing
const (
	UsageOperationUpdateScore    = "update_score"
	UsageOperationGetRank        = "get_rank"
	UsageOperationGetScore       = "get_score"
	UsageOperationGetTopN        = "get_top_n"
	UsageOperationGetUserCount   = "get_user_count"
	UsageOperationRemoveUser     = "remove_user"
	UsageOperationGetUsersAround = "get_users_around"
	UsageOperationSyncDatabase   = "sync_database"
)

// AccountAPIKey represents an API key for account authentication
type AccountAPIKey struct {
	ID        uuid.UUID `db:"id" json:"id"`
	AccountID uuid.UUID `db:"account_id" json:"account_id"`

	KeyHash   string `db:"key_hash" json:"-"` // Never expose the hash
	KeyPrefix string `db:"key_prefix" json:"key_prefix"`

	Name        string `db:"name" json:"name"`
	Environment string `db:"environment" json:"environment"`
	Scopes      JSONB  `db:"scopes" json:"scopes"`

	LastUsedAt *time.Time `db:"last_used_at" json:"last_used_at,omitempty"`
	UsageCount int        `db:"usage_count" json:"usage_count"`

	IsActive  bool       `db:"is_active" json:"is_active"`
	ExpiresAt *time.Time `db:"expires_at" json:"expires_at,omitempty"`

	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// UsageEvent represents a single API operation for billing
type UsageEvent struct {
	ID        uuid.UUID  `db:"id" json:"id"`
	AccountID uuid.UUID  `db:"account_id" json:"account_id"`
	CampaignID *uuid.UUID `db:"campaign_id" json:"campaign_id,omitempty"`
	APIKeyID  *uuid.UUID `db:"api_key_id" json:"api_key_id,omitempty"`

	Operation string `db:"operation" json:"operation"`
	Count     int    `db:"count" json:"count"`

	RequestID      *uuid.UUID `db:"request_id" json:"request_id,omitempty"`
	IPAddress      *string    `db:"ip_address" json:"ip_address,omitempty"`
	UserAgent      *string    `db:"user_agent" json:"user_agent,omitempty"`
	ResponseTimeMs *int       `db:"response_time_ms" json:"response_time_ms,omitempty"`
	StatusCode     *int       `db:"status_code" json:"status_code,omitempty"`

	BillingDate time.Time `db:"billing_date" json:"billing_date"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

// UsageAggregate represents pre-aggregated usage data for billing
type UsageAggregate struct {
	ID        uuid.UUID `db:"id" json:"id"`
	AccountID uuid.UUID `db:"account_id" json:"account_id"`

	PeriodStart time.Time `db:"period_start" json:"period_start"`
	PeriodEnd   time.Time `db:"period_end" json:"period_end"`

	TotalOperations  int   `db:"total_operations" json:"total_operations"`
	OperationsByType JSONB `db:"operations_by_type" json:"operations_by_type"`
	TotalCostCents   int   `db:"total_cost_cents" json:"total_cost_cents"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// RateLimit represents rate limit tracking for an account
type RateLimit struct {
	ID        uuid.UUID `db:"id" json:"id"`
	AccountID uuid.UUID `db:"account_id" json:"account_id"`

	WindowStart time.Time `db:"window_start" json:"window_start"`
	WindowEnd   time.Time `db:"window_end" json:"window_end"`

	RequestsCount int  `db:"requests_count" json:"requests_count"`
	RequestsLimit int  `db:"requests_limit" json:"requests_limit"`
	IsThrottled   bool `db:"is_throttled" json:"is_throttled"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// ============================================================================
// ACCOUNT API KEY OPERATIONS
// ============================================================================

// CreateAPIKeyParams represents parameters for creating an API key
type CreateAPIKeyParams struct {
	AccountID   uuid.UUID
	KeyHash     string
	KeyPrefix   string
	Name        string
	Environment string
	Scopes      JSONB
	ExpiresAt   *time.Time
}

// CreateAPIKey creates a new API key for an account
func (s Store) CreateAPIKey(ctx context.Context, params CreateAPIKeyParams) (AccountAPIKey, error) {
	var apiKey AccountAPIKey
	query := `
		INSERT INTO account_api_keys (account_id, key_hash, key_prefix, name, environment, scopes, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, account_id, key_hash, key_prefix, name, environment, scopes,
		          last_used_at, usage_count, is_active, expires_at, created_at, updated_at, deleted_at
	`

	err := s.db.GetContext(ctx, &apiKey, query,
		params.AccountID,
		params.KeyHash,
		params.KeyPrefix,
		params.Name,
		params.Environment,
		params.Scopes,
		params.ExpiresAt,
	)

	if err != nil {
		return AccountAPIKey{}, fmt.Errorf("failed to create API key: %w", err)
	}

	return apiKey, nil
}

// GetAPIKeyByHash retrieves an API key by its hash
func (s Store) GetAPIKeyByHash(ctx context.Context, keyHash string) (AccountAPIKey, error) {
	var apiKey AccountAPIKey
	query := `
		SELECT id, account_id, key_hash, key_prefix, name, environment, scopes,
		       last_used_at, usage_count, is_active, expires_at, created_at, updated_at, deleted_at
		FROM account_api_keys
		WHERE key_hash = $1 AND deleted_at IS NULL AND is_active = true
	`

	err := s.db.GetContext(ctx, &apiKey, query, keyHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return AccountAPIKey{}, ErrNotFound
		}
		return AccountAPIKey{}, fmt.Errorf("failed to get API key: %w", err)
	}

	// Check if key is expired
	if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
		return AccountAPIKey{}, ErrNotFound
	}

	return apiKey, nil
}

// ListAPIKeysByAccount retrieves all API keys for an account
func (s Store) ListAPIKeysByAccount(ctx context.Context, accountID uuid.UUID) ([]AccountAPIKey, error) {
	var apiKeys []AccountAPIKey
	query := `
		SELECT id, account_id, key_hash, key_prefix, name, environment, scopes,
		       last_used_at, usage_count, is_active, expires_at, created_at, updated_at, deleted_at
		FROM account_api_keys
		WHERE account_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	err := s.db.SelectContext(ctx, &apiKeys, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	return apiKeys, nil
}

// UpdateAPIKeyUsage updates the last used timestamp and usage count
func (s Store) UpdateAPIKeyUsage(ctx context.Context, apiKeyID uuid.UUID) error {
	query := `
		UPDATE account_api_keys
		SET last_used_at = CURRENT_TIMESTAMP,
		    usage_count = usage_count + 1
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := s.db.ExecContext(ctx, query, apiKeyID)
	if err != nil {
		return fmt.Errorf("failed to update API key usage: %w", err)
	}

	return nil
}

// RevokeAPIKey deactivates an API key
func (s Store) RevokeAPIKey(ctx context.Context, accountID, apiKeyID uuid.UUID) error {
	query := `
		UPDATE account_api_keys
		SET is_active = false,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND account_id = $2 AND deleted_at IS NULL
	`

	result, err := s.db.ExecContext(ctx, query, apiKeyID, accountID)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// ============================================================================
// USAGE EVENT OPERATIONS
// ============================================================================

// CreateUsageEventParams represents parameters for creating a usage event
type CreateUsageEventParams struct {
	AccountID      uuid.UUID
	CampaignID     *uuid.UUID
	APIKeyID       *uuid.UUID
	Operation      string
	Count          int
	RequestID      *uuid.UUID
	IPAddress      *string
	UserAgent      *string
	ResponseTimeMs *int
	StatusCode     *int
}

// CreateUsageEvent creates a new usage event
func (s Store) CreateUsageEvent(ctx context.Context, params CreateUsageEventParams) error {
	query := `
		INSERT INTO usage_events (
			account_id, campaign_id, api_key_id, operation, count,
			request_id, ip_address, user_agent, response_time_ms, status_code,
			billing_date
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, CURRENT_DATE)
	`

	_, err := s.db.ExecContext(ctx, query,
		params.AccountID,
		params.CampaignID,
		params.APIKeyID,
		params.Operation,
		params.Count,
		params.RequestID,
		params.IPAddress,
		params.UserAgent,
		params.ResponseTimeMs,
		params.StatusCode,
	)

	if err != nil {
		return fmt.Errorf("failed to create usage event: %w", err)
	}

	return nil
}

// GetUsageByAccountAndPeriod retrieves aggregated usage for an account in a period
func (s Store) GetUsageByAccountAndPeriod(ctx context.Context, accountID uuid.UUID, startDate, endDate time.Time) (map[string]int, error) {
	query := `
		SELECT operation, SUM(count) as total
		FROM usage_events
		WHERE account_id = $1
		  AND billing_date >= $2
		  AND billing_date <= $3
		GROUP BY operation
	`

	type OperationCount struct {
		Operation string `db:"operation"`
		Total     int    `db:"total"`
	}

	var results []OperationCount
	err := s.db.SelectContext(ctx, &results, query, accountID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage: %w", err)
	}

	usage := make(map[string]int)
	for _, result := range results {
		usage[result.Operation] = result.Total
	}

	return usage, nil
}
