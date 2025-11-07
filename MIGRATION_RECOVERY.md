# Migration Recovery Guide

## If You're Stuck at Migration V0014 or V0015

Your migrations failed because V0015 required TimescaleDB which wasn't installed. This has been fixed.

### Quick Recovery Steps

#### Step 1: Verify Your Current Migration State

```bash
# Check which migrations have been applied
docker-compose -f docker-compose.services.yml up -d db
docker exec -it $(docker ps -qf "name=db") psql -U base_user -d base_db -c "SELECT version, description FROM flyway_schema_history ORDER BY installed_rank;"
```

You should see migrations up to V0014 or a failed V0015.

#### Step 2: Apply the Fix

Since the migration files have been updated, you have two options:

**Option A: Retry Migration (Recommended)**

Pull the latest code with the fixes and retry:

```bash
# Stop services
docker-compose -f docker-compose.services.yml down

# Pull latest code (if using git)
git pull

# Start database
docker-compose -f docker-compose.services.yml up -d db

# Wait for database to be ready (about 5 seconds)
sleep 5

# Run migrations - V0015 will now succeed
docker-compose -f docker-compose.services.yml up flyway
```

**Option B: Manual Recovery**

If Flyway marked V0015 as failed in the schema history:

```bash
# 1. Connect to database
docker exec -it $(docker ps -qf "name=db") psql -U base_user -d base_db

# 2. Check for failed migration record
SELECT * FROM flyway_schema_history WHERE version = '0015' AND success = false;

# 3. If found, delete the failed record
DELETE FROM flyway_schema_history WHERE version = '0015' AND success = false;

# 4. Exit psql
\q

# 5. Retry migrations
docker-compose -f docker-compose.services.yml up flyway
```

#### Step 3: Verify All Migrations Completed

```bash
# Check migrations again
docker exec -it $(docker ps -qf "name=db") psql -U base_user -d base_db -c "SELECT version, description FROM flyway_schema_history WHERE success = true ORDER BY installed_rank;"
```

You should now see migrations V0001 through V0017.

#### Step 4: Verify webhook_deliveries Table Exists

```bash
docker exec -it $(docker ps -qf "name=db") psql -U base_user -d base_db -c "\dt webhook*"
```

You should see:
- `webhooks`
- `webhook_deliveries`

### What Changed?

The fix makes TimescaleDB **optional** in migration V0015:

- ✅ Migration V0015 now works with standard PostgreSQL
- ✅ Tables are created as regular tables if TimescaleDB isn't available
- ✅ V0016 and V0017 can now proceed
- ✅ `webhook_deliveries` table will be created

### After Recovery

1. **Start your application**:
   ```bash
   # Start all services
   docker-compose -f docker-compose.services.yml up -d

   # Or start your Go server
   go run main.go
   ```

2. **Verify no errors**: Check logs for "relation does not exist" errors - they should be gone

3. **Optional**: If you want TimescaleDB optimizations, see [MIGRATION_FIX.md](MIGRATION_FIX.md#timescaledb-optional)

## Troubleshooting

### Error: "Flyway schema history table contains entries marked as failed"

```bash
# Clean up failed entries
docker exec -it $(docker ps -qf "name=db") psql -U base_user -d base_db -c \
  "DELETE FROM flyway_schema_history WHERE success = false;"

# Retry migrations
docker-compose -f docker-compose.services.yml up flyway
```

### Error: "Migration checksum mismatch"

This means you modified a migration that was already applied. Options:

1. **Repair Flyway checksums** (recommended):
   ```bash
   docker run --rm \
     -v ./migrations:/flyway/sql \
     -e FLYWAY_URL=jdbc:postgresql://db:5432/base_db \
     -e FLYWAY_USER=base_user \
     -e FLYWAY_PASSWORD=base_password \
     flyway/flyway repair
   ```

2. **Nuclear option** - Reset and rerun all migrations (⚠️ THIS WILL DELETE ALL DATA):
   ```bash
   docker-compose -f docker-compose.services.yml down -v
   docker-compose -f docker-compose.services.yml up -d
   ```

### Still Having Issues?

1. Check the detailed guide: [MIGRATION_FIX.md](MIGRATION_FIX.md)
2. Run the helper script: `./run-migrations.sh`
3. Check Flyway logs: `docker-compose -f docker-compose.services.yml logs flyway`

## Prevention

To avoid migration issues in the future:

1. Always test migrations locally before deploying
2. Use the development docker-compose setup
3. Never modify migrations that have been applied to production
4. Create new migration files for schema changes
