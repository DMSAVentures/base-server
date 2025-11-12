package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Customer plan constants
const (
	CustomerPlanFree       = "free"
	CustomerPlanStarter    = "starter"
	CustomerPlanPro        = "pro"
	CustomerPlanEnterprise = "enterprise"
)

// Customer status constants
const (
	CustomerStatusActive    = "active"
	CustomerStatusSuspended = "suspended"
	CustomerStatusCancelled = "cancelled"
)

// API key environment constants
const (
	APIKeyEnvironmentProduction  = "production"
	APIKeyEnvironmentStaging     = "staging"
	APIKeyEnvironmentDevelopment = "development"
)

// Usage operation constants
const (
	UsageOperationUpdateScore   = "update_score"
	UsageOperationGetRank       = "get_rank"
	UsageOperationGetScore      = "get_score"
	UsageOperationGetTopN       = "get_top_n"
	UsageOperationGetUserCount  = "get_user_count"
	UsageOperationRemoveUser    = "remove_user"
	UsageOperationGetUsersAround = "get_users_around"
	UsageOperationSyncDatabase  = "sync_database"
)

// Customer represents a multi-tenant customer in the leaderboard-as-a-service
type Customer struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	Name         string     `db:"name" json:"name"`
	Email        string     `db:"email" json:"email"`
	Plan         string     `db:"plan" json:"plan"`
	RateLimitRPM int        `db:"rate_limit_rpm" json:"rate_limit_rpm"`

	// Feature flags
	RedisEnabled     bool `db:"redis_enabled" json:"redis_enabled"`
	WebhooksEnabled  bool `db:"webhooks_enabled" json:"webhooks_enabled"`
	AnalyticsEnabled bool `db:"analytics_enabled" json:"analytics_enabled"`

	Status   string `db:"status" json:"status"`
	Metadata JSONB  `db:"metadata" json:"metadata,omitempty"`

	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// CustomerAPIKey represents an API key for customer authentication
type CustomerAPIKey struct {
	ID         uuid.UUID `db:"id" json:"id"`
	CustomerID uuid.UUID `db:"customer_id" json:"customer_id"`

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
	ID         uuid.UUID  `db:"id" json:"id"`
	CustomerID uuid.UUID  `db:"customer_id" json:"customer_id"`
	CampaignID *uuid.UUID `db:"campaign_id" json:"campaign_id,omitempty"`
	APIKeyID   *uuid.UUID `db:"api_key_id" json:"api_key_id,omitempty"`

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
	ID         uuid.UUID `db:"id" json:"id"`
	CustomerID uuid.UUID `db:"customer_id" json:"customer_id"`

	PeriodStart time.Time `db:"period_start" json:"period_start"`
	PeriodEnd   time.Time `db:"period_end" json:"period_end"`

	TotalOperations   int   `db:"total_operations" json:"total_operations"`
	OperationsByType  JSONB `db:"operations_by_type" json:"operations_by_type"`
	TotalCostCents    int   `db:"total_cost_cents" json:"total_cost_cents"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// RateLimit represents rate limit tracking for a customer
type RateLimit struct {
	ID         uuid.UUID `db:"id" json:"id"`
	CustomerID uuid.UUID `db:"customer_id" json:"customer_id"`

	WindowStart time.Time `db:"window_start" json:"window_start"`
	WindowEnd   time.Time `db:"window_end" json:"window_end"`

	RequestsCount int  `db:"requests_count" json:"requests_count"`
	RequestsLimit int  `db:"requests_limit" json:"requests_limit"`
	IsThrottled   bool `db:"is_throttled" json:"is_throttled"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// ============================================================================
// CUSTOMER OPERATIONS
// ============================================================================

// CreateCustomerParams represents parameters for creating a customer
type CreateCustomerParams struct {
	Name         string
	Email        string
	Plan         string
	RateLimitRPM int
	Metadata     JSONB
}

// CreateCustomer creates a new customer
func (s Store) CreateCustomer(ctx context.Context, params CreateCustomerParams) (Customer, error) {
	var customer Customer
	query := `
		INSERT INTO customers (name, email, plan, rate_limit_rpm, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, email, plan, rate_limit_rpm, redis_enabled, webhooks_enabled,
		          analytics_enabled, status, metadata, created_at, updated_at, deleted_at
	`

	err := s.db.GetContext(ctx, &customer, query,
		params.Name,
		params.Email,
		params.Plan,
		params.RateLimitRPM,
		params.Metadata,
	)

	if err != nil {
		return Customer{}, fmt.Errorf("failed to create customer: %w", err)
	}

	return customer, nil
}

// GetCustomerByID retrieves a customer by ID
func (s Store) GetCustomerByID(ctx context.Context, customerID uuid.UUID) (Customer, error) {
	var customer Customer
	query := `
		SELECT id, name, email, plan, rate_limit_rpm, redis_enabled, webhooks_enabled,
		       analytics_enabled, status, metadata, created_at, updated_at, deleted_at
		FROM customers
		WHERE id = $1 AND deleted_at IS NULL
	`

	err := s.db.GetContext(ctx, &customer, query, customerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return Customer{}, ErrNotFound
		}
		return Customer{}, fmt.Errorf("failed to get customer: %w", err)
	}

	return customer, nil
}

// GetCustomerByEmail retrieves a customer by email
func (s Store) GetCustomerByEmail(ctx context.Context, email string) (Customer, error) {
	var customer Customer
	query := `
		SELECT id, name, email, plan, rate_limit_rpm, redis_enabled, webhooks_enabled,
		       analytics_enabled, status, metadata, created_at, updated_at, deleted_at
		FROM customers
		WHERE email = $1 AND deleted_at IS NULL
	`

	err := s.db.GetContext(ctx, &customer, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return Customer{}, ErrNotFound
		}
		return Customer{}, fmt.Errorf("failed to get customer by email: %w", err)
	}

	return customer, nil
}

// UpdateCustomerParams represents parameters for updating a customer
type UpdateCustomerParams struct {
	Name         *string
	Plan         *string
	RateLimitRPM *int
	Status       *string
	Metadata     JSONB
}

// UpdateCustomer updates a customer
func (s Store) UpdateCustomer(ctx context.Context, customerID uuid.UUID, params UpdateCustomerParams) (Customer, error) {
	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}
	argPos := 1

	if params.Name != nil {
		updates = append(updates, fmt.Sprintf("name = $%d", argPos))
		args = append(args, *params.Name)
		argPos++
	}

	if params.Plan != nil {
		updates = append(updates, fmt.Sprintf("plan = $%d", argPos))
		args = append(args, *params.Plan)
		argPos++
	}

	if params.RateLimitRPM != nil {
		updates = append(updates, fmt.Sprintf("rate_limit_rpm = $%d", argPos))
		args = append(args, *params.RateLimitRPM)
		argPos++
	}

	if params.Status != nil {
		updates = append(updates, fmt.Sprintf("status = $%d", argPos))
		args = append(args, *params.Status)
		argPos++
	}

	if params.Metadata != nil {
		updates = append(updates, fmt.Sprintf("metadata = $%d", argPos))
		args = append(args, params.Metadata)
		argPos++
	}

	// Always update updated_at
	updates = append(updates, fmt.Sprintf("updated_at = $%d", argPos))
	args = append(args, time.Now())
	argPos++

	// Add customer ID as final parameter
	args = append(args, customerID)

	if len(updates) == 1 { // Only updated_at was set
		return s.GetCustomerByID(ctx, customerID)
	}

	query := fmt.Sprintf(`
		UPDATE customers
		SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, name, email, plan, rate_limit_rpm, redis_enabled, webhooks_enabled,
		          analytics_enabled, status, metadata, created_at, updated_at, deleted_at
	`, strings.Join(updates, ", "), argPos)

	var customer Customer
	err := s.db.GetContext(ctx, &customer, query, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return Customer{}, ErrNotFound
		}
		return Customer{}, fmt.Errorf("failed to update customer: %w", err)
	}

	return customer, nil
}

// DeleteCustomer soft deletes a customer
func (s Store) DeleteCustomer(ctx context.Context, customerID uuid.UUID) error {
	query := `
		UPDATE customers
		SET deleted_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := s.db.ExecContext(ctx, query, customerID)
	if err != nil {
		return fmt.Errorf("failed to delete customer: %w", err)
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
// CUSTOMER API KEY OPERATIONS
// ============================================================================

// CreateAPIKeyParams represents parameters for creating an API key
type CreateAPIKeyParams struct {
	CustomerID  uuid.UUID
	KeyHash     string
	KeyPrefix   string
	Name        string
	Environment string
	Scopes      JSONB
	ExpiresAt   *time.Time
}

// CreateAPIKey creates a new API key for a customer
func (s Store) CreateAPIKey(ctx context.Context, params CreateAPIKeyParams) (CustomerAPIKey, error) {
	var apiKey CustomerAPIKey
	query := `
		INSERT INTO customer_api_keys (customer_id, key_hash, key_prefix, name, environment, scopes, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, customer_id, key_hash, key_prefix, name, environment, scopes,
		          last_used_at, usage_count, is_active, expires_at, created_at, updated_at, deleted_at
	`

	err := s.db.GetContext(ctx, &apiKey, query,
		params.CustomerID,
		params.KeyHash,
		params.KeyPrefix,
		params.Name,
		params.Environment,
		params.Scopes,
		params.ExpiresAt,
	)

	if err != nil {
		return CustomerAPIKey{}, fmt.Errorf("failed to create API key: %w", err)
	}

	return apiKey, nil
}

// GetAPIKeyByHash retrieves an API key by its hash
func (s Store) GetAPIKeyByHash(ctx context.Context, keyHash string) (CustomerAPIKey, error) {
	var apiKey CustomerAPIKey
	query := `
		SELECT id, customer_id, key_hash, key_prefix, name, environment, scopes,
		       last_used_at, usage_count, is_active, expires_at, created_at, updated_at, deleted_at
		FROM customer_api_keys
		WHERE key_hash = $1 AND deleted_at IS NULL AND is_active = true
	`

	err := s.db.GetContext(ctx, &apiKey, query, keyHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return CustomerAPIKey{}, ErrNotFound
		}
		return CustomerAPIKey{}, fmt.Errorf("failed to get API key: %w", err)
	}

	// Check if key is expired
	if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
		return CustomerAPIKey{}, ErrNotFound
	}

	return apiKey, nil
}

// ListAPIKeysByCustomer retrieves all API keys for a customer
func (s Store) ListAPIKeysByCustomer(ctx context.Context, customerID uuid.UUID) ([]CustomerAPIKey, error) {
	var apiKeys []CustomerAPIKey
	query := `
		SELECT id, customer_id, key_hash, key_prefix, name, environment, scopes,
		       last_used_at, usage_count, is_active, expires_at, created_at, updated_at, deleted_at
		FROM customer_api_keys
		WHERE customer_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	err := s.db.SelectContext(ctx, &apiKeys, query, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	return apiKeys, nil
}

// UpdateAPIKeyUsage updates the last used timestamp and usage count
func (s Store) UpdateAPIKeyUsage(ctx context.Context, apiKeyID uuid.UUID) error {
	query := `
		UPDATE customer_api_keys
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
func (s Store) RevokeAPIKey(ctx context.Context, customerID, apiKeyID uuid.UUID) error {
	query := `
		UPDATE customer_api_keys
		SET is_active = false,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND customer_id = $2 AND deleted_at IS NULL
	`

	result, err := s.db.ExecContext(ctx, query, apiKeyID, customerID)
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
	CustomerID     uuid.UUID
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
			customer_id, campaign_id, api_key_id, operation, count,
			request_id, ip_address, user_agent, response_time_ms, status_code,
			billing_date
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, CURRENT_DATE)
	`

	_, err := s.db.ExecContext(ctx, query,
		params.CustomerID,
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

// GetUsageByCustomerAndPeriod retrieves aggregated usage for a customer in a period
func (s Store) GetUsageByCustomerAndPeriod(ctx context.Context, customerID uuid.UUID, startDate, endDate time.Time) (map[string]int, error) {
	query := `
		SELECT operation, SUM(count) as total
		FROM usage_events
		WHERE customer_id = $1
		  AND billing_date >= $2
		  AND billing_date <= $3
		GROUP BY operation
	`

	type OperationCount struct {
		Operation string `db:"operation"`
		Total     int    `db:"total"`
	}

	var results []OperationCount
	err := s.db.SelectContext(ctx, &results, query, customerID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage: %w", err)
	}

	usage := make(map[string]int)
	for _, result := range results {
		usage[result.Operation] = result.Total
	}

	return usage, nil
}
