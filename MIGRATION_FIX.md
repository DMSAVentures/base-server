# Fix for Missing webhook_deliveries Table

## Problem
The `webhook_deliveries` table is missing from your database because migration `V0016__add_webhooks.sql` hasn't been applied yet.

## Solution Options

### Option 1: Run Migrations Using Docker Compose (Recommended)

This is the cleanest approach as it will run ALL pending migrations:

```bash
# Start the database and run migrations
docker-compose -f docker-compose.services.yml up db flyway

# Wait for migrations to complete, then stop the services
# Press Ctrl+C or run:
docker-compose -f docker-compose.services.yml down
```

### Option 2: Run Migrations Using Custom Flyway Container

If you have a remote database and need to run migrations against it:

```bash
# 1. Build the migration image
docker build -t flyway-migrate -f dbmigrator.dockerfile .

# 2. Run migrations (replace with your actual DB credentials)
docker run --platform linux/amd64 --rm \
  -e DB_HOST=your-db-host \
  -e DB_USERNAME=your-db-user \
  -e DB_PASSWORD=your-db-password \
  flyway-migrate
```

### Option 3: Manual SQL Execution

If you can't use Docker, run this SQL directly against your database:

```sql
-- Create ENUMs (if not already created)
DO $$ BEGIN
    CREATE TYPE webhook_status AS ENUM ('active', 'paused', 'failed');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE webhook_delivery_status AS ENUM ('pending', 'success', 'failed');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Create Webhooks Table
CREATE TABLE IF NOT EXISTS webhooks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    campaign_id UUID REFERENCES campaigns(id) ON DELETE CASCADE,
    url VARCHAR(500) NOT NULL,
    secret VARCHAR(255) NOT NULL,
    events TEXT[] NOT NULL,
    status webhook_status NOT NULL DEFAULT 'active',
    retry_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    max_retries INTEGER NOT NULL DEFAULT 5,
    total_sent INTEGER NOT NULL DEFAULT 0,
    total_failed INTEGER NOT NULL DEFAULT 0,
    last_success_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_webhooks_account ON webhooks(account_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_campaign ON webhooks(campaign_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_status ON webhooks(status);

-- Create Webhook Deliveries Table
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    status webhook_delivery_status NOT NULL DEFAULT 'pending',
    request_headers JSONB,
    response_status INTEGER,
    response_body TEXT,
    response_headers JSONB,
    duration_ms INTEGER,
    attempt_number INTEGER NOT NULL DEFAULT 1,
    next_retry_at TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries(status);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_retry ON webhook_deliveries(status, next_retry_at) WHERE status = 'failed';

-- Update Flyway schema history (if using Flyway manually)
-- This ensures Flyway knows this migration was applied
-- INSERT INTO flyway_schema_history (installed_rank, version, description, type, script, checksum, installed_by, execution_time, success)
-- VALUES ((SELECT COALESCE(MAX(installed_rank), 0) + 1 FROM flyway_schema_history), '0016', 'add webhooks', 'SQL', 'V0016__add_webhooks.sql', NULL, current_user, 0, true);
```

## Verification

After running migrations, verify the tables exist:

```sql
-- Check if tables exist
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name IN ('webhooks', 'webhook_deliveries');

-- Check if ENUMs exist
SELECT typname
FROM pg_type
WHERE typname IN ('webhook_status', 'webhook_delivery_status');
```

## Next Steps

After applying the migrations:

1. Restart your server
2. Verify webhook functionality is working
3. Check that no more "relation does not exist" errors appear in logs

## Additional Notes

- Migration file location: `migrations/V0016__add_webhooks.sql`
- The migration creates both `webhooks` and `webhook_deliveries` tables
- Make sure Kafka is also running if you need webhook event streaming (port 9092)
