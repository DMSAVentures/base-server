-- Add index for velocity detection in spam detection
-- This index optimizes queries that count recent signups from the same IP address

CREATE INDEX IF NOT EXISTS idx_waitlist_users_velocity
ON waitlist_users(campaign_id, ip_address, created_at)
WHERE deleted_at IS NULL;
