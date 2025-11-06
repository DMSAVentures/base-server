# AWS RDS PostgreSQL - TimescaleDB Setup Guide

This guide explains how to install and configure TimescaleDB on AWS RDS PostgreSQL for the base-server analytics layer.

## Overview

TimescaleDB is a PostgreSQL extension used for time-series data analytics in the `campaign_analytics` hypertable. It provides optimized storage and query performance for time-series data.

## Prerequisites

- AWS RDS PostgreSQL instance (version 12.0 or higher recommended)
- Database access credentials with superuser or `rds_superuser` privileges
- AWS Console access or AWS CLI configured

## Installation Steps

### 1. Check TimescaleDB Availability

First, verify that your PostgreSQL version supports TimescaleDB:

**Supported versions:**
- PostgreSQL 12.x: TimescaleDB 2.x
- PostgreSQL 13.x: TimescaleDB 2.x
- PostgreSQL 14.x: TimescaleDB 2.x
- PostgreSQL 15.x: TimescaleDB 2.x

Check your RDS instance version:
```bash
aws rds describe-db-instances \
  --db-instance-identifier your-db-instance-name \
  --query 'DBInstances[0].EngineVersion'
```

### 2. Enable TimescaleDB in Parameter Group

#### Option A: Using AWS Console

1. **Open RDS Console**: Navigate to AWS RDS Console → Parameter Groups

2. **Create Custom Parameter Group** (if not already using one):
   - Click "Create parameter group"
   - Parameter group family: `postgres15` (match your PostgreSQL version)
   - Type: `DB Parameter Group`
   - Group name: `postgres-15-timescaledb`
   - Description: `PostgreSQL 15 with TimescaleDB support`

3. **Modify Parameter Group**:
   - Select your parameter group
   - Click "Edit parameters"
   - Search for `shared_preload_libraries`
   - Add `timescaledb` to the value (comma-separated if other extensions exist)
   - Example: `pg_stat_statements,timescaledb`

4. **Save Changes**: Click "Save changes"

5. **Apply Parameter Group to DB Instance**:
   - Go to RDS Console → Databases
   - Select your database instance
   - Click "Modify"
   - Under "Additional configuration" → "Database options"
   - Select your custom parameter group
   - Click "Continue"
   - Choose "Apply immediately" or during next maintenance window
   - Click "Modify DB instance"

6. **Reboot Database Instance**:
   - Select your database instance
   - Actions → Reboot
   - Confirm reboot
   - Wait for instance to become available (this may take 5-10 minutes)

#### Option B: Using AWS CLI

```bash
# Create parameter group
aws rds create-db-parameter-group \
  --db-parameter-group-name postgres-15-timescaledb \
  --db-parameter-group-family postgres15 \
  --description "PostgreSQL 15 with TimescaleDB support"

# Modify shared_preload_libraries parameter
aws rds modify-db-parameter-group \
  --db-parameter-group-name postgres-15-timescaledb \
  --parameters "ParameterName=shared_preload_libraries,ParameterValue=timescaledb,ApplyMethod=pending-reboot"

# Apply parameter group to instance
aws rds modify-db-instance \
  --db-instance-identifier your-db-instance-name \
  --db-parameter-group-name postgres-15-timescaledb \
  --apply-immediately

# Reboot instance (required for shared_preload_libraries change)
aws rds reboot-db-instance \
  --db-instance-identifier your-db-instance-name
```

### 3. Install TimescaleDB Extension

After the instance has rebooted and is available:

#### Option A: Run Migration (Recommended)

The V0018 migration will automatically install TimescaleDB:

```bash
# Build migration container
docker build -t flyway-migrate -f dbmigrator.dockerfile .

# Run migrations
docker run --platform linux/amd64 --rm \
  -e DB_HOST=$DB_HOST \
  -e DB_USERNAME=$DB_USERNAME \
  -e DB_PASSWORD=$DB_PASSWORD \
  flyway-migrate
```

#### Option B: Manual Installation

Connect to your database and run:

```sql
-- Install TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Verify installation
SELECT extname, extversion FROM pg_extension WHERE extname = 'timescaledb';
```

### 4. Convert Existing Tables to Hypertables

If the `campaign_analytics` table already exists but wasn't converted to a hypertable, the V0018 migration will handle this automatically. Alternatively, run manually:

```sql
-- Convert campaign_analytics to hypertable
SELECT create_hypertable('campaign_analytics', 'time',
                        if_not_exists => TRUE,
                        migrate_data => TRUE);
```

### 5. Verify Installation

Run the verification script:

```bash
# Using psql
psql -h $DB_HOST -U $DB_USERNAME -d $DB_NAME -f scripts/verify_timescaledb.sql

# Or using Docker
docker run -i --rm postgres:15-alpine psql \
  "postgresql://$DB_USERNAME:$DB_PASSWORD@$DB_HOST/$DB_NAME" \
  < scripts/verify_timescaledb.sql
```

Expected output should show:
- ✓ TimescaleDB extension is available
- ✓ TimescaleDB extension is installed
- TimescaleDB version (e.g., 2.11.0)
- ✓ campaign_analytics is a hypertable

## Troubleshooting

### Extension Not Available

**Error**: `extension "timescaledb" is not available`

**Solution**:
1. Verify `shared_preload_libraries` includes `timescaledb`
2. Ensure you rebooted the RDS instance after modifying the parameter
3. Check that your PostgreSQL version supports TimescaleDB

Check parameter value:
```sql
SHOW shared_preload_libraries;
-- Should include: timescaledb
```

### Permission Denied

**Error**: `permission denied to create extension "timescaledb"`

**Solution**: Connect with a user that has `rds_superuser` role:

```sql
-- Check your role
SELECT current_user, session_user;

-- Grant rds_superuser (run as admin user)
GRANT rds_superuser TO your_username;
```

### Table Already Exists

**Error**: `table "campaign_analytics" already exists`

**Solution**: If the table exists but isn't a hypertable, the V0018 migration will convert it. You can also run:

```sql
-- Convert existing table
SELECT create_hypertable('campaign_analytics', 'time',
                        migrate_data => TRUE,
                        if_not_exists => TRUE);
```

### Performance Impact

**Q**: Will this impact my database performance?

**A**:
- The `shared_preload_libraries` change requires a reboot but has minimal performance impact
- TimescaleDB improves performance for time-series queries
- Initial hypertable creation with `migrate_data => TRUE` may take time if you have existing data
- Consider running during low-traffic periods

## Cost Considerations

- TimescaleDB extension itself is free
- AWS RDS charges standard rates
- Storage may decrease due to TimescaleDB's compression features
- Query performance improvements may reduce compute costs

## Post-Installation

After successful installation:

1. **Monitor Performance**: Check query performance on analytics endpoints
2. **Configure Retention Policies** (optional):
   ```sql
   -- Auto-delete data older than 90 days
   SELECT add_retention_policy('campaign_analytics', INTERVAL '90 days');
   ```

3. **Enable Compression** (optional, for cost savings):
   ```sql
   -- Compress chunks older than 7 days
   ALTER TABLE campaign_analytics SET (
     timescaledb.compress,
     timescaledb.compress_segmentby = 'campaign_id'
   );

   SELECT add_compression_policy('campaign_analytics', INTERVAL '7 days');
   ```

4. **Update Application**: Ensure analytics queries use TimescaleDB functions like `time_bucket()`

## Reference

- [TimescaleDB Documentation](https://docs.timescale.com/)
- [AWS RDS PostgreSQL Extensions](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_PostgreSQL.html#PostgreSQL.Concepts.General.Extensions)
- [TimescaleDB on AWS RDS](https://docs.timescale.com/self-hosted/latest/install/installation-rds/)

## Support

For issues or questions:
1. Run the verification script: `scripts/verify_timescaledb.sql`
2. Check RDS instance logs in AWS CloudWatch
3. Review PostgreSQL error logs
