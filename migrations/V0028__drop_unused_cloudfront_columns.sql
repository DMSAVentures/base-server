-- Drop unused CloudFront columns (not forwarded due to 10 header limit)
ALTER TABLE waitlist_users DROP COLUMN IF EXISTS asn;
ALTER TABLE waitlist_users DROP COLUMN IF EXISTS tls_version;
ALTER TABLE waitlist_users DROP COLUMN IF EXISTS http_version;
