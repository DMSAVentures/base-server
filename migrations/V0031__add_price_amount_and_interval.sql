-- Add unit_amount, currency, and interval columns to prices table
-- These fields come from Stripe price data to support displaying pricing info

ALTER TABLE prices
ADD COLUMN unit_amount BIGINT,
ADD COLUMN currency VARCHAR(3),
ADD COLUMN interval VARCHAR(20);

COMMENT ON COLUMN prices.unit_amount IS 'Price amount in cents (e.g., 999 = $9.99)';
COMMENT ON COLUMN prices.currency IS 'ISO 4217 currency code (e.g., usd, eur)';
COMMENT ON COLUMN prices.interval IS 'Billing interval: month, year, week, day';
