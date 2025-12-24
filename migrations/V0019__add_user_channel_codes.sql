-- Migration: Add user_channel_codes table for channel-specific referral codes
-- Each user can have a unique referral code per sharing channel (twitter, facebook, etc.)

CREATE TABLE user_channel_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES waitlist_users(id) ON DELETE CASCADE,
    channel VARCHAR(20) NOT NULL,  -- twitter, facebook, linkedin, whatsapp, email
    code VARCHAR(20) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),

    -- Each user can only have one code per channel
    CONSTRAINT uq_user_channel UNIQUE(user_id, channel),
    -- Each code must be globally unique
    CONSTRAINT uq_channel_code UNIQUE(code)
);

-- Index for looking up codes by user
CREATE INDEX idx_user_channel_codes_user ON user_channel_codes(user_id);

-- Index for looking up user by code (when processing referrals)
CREATE INDEX idx_user_channel_codes_code ON user_channel_codes(code);

COMMENT ON TABLE user_channel_codes IS 'Stores channel-specific referral codes for each user';
COMMENT ON COLUMN user_channel_codes.channel IS 'Sharing channel: twitter, facebook, linkedin, whatsapp, email';
COMMENT ON COLUMN user_channel_codes.code IS 'Unique referral code for this channel, format: {base}_{suffix}';
