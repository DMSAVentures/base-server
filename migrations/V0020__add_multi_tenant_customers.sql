-- V0020: Extend accounts for Leaderboard-as-a-Service features
-- This migration adds API keys, usage tracking, and rate limiting for multi-tenant leaderboard service

-- ============================================================================
-- EXTEND ACCOUNTS TABLE
-- ============================================================================
-- Add rate limiting and feature flags to existing accounts table
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS rate_limit_rpm INTEGER NOT NULL DEFAULT 60;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS redis_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS webhooks_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS analytics_enabled BOOLEAN NOT NULL DEFAULT false;

-- Add comments
COMMENT ON COLUMN accounts.rate_limit_rpm IS 'Rate limit in requests per minute for API access';
COMMENT ON COLUMN accounts.redis_enabled IS 'Whether account has access to Redis-backed leaderboards';

-- ============================================================================
-- ACCOUNT API KEYS TABLE
-- ============================================================================
-- API keys for account authentication on customer-facing API
-- Supports multiple keys per account for rotation and different environments
CREATE TABLE IF NOT EXISTS account_api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    -- API key (hashed)
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    key_prefix VARCHAR(20) NOT NULL, -- First 8 chars for identification (e.g., "lb_live_12345678")

    -- Key metadata
    name VARCHAR(255) NOT NULL, -- User-friendly name (e.g., "Production", "Staging")
    environment VARCHAR(50) NOT NULL DEFAULT 'production', -- production, staging, development

    -- Permissions
    scopes JSONB DEFAULT '["read", "write"]'::jsonb,

    -- Usage tracking
    last_used_at TIMESTAMP,
    usage_count INTEGER DEFAULT 0,

    -- Status
    is_active BOOLEAN NOT NULL DEFAULT true,
    expires_at TIMESTAMP,

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP
);

-- Indexes for API keys
CREATE INDEX idx_account_api_keys_account_id ON account_api_keys(account_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_account_api_keys_key_hash ON account_api_keys(key_hash) WHERE deleted_at IS NULL AND is_active = true;
CREATE INDEX idx_account_api_keys_key_prefix ON account_api_keys(key_prefix);

-- ============================================================================
-- USAGE EVENTS TABLE
-- ============================================================================
-- Track API operations for billing and analytics
CREATE TABLE IF NOT EXISTS usage_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    campaign_id UUID REFERENCES campaigns(id) ON DELETE SET NULL,
    api_key_id UUID REFERENCES account_api_keys(id) ON DELETE SET NULL,

    -- Operation details
    operation VARCHAR(100) NOT NULL, -- update_score, get_rank, get_top_n, etc.
    count INTEGER NOT NULL DEFAULT 1,

    -- Request metadata
    request_id UUID,
    ip_address INET,
    user_agent TEXT,

    -- Performance metrics
    response_time_ms INTEGER,
    status_code INTEGER,

    -- Billing period
    billing_date DATE NOT NULL DEFAULT CURRENT_DATE,

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- Indexes for usage events (optimized for billing queries)
CREATE INDEX idx_usage_events_account_billing ON usage_events(account_id, billing_date, operation);
CREATE INDEX idx_usage_events_campaign ON usage_events(campaign_id, created_at);
CREATE INDEX idx_usage_events_api_key ON usage_events(api_key_id, created_at);
CREATE INDEX idx_usage_events_created_at ON usage_events(created_at);

-- ============================================================================
-- USAGE AGGREGATES TABLE (for faster billing queries)
-- ============================================================================
-- Pre-aggregated usage data for billing dashboards
CREATE TABLE IF NOT EXISTS usage_aggregates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    -- Aggregation period
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,

    -- Aggregated metrics
    total_operations INTEGER NOT NULL DEFAULT 0,
    operations_by_type JSONB DEFAULT '{}'::jsonb, -- {"update_score": 1000, "get_rank": 500}

    -- Cost calculation
    total_cost_cents INTEGER DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,

    UNIQUE(account_id, period_start, period_end)
);

-- Indexes for usage aggregates
CREATE INDEX idx_usage_aggregates_account_period ON usage_aggregates(account_id, period_start, period_end);
CREATE INDEX idx_usage_aggregates_period_start ON usage_aggregates(period_start);

-- ============================================================================
-- RATE LIMIT TRACKING TABLE
-- ============================================================================
-- Track rate limit consumption (Redis-backed, but PostgreSQL fallback)
CREATE TABLE IF NOT EXISTS rate_limits (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    -- Window tracking
    window_start TIMESTAMP NOT NULL,
    window_end TIMESTAMP NOT NULL,

    -- Consumption
    requests_count INTEGER NOT NULL DEFAULT 0,
    requests_limit INTEGER NOT NULL,

    -- Status
    is_throttled BOOLEAN NOT NULL DEFAULT false,

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,

    UNIQUE(account_id, window_start)
);

-- Indexes for rate limits
CREATE INDEX idx_rate_limits_account_window ON rate_limits(account_id, window_start, window_end);
CREATE INDEX idx_rate_limits_window_end ON rate_limits(window_end);

-- ============================================================================
-- UPDATE EXISTING ACCOUNTS
-- ============================================================================
-- Update existing accounts to have Redis enabled for backward compatibility
UPDATE accounts
SET redis_enabled = true,
    rate_limit_rpm = 10000
WHERE plan IN ('pro', 'enterprise')
  AND deleted_at IS NULL;

-- ============================================================================
-- COMMENTS
-- ============================================================================
COMMENT ON TABLE account_api_keys IS 'API keys for account authentication and authorization';
COMMENT ON TABLE usage_events IS 'Detailed usage events for billing and analytics';
COMMENT ON TABLE usage_aggregates IS 'Pre-aggregated usage data for billing dashboards';
COMMENT ON TABLE rate_limits IS 'Rate limit tracking for API throttling';

COMMENT ON COLUMN account_api_keys.key_hash IS 'Bcrypt hashed API key for security';
COMMENT ON COLUMN account_api_keys.key_prefix IS 'First 8 characters for key identification';
COMMENT ON COLUMN usage_events.operation IS 'API operation: update_score, get_rank, get_top_n, etc.';
COMMENT ON COLUMN usage_events.billing_date IS 'Date for billing aggregation';
