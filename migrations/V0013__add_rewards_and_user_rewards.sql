-- Create ENUMs
CREATE TYPE reward_type AS ENUM ('early_access', 'discount', 'premium_feature', 'merchandise', 'custom');
CREATE TYPE reward_trigger_type AS ENUM ('referral_count', 'position', 'milestone', 'manual');
CREATE TYPE reward_delivery_method AS ENUM ('email', 'webhook', 'manual');
CREATE TYPE reward_status AS ENUM ('active', 'paused', 'expired');
CREATE TYPE user_reward_status AS ENUM ('pending', 'earned', 'delivered', 'redeemed', 'revoked', 'expired');

-- Rewards Table
CREATE TABLE rewards (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    description TEXT,
    type reward_type NOT NULL,

    -- Reward configuration
    config JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "value": "20% off",
    --   "code_template": "EARLY{random}",
    --   "instructions": "Use this code at checkout",
    --   "expiry_days": 30
    -- }

    -- Trigger configuration
    trigger_type reward_trigger_type NOT NULL,
    trigger_config JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "referral_count": 5,
    --   "verified_only": true
    -- }

    -- Delivery configuration
    delivery_method reward_delivery_method NOT NULL,
    delivery_config JSONB NOT NULL DEFAULT '{}',

    -- Limits
    total_available INTEGER,
    total_claimed INTEGER NOT NULL DEFAULT 0,
    user_limit INTEGER DEFAULT 1, -- how many times one user can earn this

    status reward_status NOT NULL DEFAULT 'active',

    -- Scheduling
    starts_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_rewards_campaign ON rewards(campaign_id);
CREATE INDEX idx_rewards_status ON rewards(status);
CREATE INDEX idx_rewards_type ON rewards(type);

-- User Rewards Table
CREATE TABLE user_rewards (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES waitlist_users(id) ON DELETE CASCADE,
    reward_id UUID NOT NULL REFERENCES rewards(id) ON DELETE CASCADE,
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,

    status user_reward_status NOT NULL DEFAULT 'pending',

    -- Reward details (snapshot at time of earning)
    reward_data JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "code": "EARLY2024ABC",
    --   "value": "20% off",
    --   "instructions": "..."
    -- }

    -- Lifecycle timestamps
    earned_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMPTZ,
    redeemed_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,

    -- Delivery tracking
    delivery_attempts INTEGER NOT NULL DEFAULT 0,
    last_delivery_attempt_at TIMESTAMPTZ,
    delivery_error TEXT,

    -- Revocation
    revoked_reason TEXT,
    revoked_by UUID REFERENCES users(id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_rewards_user ON user_rewards(user_id);
CREATE INDEX idx_user_rewards_reward ON user_rewards(reward_id);
CREATE INDEX idx_user_rewards_campaign ON user_rewards(campaign_id);
CREATE INDEX idx_user_rewards_status ON user_rewards(status);
CREATE INDEX idx_user_rewards_earned ON user_rewards(earned_at DESC);
