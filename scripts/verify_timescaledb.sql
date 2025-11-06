-- Verification script for TimescaleDB installation
-- Run this script to check if TimescaleDB is properly installed and configured

-- 1. Check if TimescaleDB extension is available
SELECT
    CASE
        WHEN EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'timescaledb')
        THEN '✓ TimescaleDB extension is available'
        ELSE '✗ TimescaleDB extension is NOT available - check AWS RDS parameter group'
    END AS extension_availability;

-- 2. Check if TimescaleDB extension is installed
SELECT
    CASE
        WHEN EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb')
        THEN '✓ TimescaleDB extension is installed'
        ELSE '✗ TimescaleDB extension is NOT installed - run V0015 migration or CREATE EXTENSION timescaledb'
    END AS extension_installation;

-- 3. Check TimescaleDB version (if installed)
SELECT
    CASE
        WHEN EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb')
        THEN (SELECT extversion FROM pg_extension WHERE extname = 'timescaledb')
        ELSE 'N/A - extension not installed'
    END AS timescaledb_version;

-- 4. List all hypertables (should include campaign_analytics)
SELECT
    'Hypertables:' AS info,
    COALESCE(
        (SELECT string_agg(hypertable_name, ', ')
         FROM timescaledb_information.hypertables),
        'None - extension may not be installed'
    ) AS hypertables;

-- 5. Check if campaign_analytics is a hypertable
SELECT
    CASE
        WHEN EXISTS (
            SELECT 1 FROM timescaledb_information.hypertables
            WHERE hypertable_name = 'campaign_analytics'
        )
        THEN '✓ campaign_analytics is a hypertable'
        WHEN EXISTS (
            SELECT 1 FROM information_schema.tables
            WHERE table_name = 'campaign_analytics'
        )
        THEN '⚠ campaign_analytics exists but is NOT a hypertable - run V0015 migration'
        ELSE '✗ campaign_analytics table does not exist'
    END AS campaign_analytics_status;

-- 6. Show hypertable details (if exists)
SELECT
    hypertable_schema,
    hypertable_name,
    num_dimensions,
    num_chunks,
    compression_enabled,
    tablespaces
FROM timescaledb_information.hypertables
WHERE hypertable_name = 'campaign_analytics';
