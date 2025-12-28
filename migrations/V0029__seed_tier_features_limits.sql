-- Migration: Create tier_features and tier_limits tables
-- These tables map the account_plan enum directly to features and limits

-- Tier Features Table - maps plan names to feature availability
CREATE TABLE tier_features (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    plan_name VARCHAR(50) NOT NULL,
    feature_name VARCHAR(100) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(plan_name, feature_name)
);

CREATE INDEX idx_tier_features_plan ON tier_features(plan_name);

-- Tier Limits Table - maps plan names to limit values
CREATE TABLE tier_limits (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    plan_name VARCHAR(50) NOT NULL,
    limit_name VARCHAR(100) NOT NULL,
    limit_value INTEGER, -- NULL means unlimited
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(plan_name, limit_name)
);

CREATE INDEX idx_tier_limits_plan ON tier_limits(plan_name);

-- Seed Features for all plans
-- Feature names: email_verification, referral_system, visual_form_builder, visual_email_builder,
--                all_widget_types, remove_branding, anti_spam_protection, enhanced_lead_data,
--                tracking_pixels, webhooks_zapier, email_blasts, json_export

-- FREE plan features
INSERT INTO tier_features (plan_name, feature_name, enabled) VALUES
    ('free', 'email_verification', false),
    ('free', 'referral_system', false),
    ('free', 'visual_form_builder', true),
    ('free', 'visual_email_builder', false),
    ('free', 'all_widget_types', false),    -- Full only
    ('free', 'remove_branding', false),
    ('free', 'anti_spam_protection', false),
    ('free', 'enhanced_lead_data', false),
    ('free', 'tracking_pixels', false),
    ('free', 'webhooks_zapier', false),
    ('free', 'email_blasts', false),
    ('free', 'json_export', false);

-- STARTER plan features (same as free)
INSERT INTO tier_features (plan_name, feature_name, enabled) VALUES
    ('starter', 'email_verification', false),
    ('starter', 'referral_system', false),
    ('starter', 'visual_form_builder', true),
    ('starter', 'visual_email_builder', false),
    ('starter', 'all_widget_types', false), -- Full only
    ('starter', 'remove_branding', false),
    ('starter', 'anti_spam_protection', false),
    ('starter', 'enhanced_lead_data', false),
    ('starter', 'tracking_pixels', false),
    ('starter', 'webhooks_zapier', false),
    ('starter', 'email_blasts', false),
    ('starter', 'json_export', false);

-- PRO plan features
INSERT INTO tier_features (plan_name, feature_name, enabled) VALUES
    ('pro', 'email_verification', true),
    ('pro', 'referral_system', true),
    ('pro', 'visual_form_builder', true),
    ('pro', 'visual_email_builder', true),
    ('pro', 'all_widget_types', true),      -- All widget types
    ('pro', 'remove_branding', true),
    ('pro', 'anti_spam_protection', true),
    ('pro', 'enhanced_lead_data', true),
    ('pro', 'tracking_pixels', false),
    ('pro', 'webhooks_zapier', false),
    ('pro', 'email_blasts', false),
    ('pro', 'json_export', true);

-- ENTERPRISE plan features (maps to Team tier)
INSERT INTO tier_features (plan_name, feature_name, enabled) VALUES
    ('enterprise', 'email_verification', true),
    ('enterprise', 'referral_system', true),
    ('enterprise', 'visual_form_builder', true),
    ('enterprise', 'visual_email_builder', true),
    ('enterprise', 'all_widget_types', true), -- All widget types
    ('enterprise', 'remove_branding', true),
    ('enterprise', 'anti_spam_protection', true),
    ('enterprise', 'enhanced_lead_data', true),
    ('enterprise', 'tracking_pixels', true),
    ('enterprise', 'webhooks_zapier', true),
    ('enterprise', 'email_blasts', true),
    ('enterprise', 'json_export', true);

-- Seed Limits for all plans
-- Limit names: campaigns, leads, team_members
-- NULL value means unlimited

-- FREE plan limits
INSERT INTO tier_limits (plan_name, limit_name, limit_value) VALUES
    ('free', 'campaigns', 1),
    ('free', 'leads', 200),
    ('free', 'team_members', 1);

-- STARTER plan limits (same as free)
INSERT INTO tier_limits (plan_name, limit_name, limit_value) VALUES
    ('starter', 'campaigns', 1),
    ('starter', 'leads', 200),
    ('starter', 'team_members', 1);

-- PRO plan limits
INSERT INTO tier_limits (plan_name, limit_name, limit_value) VALUES
    ('pro', 'campaigns', NULL), -- unlimited
    ('pro', 'leads', 5000),
    ('pro', 'team_members', 1);

-- ENTERPRISE plan limits (maps to Team tier)
INSERT INTO tier_limits (plan_name, limit_name, limit_value) VALUES
    ('enterprise', 'campaigns', NULL), -- unlimited
    ('enterprise', 'leads', 100000),
    ('enterprise', 'team_members', 5);
