#!/bin/bash
# Quick setup script for TimescaleDB on AWS RDS PostgreSQL
# This script helps configure the parameter group and install the extension

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
DB_INSTANCE_NAME="${DB_INSTANCE_NAME:-}"
DB_HOST="${DB_HOST:-}"
DB_USERNAME="${DB_USERNAME:-}"
DB_PASSWORD="${DB_PASSWORD:-}"
DB_NAME="${DB_NAME:-}"
PG_VERSION="${PG_VERSION:-15}"

echo "======================================"
echo "TimescaleDB AWS RDS Setup Script"
echo "======================================"
echo ""

# Check if AWS CLI is installed
if ! command -v aws &> /dev/null; then
    echo -e "${YELLOW}Warning: AWS CLI not found. AWS CLI steps will be skipped.${NC}"
    echo "You can install it from: https://aws.amazon.com/cli/"
    SKIP_AWS=true
fi

# Check environment variables
if [ -z "$DB_INSTANCE_NAME" ] && [ "$SKIP_AWS" != true ]; then
    read -p "Enter RDS instance name: " DB_INSTANCE_NAME
fi

if [ -z "$DB_HOST" ]; then
    read -p "Enter database host: " DB_HOST
fi

if [ -z "$DB_USERNAME" ]; then
    read -p "Enter database username: " DB_USERNAME
fi

if [ -z "$DB_PASSWORD" ]; then
    read -sp "Enter database password: " DB_PASSWORD
    echo ""
fi

if [ -z "$DB_NAME" ]; then
    read -p "Enter database name: " DB_NAME
fi

echo ""
echo "======================================"
echo "Step 1: Check Current Configuration"
echo "======================================"
echo ""

if [ "$SKIP_AWS" != true ]; then
    echo "Checking RDS instance..."
    aws rds describe-db-instances \
        --db-instance-identifier "$DB_INSTANCE_NAME" \
        --query 'DBInstances[0].[EngineVersion,DBParameterGroups[0].DBParameterGroupName]' \
        --output table || echo -e "${RED}Failed to describe instance. Check instance name and AWS credentials.${NC}"
fi

echo ""
echo "======================================"
echo "Step 2: Create Parameter Group"
echo "======================================"
echo ""

if [ "$SKIP_AWS" != true ]; then
    PARAM_GROUP_NAME="postgres-${PG_VERSION}-timescaledb"

    echo "Creating parameter group: $PARAM_GROUP_NAME"
    aws rds create-db-parameter-group \
        --db-parameter-group-name "$PARAM_GROUP_NAME" \
        --db-parameter-group-family "postgres${PG_VERSION}" \
        --description "PostgreSQL ${PG_VERSION} with TimescaleDB support" 2>/dev/null \
        && echo -e "${GREEN}✓ Parameter group created${NC}" \
        || echo -e "${YELLOW}Parameter group may already exist${NC}"

    echo ""
    echo "Modifying shared_preload_libraries parameter..."
    aws rds modify-db-parameter-group \
        --db-parameter-group-name "$PARAM_GROUP_NAME" \
        --parameters "ParameterName=shared_preload_libraries,ParameterValue=timescaledb,ApplyMethod=pending-reboot" \
        && echo -e "${GREEN}✓ Parameter updated${NC}"

    echo ""
    echo "======================================"
    echo "Step 3: Apply Parameter Group"
    echo "======================================"
    echo ""

    echo "Applying parameter group to instance..."
    aws rds modify-db-instance \
        --db-instance-identifier "$DB_INSTANCE_NAME" \
        --db-parameter-group-name "$PARAM_GROUP_NAME" \
        --apply-immediately \
        && echo -e "${GREEN}✓ Parameter group applied${NC}"

    echo ""
    echo -e "${YELLOW}Note: A reboot is required for the parameter change to take effect.${NC}"
    read -p "Do you want to reboot the instance now? (y/n): " -n 1 -r
    echo ""

    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Rebooting instance..."
        aws rds reboot-db-instance \
            --db-instance-identifier "$DB_INSTANCE_NAME" \
            && echo -e "${GREEN}✓ Instance is rebooting${NC}"

        echo "Waiting for instance to become available..."
        aws rds wait db-instance-available \
            --db-instance-identifier "$DB_INSTANCE_NAME" \
            && echo -e "${GREEN}✓ Instance is available${NC}"
    else
        echo -e "${YELLOW}Skipping reboot. You must reboot manually before proceeding to Step 4.${NC}"
        exit 0
    fi
else
    echo -e "${YELLOW}AWS CLI not available. Please manually:${NC}"
    echo "1. Create a parameter group with shared_preload_libraries = 'timescaledb'"
    echo "2. Apply it to your RDS instance"
    echo "3. Reboot the instance"
    echo ""
    read -p "Press enter when ready to continue with database setup..."
fi

echo ""
echo "======================================"
echo "Step 4: Install TimescaleDB Extension"
echo "======================================"
echo ""

# Check if psql is installed
if ! command -v psql &> /dev/null; then
    echo -e "${RED}Error: psql not found. Please install PostgreSQL client.${NC}"
    echo ""
    echo "Alternative: Run the migration using Docker:"
    echo "  docker build -t flyway-migrate -f dbmigrator.dockerfile ."
    echo "  docker run --platform linux/amd64 --rm \\"
    echo "    -e DB_HOST=$DB_HOST \\"
    echo "    -e DB_USERNAME=$DB_USERNAME \\"
    echo "    -e DB_PASSWORD=$DB_PASSWORD \\"
    echo "    flyway-migrate"
    exit 1
fi

echo "Installing TimescaleDB extension..."
export PGPASSWORD="$DB_PASSWORD"

psql -h "$DB_HOST" -U "$DB_USERNAME" -d "$DB_NAME" -c "CREATE EXTENSION IF NOT EXISTS timescaledb;" \
    && echo -e "${GREEN}✓ TimescaleDB extension installed${NC}" \
    || echo -e "${RED}Failed to install extension. Check database permissions.${NC}"

echo ""
echo "======================================"
echo "Step 5: Verify Installation"
echo "======================================"
echo ""

echo "Running verification checks..."
psql -h "$DB_HOST" -U "$DB_USERNAME" -d "$DB_NAME" -f scripts/verify_timescaledb.sql \
    && echo -e "${GREEN}✓ Verification complete${NC}" \
    || echo -e "${YELLOW}Verification script failed. Check scripts/verify_timescaledb.sql${NC}"

echo ""
echo "======================================"
echo "Setup Complete!"
echo "======================================"
echo ""
echo "Next steps:"
echo "1. Run database migrations to convert campaign_analytics to hypertable"
echo "2. Test analytics queries using time_bucket() functions"
echo "3. Consider enabling compression and retention policies (see docs)"
echo ""
echo "For detailed documentation, see: docs/AWS_RDS_TIMESCALEDB_SETUP.md"
