-- Add default positions_per_referral configuration to existing campaigns
-- This enables configurable position jumps per referral (e.g., 1 referral = 5 positions)

-- Update all campaigns to have default positions_per_referral = 1
-- This maintains backward compatibility with existing behavior
UPDATE campaigns
SET referral_config = jsonb_set(
    COALESCE(referral_config, '{}'::jsonb),
    '{positions_per_referral}',
    '1'
)
WHERE referral_config->>'positions_per_referral' IS NULL;

-- Add comment explaining the configuration
COMMENT ON COLUMN campaigns.referral_config IS 'Referral configuration including positions_per_referral (default 1), verified_only, and sharing channels';

-- Add index for efficient position-based queries (if not exists)
-- This supports efficient sorting and filtering by position
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_waitlist_users_position_lookup
ON waitlist_users(campaign_id, position, created_at)
WHERE deleted_at IS NULL;

-- Add comment explaining position calculation
COMMENT ON COLUMN waitlist_users.position IS 'User position in waitlist. Calculated as: original_position - (referral_count × positions_per_referral). -1 indicates position is being calculated.';
