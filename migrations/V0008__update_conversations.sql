ALTER TABLE messages DROP COLUMN IF EXISTS token_count;

ALTER TABLE usage_logs DROP COLUMN IF EXISTS message_id;