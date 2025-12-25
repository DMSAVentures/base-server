-- ============================================================================
-- Campaign Settings Migration: JSONB to Tables
-- Replaces form_config, referral_config, email_config, branding_config
-- with proper relational tables
-- ============================================================================

-- ============================================================================
-- ENUMS
-- ============================================================================

-- Form field types
CREATE TYPE form_field_type AS ENUM (
    'email', 'text', 'textarea', 'select', 'checkbox',
    'radio', 'phone', 'url', 'date', 'number'
);

-- Captcha providers
CREATE TYPE captcha_provider AS ENUM ('turnstile', 'recaptcha', 'hcaptcha');

-- Sharing channels for referrals
CREATE TYPE sharing_channel AS ENUM ('email', 'twitter', 'facebook', 'linkedin', 'whatsapp');

-- Tracking integration types
CREATE TYPE tracking_integration_type AS ENUM (
    'google_analytics', 'meta_pixel', 'google_ads', 'tiktok_pixel', 'linkedin_insight'
);

-- ============================================================================
-- 1:1 SETTINGS TABLES
-- ============================================================================

-- Email Settings (replaces email_config JSONB)
CREATE TABLE campaign_email_settings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL UNIQUE REFERENCES campaigns(id) ON DELETE CASCADE,
    from_name VARCHAR(255),
    from_email VARCHAR(255),
    reply_to VARCHAR(255),
    verification_required BOOLEAN NOT NULL DEFAULT TRUE,
    send_welcome_email BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_campaign_email_settings_campaign ON campaign_email_settings(campaign_id);

-- Branding Settings (replaces branding_config JSONB)
CREATE TABLE campaign_branding_settings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL UNIQUE REFERENCES campaigns(id) ON DELETE CASCADE,
    logo_url VARCHAR(500),
    primary_color VARCHAR(7),
    font_family VARCHAR(100),
    custom_domain VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_campaign_branding_settings_campaign ON campaign_branding_settings(campaign_id);

-- Form Settings (replaces form_config JSONB scalar fields)
CREATE TABLE campaign_form_settings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL UNIQUE REFERENCES campaigns(id) ON DELETE CASCADE,
    captcha_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    captcha_provider captcha_provider,
    captcha_site_key VARCHAR(255),
    double_opt_in BOOLEAN NOT NULL DEFAULT TRUE,
    design JSONB NOT NULL DEFAULT '{}',  -- Stores layout, colors, typography, spacing, borderRadius, submitButtonText
    success_title VARCHAR(255),
    success_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_campaign_form_settings_campaign ON campaign_form_settings(campaign_id);

-- Referral Settings (replaces referral_config JSONB scalar fields)
CREATE TABLE campaign_referral_settings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL UNIQUE REFERENCES campaigns(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    points_per_referral INTEGER NOT NULL DEFAULT 1,
    verified_only BOOLEAN NOT NULL DEFAULT TRUE,
    positions_to_jump INTEGER NOT NULL DEFAULT 5,
    referrer_positions_to_jump INTEGER NOT NULL DEFAULT 3,
    sharing_channels sharing_channel[] NOT NULL DEFAULT '{email}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_campaign_referral_settings_campaign ON campaign_referral_settings(campaign_id);

-- ============================================================================
-- 1:N ENTITY TABLES
-- ============================================================================

-- Form Fields (replaces form_config.fields array)
CREATE TABLE campaign_form_fields (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    field_type form_field_type NOT NULL,
    label VARCHAR(255) NOT NULL,
    placeholder VARCHAR(255),
    required BOOLEAN NOT NULL DEFAULT FALSE,
    validation_pattern VARCHAR(500),
    options TEXT[],
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(campaign_id, name)
);

CREATE INDEX idx_campaign_form_fields_campaign ON campaign_form_fields(campaign_id);
CREATE INDEX idx_campaign_form_fields_order ON campaign_form_fields(campaign_id, display_order);

-- Share Messages (replaces referral_config.custom_share_messages object)
CREATE TABLE campaign_share_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    channel sharing_channel NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(campaign_id, channel)
);

CREATE INDEX idx_campaign_share_messages_campaign ON campaign_share_messages(campaign_id);

-- Tracking Integrations (new - implements tracking_config from frontend types)
CREATE TABLE campaign_tracking_integrations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    integration_type tracking_integration_type NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    tracking_id VARCHAR(100) NOT NULL,
    tracking_label VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(campaign_id, integration_type)
);

CREATE INDEX idx_campaign_tracking_integrations_campaign ON campaign_tracking_integrations(campaign_id);
CREATE INDEX idx_campaign_tracking_integrations_type ON campaign_tracking_integrations(integration_type);

-- ============================================================================
-- DROP JSONB COLUMNS FROM CAMPAIGNS TABLE
-- ============================================================================

ALTER TABLE campaigns DROP COLUMN form_config;
ALTER TABLE campaigns DROP COLUMN referral_config;
ALTER TABLE campaigns DROP COLUMN email_config;
ALTER TABLE campaigns DROP COLUMN branding_config;
