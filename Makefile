.PHONY: services-up services-down test-unit test-store test-integration help

# Test database configuration
export TEST_DB_HOST ?= localhost
export TEST_DB_PORT ?= 5432
export TEST_DB_USER ?= base_user
export TEST_DB_PASS ?= base_password
export TEST_DB_NAME ?= base_db

# API test configuration
export TEST_API_HOST ?= localhost
export TEST_API_PORT ?= 8080

# Start all services (database, kafka, migrations)
services-up:
	@echo "Starting services..."
	docker compose -f docker-compose.services.yml up -d --wait db kafka
	docker compose -f docker-compose.services.yml up flyway
	@echo "Services ready!"

# Stop all services
services-down:
	@echo "Stopping services..."
	docker compose -f docker-compose.services.yml down -v
	@echo "Services stopped!"

# Run unit tests (mocked - no DB needed)
test-unit:
	@echo "Running unit tests..."
	go test -short ./internal/.../processor/... ./internal/.../service/...

# Run store tests (requires DB - run services-up first)
test-store:
	@echo "Running store tests..."
	go test -short ./internal/store/...

# Run integration tests (requires running server - run services-up first)
test-integration:
	@echo "Running integration tests..."
	go test -v -race -tags=integration ./tests/...

# Help command
help:
	@echo "Available commands:"
	@echo "  make services-up       - Start services (db, kafka, migrations)"
	@echo "  make services-down     - Stop all services"
	@echo "  make test-unit         - Run unit tests (mocked, no DB needed)"
	@echo "  make test-store        - Run store tests (requires services-up)"
	@echo "  make test-integration  - Run integration tests (requires running server)"
