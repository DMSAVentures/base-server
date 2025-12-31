-- Migration: Refactor Email Templates and Email Blasts
--
-- Changes:
-- 1. Rename email_templates → campaign_email_templates (campaign-scoped, for automated emails)
-- 2. Create blast_email_templates (account-scoped, for manual email blasts)
-- 3. Update email_blasts to be account-scoped with multi-segment support

-- ============================================================================
-- STEP 1: Rename email_templates to campaign_email_templates
-- ============================================================================

ALTER TABLE email_templates RENAME TO campaign_email_templates;

-- Rename indexes
ALTER INDEX idx_email_templates_campaign RENAME TO idx_campaign_email_templates_campaign;
ALTER INDEX idx_email_templates_type RENAME TO idx_campaign_email_templates_type;

-- ============================================================================
-- STEP 2: Create blast_email_templates table (account-scoped)
-- ============================================================================

CREATE TABLE blast_email_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    subject VARCHAR(255) NOT NULL,

    -- Email content
    html_body TEXT NOT NULL DEFAULT '',
    blocks_json JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,

    UNIQUE(account_id, name)
);

CREATE INDEX idx_blast_email_templates_account ON blast_email_templates(account_id);
CREATE INDEX idx_blast_email_templates_account_active ON blast_email_templates(account_id) WHERE deleted_at IS NULL;

-- ============================================================================
-- STEP 3: Update email_blasts table
-- ============================================================================

-- Add new columns
ALTER TABLE email_blasts ADD COLUMN account_id UUID REFERENCES accounts(id) ON DELETE CASCADE;
ALTER TABLE email_blasts ADD COLUMN segment_ids UUID[] NOT NULL DEFAULT '{}';
ALTER TABLE email_blasts ADD COLUMN blast_template_id UUID REFERENCES blast_email_templates(id) ON DELETE RESTRICT;

-- Backfill account_id from campaigns
UPDATE email_blasts eb
SET account_id = c.account_id
FROM campaigns c
WHERE eb.campaign_id = c.id;

-- Migrate segment_id to segment_ids array (for existing blasts)
UPDATE email_blasts
SET segment_ids = ARRAY[segment_id]
WHERE segment_id IS NOT NULL;

-- For existing blasts that reference email_templates (now campaign_email_templates),
-- we need to create corresponding blast_email_templates and link them.
-- This creates a blast template copy for each unique template used in blasts.

-- First, create blast templates from campaign templates that were used in blasts
INSERT INTO blast_email_templates (id, account_id, name, subject, html_body, blocks_json, created_at, updated_at)
SELECT
    uuid_generate_v4(),
    c.account_id,
    CONCAT(cet.name, ' (Migrated from ', ca.name, ')'),
    cet.subject,
    COALESCE(cet.html_body, ''),
    cet.blocks_json,
    cet.created_at,
    cet.updated_at
FROM email_blasts eb
JOIN campaign_email_templates cet ON eb.template_id = cet.id
JOIN campaigns c ON eb.campaign_id = c.id
JOIN campaigns ca ON cet.campaign_id = ca.id
WHERE eb.template_id IS NOT NULL
AND eb.blast_template_id IS NULL
ON CONFLICT (account_id, name) DO NOTHING;

-- Link existing blasts to their new blast templates
UPDATE email_blasts eb
SET blast_template_id = bet.id
FROM campaign_email_templates cet
JOIN campaigns c ON cet.campaign_id = c.id
JOIN blast_email_templates bet ON bet.account_id = c.account_id
    AND bet.name = CONCAT(cet.name, ' (Migrated from ', c.name, ')')
WHERE eb.template_id = cet.id
AND eb.blast_template_id IS NULL;

-- Make account_id NOT NULL after backfill
ALTER TABLE email_blasts ALTER COLUMN account_id SET NOT NULL;

-- Drop old columns and constraints
ALTER TABLE email_blasts DROP CONSTRAINT IF EXISTS email_blasts_segment_id_fkey;
ALTER TABLE email_blasts DROP CONSTRAINT IF EXISTS email_blasts_template_id_fkey;
ALTER TABLE email_blasts DROP CONSTRAINT IF EXISTS email_blasts_campaign_id_fkey;

DROP INDEX IF EXISTS idx_email_blasts_campaign;
DROP INDEX IF EXISTS idx_email_blasts_segment;

ALTER TABLE email_blasts DROP COLUMN campaign_id;
ALTER TABLE email_blasts DROP COLUMN segment_id;
ALTER TABLE email_blasts DROP COLUMN template_id;

-- Create new indexes
CREATE INDEX idx_email_blasts_account ON email_blasts(account_id);
CREATE INDEX idx_email_blasts_account_status ON email_blasts(account_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_email_blasts_blast_template ON email_blasts(blast_template_id);
CREATE INDEX idx_email_blasts_segment_ids ON email_blasts USING GIN(segment_ids);

-- ============================================================================
-- STEP 4: Update email_logs to handle both template types
-- ============================================================================

-- Add blast_template_id column (campaign template_id already exists as template_id)
ALTER TABLE email_logs ADD COLUMN blast_template_id UUID REFERENCES blast_email_templates(id) ON DELETE SET NULL;

-- Rename template_id to campaign_template_id for clarity
ALTER TABLE email_logs RENAME COLUMN template_id TO campaign_template_id;

-- Update foreign key constraint name
ALTER TABLE email_logs DROP CONSTRAINT IF EXISTS email_logs_template_id_fkey;
ALTER TABLE email_logs ADD CONSTRAINT email_logs_campaign_template_id_fkey
    FOREIGN KEY (campaign_template_id) REFERENCES campaign_email_templates(id) ON DELETE SET NULL;

CREATE INDEX idx_email_logs_blast_template ON email_logs(blast_template_id) WHERE blast_template_id IS NOT NULL;
