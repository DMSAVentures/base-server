-- Integrations System: Zapier, Slack, and future integrations
-- Provides shared infrastructure for OAuth, subscriptions, and delivery logging

-- Enum for integration types
CREATE TYPE integration_type AS ENUM ('zapier', 'slack', 'discord', 'custom');

-- OAuth tokens (shared across integrations)
CREATE TABLE integration_oauth_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    integration_type integration_type NOT NULL,
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    refresh_token_hash VARCHAR(255),
    scopes TEXT[] NOT NULL DEFAULT '{}',
    metadata JSONB,                          -- Integration-specific data (workspace, channel, etc.)
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Event subscriptions (shared across integrations)
CREATE TABLE integration_subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    token_id UUID NOT NULL REFERENCES integration_oauth_tokens(id) ON DELETE CASCADE,
    integration_type integration_type NOT NULL,

    -- Subscription target
    target_url VARCHAR(500) NOT NULL,         -- Webhook URL (Zapier) or channel ID (Slack)
    event_type VARCHAR(100) NOT NULL,
    campaign_id UUID REFERENCES campaigns(id) ON DELETE CASCADE,

    -- Integration-specific config
    config JSONB,                             -- e.g., Slack: { channel: "#alerts", format: "detailed" }

    -- Status
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    last_triggered_at TIMESTAMPTZ,
    trigger_count INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

-- Delivery log (shared)
CREATE TABLE integration_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    subscription_id UUID NOT NULL REFERENCES integration_subscriptions(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL,              -- 'success', 'failed', 'pending'
    response_status INTEGER,
    duration_ms INTEGER,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for OAuth tokens
CREATE INDEX idx_int_oauth_account ON integration_oauth_tokens(account_id, integration_type);
CREATE INDEX idx_int_oauth_hash ON integration_oauth_tokens(token_hash);
CREATE INDEX idx_int_oauth_account_type ON integration_oauth_tokens(account_id, integration_type) WHERE revoked_at IS NULL;

-- Indexes for subscriptions
CREATE INDEX idx_int_subs_account ON integration_subscriptions(account_id, integration_type) WHERE deleted_at IS NULL;
CREATE INDEX idx_int_subs_event ON integration_subscriptions(event_type, integration_type) WHERE deleted_at IS NULL AND status = 'active';
CREATE INDEX idx_int_subs_account_id ON integration_subscriptions(account_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_int_subs_token ON integration_subscriptions(token_id) WHERE deleted_at IS NULL;

-- Indexes for deliveries
CREATE INDEX idx_int_deliveries_sub ON integration_deliveries(subscription_id, created_at DESC);
CREATE INDEX idx_int_deliveries_created ON integration_deliveries(created_at DESC);
