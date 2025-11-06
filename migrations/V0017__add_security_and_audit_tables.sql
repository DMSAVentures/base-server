-- Create ENUMs
CREATE TYPE api_key_rate_limit_tier AS ENUM ('standard', 'pro', 'enterprise');
CREATE TYPE api_key_status AS ENUM ('active', 'revoked');
CREATE TYPE actor_type AS ENUM ('user', 'api_key', 'system');
CREATE TYPE fraud_detection_type AS ENUM ('self_referral', 'fake_email', 'bot', 'suspicious_ip', 'velocity');
CREATE TYPE fraud_detection_status AS ENUM ('pending', 'confirmed', 'false_positive', 'resolved');

-- API Keys Table
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL UNIQUE, -- SHA-256 hash of the key
    key_prefix VARCHAR(20) NOT NULL, -- First 8 chars for identification (e.g., "sk_live_")

    -- Permissions
    scopes TEXT[] NOT NULL DEFAULT '{}', -- ['campaigns:read', 'users:write', etc.]

    -- Rate limiting
    rate_limit_tier api_key_rate_limit_tier NOT NULL DEFAULT 'standard',

    status api_key_status NOT NULL DEFAULT 'active',

    -- Usage tracking
    last_used_at TIMESTAMPTZ,
    total_requests INTEGER NOT NULL DEFAULT 0,

    expires_at TIMESTAMPTZ,

    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMPTZ,
    revoked_by UUID REFERENCES users(id)
);

CREATE INDEX idx_api_keys_account ON api_keys(account_id);
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_status ON api_keys(status);

-- Audit Logs Table
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID REFERENCES accounts(id) ON DELETE CASCADE,

    -- Actor
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_type actor_type NOT NULL,
    actor_identifier VARCHAR(255), -- email, API key prefix, etc.

    -- Action
    action VARCHAR(100) NOT NULL, -- campaign.created, user.deleted, settings.updated, etc.
    resource_type VARCHAR(100) NOT NULL, -- campaign, user, reward, etc.
    resource_id UUID,

    -- Changes
    changes JSONB, -- {"field": {"old": "value", "new": "value"}}

    -- Context
    ip_address INET,
    user_agent TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_logs_account ON audit_logs(account_id, created_at DESC);
CREATE INDEX idx_audit_logs_actor ON audit_logs(actor_user_id, created_at DESC);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action, created_at DESC);

-- Fraud Detection Table
CREATE TABLE fraud_detections (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id UUID REFERENCES waitlist_users(id) ON DELETE CASCADE,

    detection_type fraud_detection_type NOT NULL,
    confidence_score DECIMAL(3, 2) NOT NULL, -- 0.00 to 1.00

    details JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "reason": "Same IP as referrer",
    --   "evidence": {...}
    -- }

    status fraud_detection_status NOT NULL DEFAULT 'pending',

    -- Review
    reviewed_by UUID REFERENCES users(id),
    reviewed_at TIMESTAMPTZ,
    review_notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_fraud_detections_campaign ON fraud_detections(campaign_id, created_at DESC);
CREATE INDEX idx_fraud_detections_user ON fraud_detections(user_id);
CREATE INDEX idx_fraud_detections_status ON fraud_detections(status);
CREATE INDEX idx_fraud_detections_confidence ON fraud_detections(confidence_score DESC);
