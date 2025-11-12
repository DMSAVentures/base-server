-- Migration: Refactor campaign JSONB configs to normalized tables
-- This migration extracts form_config, referral_config, email_config, and branding_config
-- from JSONB columns into dedicated tables with proper structure and indexes.

-- ============================================================================
-- STEP 1: Create new config tables
-- ============================================================================

-- Create form field type enum
CREATE TYPE form_field_type AS ENUM ('email', 'text', 'select', 'checkbox', 'textarea', 'number');

-- Create sharing channel enum
CREATE TYPE sharing_channel AS ENUM ('email', 'twitter', 'facebook', 'linkedin', 'whatsapp');

-- Campaign Form Configs
CREATE TABLE campaign_form_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL UNIQUE REFERENCES campaigns(id) ON DELETE CASCADE,

    -- Form settings
    captcha_enabled BOOLEAN NOT NULL DEFAULT false,
    double_opt_in BOOLEAN NOT NULL DEFAULT true,
    custom_css TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Campaign Form Fields (dynamic fields array)
CREATE TABLE campaign_form_fields (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    form_config_id UUID NOT NULL REFERENCES campaign_form_configs(id) ON DELETE CASCADE,

    -- Field definition
    name VARCHAR(255) NOT NULL,
    type form_field_type NOT NULL,
    label VARCHAR(255) NOT NULL,
    placeholder VARCHAR(255),
    required BOOLEAN NOT NULL DEFAULT false,

    -- For select fields
    options TEXT[],

    -- Validation rules (keeping as JSONB for flexibility)
    validation JSONB DEFAULT '{}',

    -- Display order
    display_order INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Campaign Referral Configs
CREATE TABLE campaign_referral_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL UNIQUE REFERENCES campaigns(id) ON DELETE CASCADE,

    -- Referral settings
    enabled BOOLEAN NOT NULL DEFAULT true,
    points_per_referral INTEGER NOT NULL DEFAULT 1,
    verified_only BOOLEAN NOT NULL DEFAULT true,

    -- Sharing channels
    sharing_channels sharing_channel[] DEFAULT ARRAY['email', 'twitter', 'facebook', 'linkedin', 'whatsapp']::sharing_channel[],

    -- Custom share messages (key-value pairs)
    custom_share_messages JSONB DEFAULT '{}',

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_points_positive CHECK (points_per_referral > 0)
);

-- Campaign Email Configs
CREATE TABLE campaign_email_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL UNIQUE REFERENCES campaigns(id) ON DELETE CASCADE,

    -- Email settings
    from_name VARCHAR(255),
    from_email VARCHAR(255),
    reply_to VARCHAR(255),
    verification_required BOOLEAN NOT NULL DEFAULT true,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Campaign Branding Configs
CREATE TABLE campaign_branding_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL UNIQUE REFERENCES campaigns(id) ON DELETE CASCADE,

    -- Branding settings
    logo_url VARCHAR(500),
    primary_color VARCHAR(7) DEFAULT '#2563EB',
    font_family VARCHAR(255) DEFAULT 'Inter',
    custom_domain VARCHAR(255),

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_color_format CHECK (primary_color ~ '^#[0-9A-Fa-f]{6}$')
);

-- ============================================================================
-- STEP 2: Create indexes for performance
-- ============================================================================

-- Form config indexes
CREATE INDEX idx_form_configs_campaign ON campaign_form_configs(campaign_id);
CREATE INDEX idx_form_configs_captcha ON campaign_form_configs(captcha_enabled) WHERE captcha_enabled = true;
CREATE INDEX idx_form_configs_double_opt_in ON campaign_form_configs(double_opt_in);

-- Form fields indexes
CREATE INDEX idx_form_fields_config ON campaign_form_fields(form_config_id);
CREATE INDEX idx_form_fields_order ON campaign_form_fields(form_config_id, display_order);
CREATE INDEX idx_form_fields_name ON campaign_form_fields(name);
CREATE INDEX idx_form_fields_required ON campaign_form_fields(required) WHERE required = true;

-- Referral config indexes
CREATE INDEX idx_referral_configs_campaign ON campaign_referral_configs(campaign_id);
CREATE INDEX idx_referral_configs_enabled ON campaign_referral_configs(enabled) WHERE enabled = true;
CREATE INDEX idx_referral_configs_points ON campaign_referral_configs(points_per_referral);
CREATE INDEX idx_referral_configs_verified_only ON campaign_referral_configs(verified_only);

-- Email config indexes
CREATE INDEX idx_email_configs_campaign ON campaign_email_configs(campaign_id);
CREATE INDEX idx_email_configs_verification ON campaign_email_configs(verification_required);
CREATE INDEX idx_email_configs_from_email ON campaign_email_configs(from_email);

-- Branding config indexes
CREATE INDEX idx_branding_configs_campaign ON campaign_branding_configs(campaign_id);
CREATE INDEX idx_branding_configs_custom_domain ON campaign_branding_configs(custom_domain) WHERE custom_domain IS NOT NULL;
CREATE INDEX idx_branding_configs_color ON campaign_branding_configs(primary_color);

-- ============================================================================
-- STEP 3: Migrate existing data from JSONB to normalized tables
-- ============================================================================

-- Migrate form_config data
INSERT INTO campaign_form_configs (campaign_id, captcha_enabled, double_opt_in, custom_css, created_at, updated_at)
SELECT
    id as campaign_id,
    COALESCE((form_config->>'captcha_enabled')::boolean, false) as captcha_enabled,
    COALESCE((form_config->>'double_opt_in')::boolean, true) as double_opt_in,
    form_config->>'custom_css' as custom_css,
    created_at,
    updated_at
FROM campaigns
WHERE deleted_at IS NULL;

-- Migrate form fields (if they exist in JSONB)
INSERT INTO campaign_form_fields (form_config_id, name, type, label, placeholder, required, options, validation, display_order, created_at, updated_at)
SELECT
    fc.id as form_config_id,
    field->>'name' as name,
    CAST(field->>'type' as form_field_type) as type,
    field->>'label' as label,
    field->>'placeholder' as placeholder,
    COALESCE((field->>'required')::boolean, false) as required,
    CASE
        WHEN field->'options' IS NOT NULL AND jsonb_typeof(field->'options') = 'array'
        THEN ARRAY(SELECT jsonb_array_elements_text(field->'options'))
        ELSE NULL
    END as options,
    COALESCE(field->'validation', '{}'::jsonb) as validation,
    COALESCE((field->>'display_order')::integer, row_number() OVER (PARTITION BY c.id ORDER BY ordinality)) as display_order,
    c.created_at,
    c.updated_at
FROM campaigns c
JOIN campaign_form_configs fc ON fc.campaign_id = c.id
CROSS JOIN LATERAL jsonb_array_elements(
    CASE
        WHEN jsonb_typeof(c.form_config->'fields') = 'array'
        THEN c.form_config->'fields'
        ELSE '[]'::jsonb
    END
) WITH ORDINALITY AS field
WHERE c.deleted_at IS NULL
  AND jsonb_typeof(c.form_config->'fields') = 'array';

-- Migrate referral_config data
INSERT INTO campaign_referral_configs (campaign_id, enabled, points_per_referral, verified_only, sharing_channels, custom_share_messages, created_at, updated_at)
SELECT
    id as campaign_id,
    COALESCE((referral_config->>'enabled')::boolean, true) as enabled,
    COALESCE((referral_config->>'points_per_referral')::integer, 1) as points_per_referral,
    COALESCE((referral_config->>'verified_only')::boolean, true) as verified_only,
    CASE
        WHEN referral_config->'sharing_channels' IS NOT NULL AND jsonb_typeof(referral_config->'sharing_channels') = 'array'
        THEN ARRAY(SELECT jsonb_array_elements_text(referral_config->'sharing_channels'))::sharing_channel[]
        ELSE ARRAY['email', 'twitter', 'facebook', 'linkedin', 'whatsapp']::sharing_channel[]
    END as sharing_channels,
    COALESCE(referral_config->'custom_share_messages', '{}'::jsonb) as custom_share_messages,
    created_at,
    updated_at
FROM campaigns
WHERE deleted_at IS NULL;

-- Migrate email_config data
INSERT INTO campaign_email_configs (campaign_id, from_name, from_email, reply_to, verification_required, created_at, updated_at)
SELECT
    id as campaign_id,
    email_config->>'from_name' as from_name,
    email_config->>'from_email' as from_email,
    email_config->>'reply_to' as reply_to,
    COALESCE((email_config->>'verification_required')::boolean, true) as verification_required,
    created_at,
    updated_at
FROM campaigns
WHERE deleted_at IS NULL;

-- Migrate branding_config data
INSERT INTO campaign_branding_configs (campaign_id, logo_url, primary_color, font_family, custom_domain, created_at, updated_at)
SELECT
    id as campaign_id,
    branding_config->>'logo_url' as logo_url,
    COALESCE(branding_config->>'primary_color', '#2563EB') as primary_color,
    COALESCE(branding_config->>'font_family', 'Inter') as font_family,
    branding_config->>'custom_domain' as custom_domain,
    created_at,
    updated_at
FROM campaigns
WHERE deleted_at IS NULL;

-- ============================================================================
-- STEP 4: Drop old JSONB columns from campaigns table
-- ============================================================================

-- Note: We're keeping these columns temporarily for rollback safety
-- Uncomment these after verifying the migration is successful:

-- ALTER TABLE campaigns DROP COLUMN form_config;
-- ALTER TABLE campaigns DROP COLUMN referral_config;
-- ALTER TABLE campaigns DROP COLUMN email_config;
-- ALTER TABLE campaigns DROP COLUMN branding_config;

-- For now, we'll just add a comment to mark them as deprecated
COMMENT ON COLUMN campaigns.form_config IS 'DEPRECATED: Use campaign_form_configs table instead. Will be removed in future migration.';
COMMENT ON COLUMN campaigns.referral_config IS 'DEPRECATED: Use campaign_referral_configs table instead. Will be removed in future migration.';
COMMENT ON COLUMN campaigns.email_config IS 'DEPRECATED: Use campaign_email_configs table instead. Will be removed in future migration.';
COMMENT ON COLUMN campaigns.branding_config IS 'DEPRECATED: Use campaign_branding_configs table instead. Will be removed in future migration.';
