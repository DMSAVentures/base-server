-- Add blocks_json column to email_templates table
-- This stores the block-based editor structure as JSON

ALTER TABLE email_templates
ADD COLUMN blocks_json JSONB;

-- Add comment
COMMENT ON COLUMN email_templates.blocks_json IS 'JSON structure of email builder blocks for the visual editor';
