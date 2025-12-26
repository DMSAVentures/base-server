-- Remove text_body column from email_templates table
-- Text body is not used; HTML is the only format sent via Resend

ALTER TABLE email_templates
DROP COLUMN IF EXISTS text_body;
