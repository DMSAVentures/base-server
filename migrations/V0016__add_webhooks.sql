-- Create ENUMs
CREATE TYPE webhook_status AS ENUM ('active', 'paused', 'failed');
CREATE TYPE webhook_delivery_status AS ENUM ('pending', 'success', 'failed');

-- Webhooks Table
CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    campaign_id UUID REFERENCES campaigns(id) ON DELETE CASCADE, -- NULL = account-level

    url VARCHAR(500) NOT NULL,
    secret VARCHAR(255) NOT NULL, -- For HMAC signature

    -- Event subscriptions
    events TEXT[] NOT NULL, -- ['user.created', 'user.verified', 'referral.created', etc.]

    status webhook_status NOT NULL DEFAULT 'active',

    -- Delivery settings
    retry_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    max_retries INTEGER NOT NULL DEFAULT 5,

    -- Stats
    total_sent INTEGER NOT NULL DEFAULT 0,
    total_failed INTEGER NOT NULL DEFAULT 0,
    last_success_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_webhooks_account ON webhooks(account_id);
CREATE INDEX idx_webhooks_campaign ON webhooks(campaign_id);
CREATE INDEX idx_webhooks_status ON webhooks(status);

-- Webhook Deliveries Table
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,

    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,

    status webhook_delivery_status NOT NULL DEFAULT 'pending',

    -- Request/Response
    request_headers JSONB,
    response_status INTEGER,
    response_body TEXT,
    response_headers JSONB,

    -- Timing
    duration_ms INTEGER,

    -- Retry tracking
    attempt_number INTEGER NOT NULL DEFAULT 1,
    next_retry_at TIMESTAMPTZ,

    error_message TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMPTZ
);

CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id, created_at DESC);
CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status);
CREATE INDEX idx_webhook_deliveries_retry ON webhook_deliveries(status, next_retry_at) WHERE status = 'failed';
