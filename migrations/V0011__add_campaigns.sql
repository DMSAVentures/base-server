-- Create ENUMs
CREATE TYPE campaign_status AS ENUM ('draft', 'active', 'paused', 'completed');
CREATE TYPE campaign_type AS ENUM ('waitlist', 'referral', 'contest');

-- Campaigns Table
CREATE TABLE campaigns (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    description TEXT,
    status campaign_status NOT NULL DEFAULT 'draft',

    -- Campaign type
    type campaign_type NOT NULL DEFAULT 'waitlist',

    -- Launch settings
    launch_date TIMESTAMPTZ,
    end_date TIMESTAMPTZ,

    -- Form settings
    form_config JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "fields": [{"name": "email", "type": "email", "required": true, "label": "Email"}],
    --   "captcha_enabled": true,
    --   "double_opt_in": true,
    --   "custom_css": ""
    -- }

    -- Referral settings
    referral_config JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "enabled": true,
    --   "points_per_referral": 1,
    --   "verified_only": true,
    --   "sharing_channels": ["email", "twitter", "facebook", "linkedin", "whatsapp"],
    --   "custom_share_messages": {}
    -- }

    -- Email settings
    email_config JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "from_name": "Company Name",
    --   "from_email": "hello@example.com",
    --   "reply_to": "support@example.com",
    --   "verification_required": true
    -- }

    -- Branding
    branding_config JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "logo_url": "",
    --   "primary_color": "#2563EB",
    --   "font_family": "Inter",
    --   "custom_domain": ""
    -- }

    -- Privacy & legal
    privacy_policy_url VARCHAR(500),
    terms_url VARCHAR(500),

    -- Limits
    max_signups INTEGER,

    -- Stats (denormalized for performance)
    total_signups INTEGER NOT NULL DEFAULT 0,
    total_verified INTEGER NOT NULL DEFAULT 0,
    total_referrals INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,

    UNIQUE(account_id, slug)
);

CREATE INDEX idx_campaigns_account ON campaigns(account_id);
CREATE INDEX idx_campaigns_slug ON campaigns(account_id, slug);
CREATE INDEX idx_campaigns_status ON campaigns(status);
CREATE INDEX idx_campaigns_launch_date ON campaigns(launch_date);
