#!/bin/bash

# Script to run tests with proper setup and teardown

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Starting test database...${NC}"
docker-compose -f docker-compose.test.yml up -d

echo -e "${YELLOW}Waiting for database to be ready...${NC}"
sleep 3

# Set test environment variables
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5433
export TEST_DB_USER=postgres
export TEST_DB_PASSWORD=postgres
export TEST_DB_TYPE=postgres

echo -e "${YELLOW}Running tests...${NC}"
if go test ./internal/store/... -v -count=1; then
    echo -e "${GREEN}Tests passed!${NC}"
    EXIT_CODE=0
else
    echo -e "${RED}Tests failed!${NC}"
    EXIT_CODE=1
fi

echo -e "${YELLOW}Stopping test database...${NC}"
docker-compose -f docker-compose.test.yml down -v

exit $EXIT_CODE
