-- Add CloudFront geolocation and device detection columns to waitlist_users
-- These columns capture data from CloudFront headers for analytics

-- Create ENUMs for device type and OS
CREATE TYPE device_type AS ENUM ('desktop', 'mobile', 'tablet', 'smarttv', 'unknown');
CREATE TYPE device_os AS ENUM ('android', 'ios', 'other');

-- Geographic data from CloudFront
ALTER TABLE waitlist_users ADD COLUMN IF NOT EXISTS country VARCHAR(100);
ALTER TABLE waitlist_users ADD COLUMN IF NOT EXISTS region VARCHAR(100);
ALTER TABLE waitlist_users ADD COLUMN IF NOT EXISTS region_code VARCHAR(10);
ALTER TABLE waitlist_users ADD COLUMN IF NOT EXISTS postal_code VARCHAR(20);
ALTER TABLE waitlist_users ADD COLUMN IF NOT EXISTS user_timezone VARCHAR(50);
ALTER TABLE waitlist_users ADD COLUMN IF NOT EXISTS latitude DECIMAL(10, 7);
ALTER TABLE waitlist_users ADD COLUMN IF NOT EXISTS longitude DECIMAL(10, 7);
ALTER TABLE waitlist_users ADD COLUMN IF NOT EXISTS metro_code VARCHAR(10);

-- Device detection from CloudFront (using enums)
ALTER TABLE waitlist_users ADD COLUMN IF NOT EXISTS device_type device_type;
ALTER TABLE waitlist_users ADD COLUMN IF NOT EXISTS device_os device_os;

-- Connection info from CloudFront
ALTER TABLE waitlist_users ADD COLUMN IF NOT EXISTS asn VARCHAR(20);
ALTER TABLE waitlist_users ADD COLUMN IF NOT EXISTS tls_version VARCHAR(20);
ALTER TABLE waitlist_users ADD COLUMN IF NOT EXISTS http_version VARCHAR(10);

-- Add indexes for commonly queried CloudFront data
CREATE INDEX IF NOT EXISTS idx_waitlist_users_country ON waitlist_users(country) WHERE country IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_waitlist_users_device_type ON waitlist_users(device_type) WHERE device_type IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_waitlist_users_device_os ON waitlist_users(device_os) WHERE device_os IS NOT NULL;

-- Add similar columns to user_activity_logs for event-level tracking
ALTER TABLE user_activity_logs ADD COLUMN IF NOT EXISTS country VARCHAR(100);
ALTER TABLE user_activity_logs ADD COLUMN IF NOT EXISTS region VARCHAR(100);
ALTER TABLE user_activity_logs ADD COLUMN IF NOT EXISTS city VARCHAR(100);
ALTER TABLE user_activity_logs ADD COLUMN IF NOT EXISTS device_type device_type;
ALTER TABLE user_activity_logs ADD COLUMN IF NOT EXISTS device_os device_os;

-- Add index for geographic analysis on activity logs
CREATE INDEX IF NOT EXISTS idx_user_activity_logs_country ON user_activity_logs(country) WHERE country IS NOT NULL;
