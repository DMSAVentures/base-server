-- Create ENUMs
CREATE TYPE waitlist_user_status AS ENUM ('pending', 'verified', 'converted', 'removed', 'blocked');
CREATE TYPE user_source AS ENUM ('direct', 'referral', 'social', 'ad');
CREATE TYPE referral_status AS ENUM ('pending', 'verified', 'converted', 'invalid');
CREATE TYPE referral_source AS ENUM ('email', 'twitter', 'facebook', 'linkedin', 'whatsapp', 'direct');

-- Waitlist Users Table
CREATE TABLE waitlist_users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,

    -- Basic info
    email VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),

    -- Status
    status waitlist_user_status NOT NULL DEFAULT 'pending',

    -- Position tracking
    position INTEGER NOT NULL,
    original_position INTEGER NOT NULL,

    -- Referral tracking
    referral_code VARCHAR(50) UNIQUE NOT NULL,
    referred_by_id UUID REFERENCES waitlist_users(id) ON DELETE SET NULL,
    referral_count INTEGER NOT NULL DEFAULT 0,
    verified_referral_count INTEGER NOT NULL DEFAULT 0,

    -- Points & rewards
    points INTEGER NOT NULL DEFAULT 0,

    -- Verification
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    verification_token VARCHAR(255),
    verification_sent_at TIMESTAMPTZ,
    verified_at TIMESTAMPTZ,

    -- Source tracking
    source user_source,
    utm_source VARCHAR(255),
    utm_medium VARCHAR(255),
    utm_campaign VARCHAR(255),
    utm_term VARCHAR(255),
    utm_content VARCHAR(255),

    -- Device & location
    ip_address INET,
    user_agent TEXT,
    country_code VARCHAR(2),
    city VARCHAR(100),
    device_fingerprint VARCHAR(255),

    -- Custom fields (flexible)
    metadata JSONB DEFAULT '{}',

    -- Consent tracking
    marketing_consent BOOLEAN DEFAULT FALSE,
    marketing_consent_at TIMESTAMPTZ,
    terms_accepted BOOLEAN DEFAULT FALSE,
    terms_accepted_at TIMESTAMPTZ,

    -- Engagement
    last_activity_at TIMESTAMPTZ,
    share_count INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,

    UNIQUE(campaign_id, email)
);

CREATE INDEX idx_waitlist_users_campaign ON waitlist_users(campaign_id);
CREATE INDEX idx_waitlist_users_email ON waitlist_users(email);
CREATE INDEX idx_waitlist_users_position ON waitlist_users(campaign_id, position);
CREATE INDEX idx_waitlist_users_referral_code ON waitlist_users(referral_code);
CREATE INDEX idx_waitlist_users_referred_by ON waitlist_users(referred_by_id);
CREATE INDEX idx_waitlist_users_status ON waitlist_users(campaign_id, status);
CREATE INDEX idx_waitlist_users_verified ON waitlist_users(campaign_id, email_verified);
CREATE INDEX idx_waitlist_users_created ON waitlist_users(campaign_id, created_at DESC);

-- Referrals Table
CREATE TABLE referrals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    referrer_id UUID NOT NULL REFERENCES waitlist_users(id) ON DELETE CASCADE,
    referred_id UUID NOT NULL REFERENCES waitlist_users(id) ON DELETE CASCADE,

    status referral_status NOT NULL DEFAULT 'pending',

    -- Tracking
    source referral_source,
    ip_address INET,

    verified_at TIMESTAMPTZ,
    converted_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(referrer_id, referred_id)
);

CREATE INDEX idx_referrals_campaign ON referrals(campaign_id);
CREATE INDEX idx_referrals_referrer ON referrals(referrer_id);
CREATE INDEX idx_referrals_referred ON referrals(referred_id);
CREATE INDEX idx_referrals_status ON referrals(status);
CREATE INDEX idx_referrals_created ON referrals(created_at DESC);
