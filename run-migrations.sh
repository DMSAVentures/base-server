#!/bin/bash
# Migration Runner Script
# This script helps run database migrations against your PostgreSQL database

set -e

echo "=== Base Server Migration Runner ==="
echo ""

# Check if environment variables are set
if [ -f "env.local" ]; then
    echo "Loading environment variables from env.local..."
    export $(cat env.local | grep -v '^#' | xargs)
fi

# Verify required variables
if [ -z "$DB_HOST" ] || [ -z "$DB_USERNAME" ] || [ -z "$DB_PASSWORD" ]; then
    echo "Error: Database credentials not found!"
    echo ""
    echo "Please set the following environment variables:"
    echo "  - DB_HOST"
    echo "  - DB_USERNAME"
    echo "  - DB_PASSWORD"
    echo ""
    echo "You can either:"
    echo "  1. Create an env.local file with these variables"
    echo "  2. Export them in your shell before running this script"
    echo "  3. Pass them as arguments: DB_HOST=... DB_USERNAME=... DB_PASSWORD=... ./run-migrations.sh"
    exit 1
fi

echo "Database Configuration:"
echo "  Host: $DB_HOST"
echo "  Username: $DB_USERNAME"
echo "  Database: ${DB_NAME:-base_db}"
echo ""

# Check if docker is available
if command -v docker &> /dev/null; then
    echo "Docker found. Running migrations using Docker..."
    echo ""

    # Build migration image
    echo "Building migration image..."
    docker build -t flyway-migrate -f dbmigrator.dockerfile .

    # Run migrations
    echo ""
    echo "Running migrations..."
    docker run --platform linux/amd64 --rm \
      -e DB_HOST="$DB_HOST" \
      -e DB_USERNAME="$DB_USERNAME" \
      -e DB_PASSWORD="$DB_PASSWORD" \
      flyway-migrate

    echo ""
    echo "✓ Migrations completed successfully!"

elif command -v psql &> /dev/null; then
    echo "PostgreSQL client (psql) found. Running migrations manually..."
    echo ""

    DB_NAME="${DB_NAME:-base_db}"

    # Check connection
    echo "Testing database connection..."
    if ! PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -U "$DB_USERNAME" -d "$DB_NAME" -c "SELECT 1;" &> /dev/null; then
        echo "Error: Cannot connect to database!"
        exit 1
    fi

    echo "✓ Database connection successful"
    echo ""

    # Check current migration status
    echo "Checking migration status..."
    MIGRATIONS_APPLIED=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -U "$DB_USERNAME" -d "$DB_NAME" -tAc "SELECT COUNT(*) FROM flyway_schema_history;" 2>/dev/null || echo "0")
    echo "Migrations applied: $MIGRATIONS_APPLIED"
    echo ""

    # Check for missing webhook_deliveries table
    TABLE_EXISTS=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -U "$DB_USERNAME" -d "$DB_NAME" -tAc "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'webhook_deliveries');" || echo "f")

    if [ "$TABLE_EXISTS" = "f" ]; then
        echo "⚠ webhook_deliveries table is missing!"
        echo ""
        echo "Applying V0016__add_webhooks.sql migration..."

        if PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -U "$DB_USERNAME" -d "$DB_NAME" -f migrations/V0016__add_webhooks.sql; then
            echo "✓ Migration applied successfully!"
        else
            echo "✗ Migration failed!"
            exit 1
        fi
    else
        echo "✓ webhook_deliveries table already exists"
    fi

    echo ""
    echo "Checking for other pending migrations..."

    # List all migration files
    for migration in migrations/V*.sql; do
        filename=$(basename "$migration")
        version=$(echo "$filename" | sed 's/V\([0-9]*\)__.*/\1/')

        # Check if this migration was applied
        APPLIED=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -U "$DB_USERNAME" -d "$DB_NAME" -tAc "SELECT EXISTS (SELECT 1 FROM flyway_schema_history WHERE version = '$version');" 2>/dev/null || echo "f")

        if [ "$APPLIED" = "f" ]; then
            echo "  Pending: $filename"
        fi
    done

    echo ""
    echo "✓ Migration check completed!"
    echo ""
    echo "Note: For proper migration management, consider using Flyway via Docker."

else
    echo "Error: Neither Docker nor PostgreSQL client (psql) found!"
    echo ""
    echo "Please install one of the following:"
    echo "  1. Docker - https://docs.docker.com/get-docker/"
    echo "  2. PostgreSQL client - sudo apt-get install postgresql-client"
    echo ""
    echo "Or manually run the SQL from migrations/V0016__add_webhooks.sql"
    exit 1
fi

echo ""
echo "=== Migration Process Complete ==="
