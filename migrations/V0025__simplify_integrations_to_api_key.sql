-- Simplify integrations to use API keys instead of OAuth
-- This migration removes OAuth token infrastructure for Zapier (now uses API keys)

-- Step 1: Drop the foreign key constraint from integration_subscriptions
ALTER TABLE integration_subscriptions
    DROP CONSTRAINT IF EXISTS integration_subscriptions_token_id_fkey;

-- Step 2: Drop the token_id column (no longer needed - was referencing OAuth tokens, not API keys)
ALTER TABLE integration_subscriptions
    DROP COLUMN IF EXISTS token_id;

-- Step 3: Add api_key_id column to track which API key created the subscription
-- Note: Existing subscriptions will have NULL api_key_id
ALTER TABLE integration_subscriptions
    ADD COLUMN IF NOT EXISTS api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL;

-- Step 4: Drop indexes related to OAuth tokens
DROP INDEX IF EXISTS idx_int_subs_token;
DROP INDEX IF EXISTS idx_int_oauth_account;
DROP INDEX IF EXISTS idx_int_oauth_hash;
DROP INDEX IF EXISTS idx_int_oauth_account_type;

-- Step 5: Drop the OAuth tokens table (no longer needed for Zapier)
DROP TABLE IF EXISTS integration_oauth_tokens;

-- Step 6: Create new index for api_key_id
CREATE INDEX IF NOT EXISTS idx_int_subs_api_key ON integration_subscriptions(api_key_id)
    WHERE deleted_at IS NULL;
