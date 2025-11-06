-- Install TimescaleDB extension for time-series analytics
-- This is required for the campaign_analytics hypertable created in V0015

-- Create extension if not exists (idempotent)
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- If campaign_analytics table already exists but wasn't converted to a hypertable,
-- convert it now (this handles cases where V0015 ran before TimescaleDB was available)
DO $$
BEGIN
    -- Check if the table exists and is not already a hypertable
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'campaign_analytics'
    ) AND NOT EXISTS (
        SELECT 1 FROM timescaledb_information.hypertables
        WHERE hypertable_name = 'campaign_analytics'
    ) THEN
        PERFORM create_hypertable('campaign_analytics', 'time',
                                 if_not_exists => TRUE,
                                 migrate_data => TRUE);
    END IF;
END
$$;
