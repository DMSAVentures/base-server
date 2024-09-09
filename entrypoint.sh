#!/bin/sh
# Set Flyway configuration variables
export FLYWAY_URL="jdbc:postgresql://$DB_HOST:5432/base_db"
export FLYWAY_USER="$DB_USERNAME"
export FLYWAY_PASSWORD="$DB_PASSWORD"

echo "FLYWAY_URL: $FLYWAY_URL"

# Run the Flyway migration
/flyway/flyway migrate
