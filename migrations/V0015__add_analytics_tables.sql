-- Campaign Analytics Table
-- Note: Can be migrated to TimescaleDB hypertable later for time-series optimization
CREATE TABLE campaign_analytics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    time TIMESTAMPTZ NOT NULL,
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,

    -- Metrics
    new_signups INTEGER NOT NULL DEFAULT 0,
    new_verified INTEGER NOT NULL DEFAULT 0,
    new_referrals INTEGER NOT NULL DEFAULT 0,
    new_conversions INTEGER NOT NULL DEFAULT 0,

    emails_sent INTEGER NOT NULL DEFAULT 0,
    emails_opened INTEGER NOT NULL DEFAULT 0,
    emails_clicked INTEGER NOT NULL DEFAULT 0,

    rewards_earned INTEGER NOT NULL DEFAULT 0,
    rewards_delivered INTEGER NOT NULL DEFAULT 0,

    -- Aggregated for quick queries
    total_signups INTEGER NOT NULL DEFAULT 0,
    total_verified INTEGER NOT NULL DEFAULT 0,
    total_referrals INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(time, campaign_id)
);

CREATE INDEX idx_campaign_analytics_campaign ON campaign_analytics(campaign_id, time DESC);
CREATE INDEX idx_campaign_analytics_time ON campaign_analytics(time DESC);

-- User Activity Logs Table
CREATE TABLE user_activity_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id UUID REFERENCES waitlist_users(id) ON DELETE SET NULL,

    event_type VARCHAR(100) NOT NULL, -- signup, verify, share, referral, reward, click, etc.
    event_data JSONB DEFAULT '{}',

    -- Context
    ip_address INET,
    user_agent TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_activity_logs_campaign ON user_activity_logs(campaign_id, created_at DESC);
CREATE INDEX idx_user_activity_logs_user ON user_activity_logs(user_id, created_at DESC);
CREATE INDEX idx_user_activity_logs_event ON user_activity_logs(event_type, created_at DESC);
