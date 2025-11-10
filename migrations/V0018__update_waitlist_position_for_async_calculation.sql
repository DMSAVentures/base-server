-- Update waitlist_users position column to support async calculation
-- Allow position to be -1 to indicate "calculating" or "not yet calculated"

-- Drop existing NOT NULL constraint on position
ALTER TABLE waitlist_users
ALTER COLUMN position DROP NOT NULL;

-- Add CHECK constraint to allow -1 or positive integers
ALTER TABLE waitlist_users
ADD CONSTRAINT check_position_valid CHECK (position = -1 OR position > 0);

-- Set existing NULL positions to -1 (if any)
UPDATE waitlist_users
SET position = -1
WHERE position IS NULL;

-- Update any existing positions that are 0 to -1
UPDATE waitlist_users
SET position = -1
WHERE position = 0;

-- Add comment for clarity
COMMENT ON COLUMN waitlist_users.position IS 'User position in waitlist. -1 indicates position is being calculated or not yet calculated.';
