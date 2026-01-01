.PHONY: dev-up dev-down dev-test-unit dev-test-store dev-test-integration \
       docker-up docker-down docker-test-unit docker-test-store docker-test-integration docker-test-all \
       docker-logs docker-clean help

# =============================================================================
# LOCAL DEVELOPMENT (uses local DB - may pollute your database)
# =============================================================================

# Local test database configuration
export TEST_DB_HOST ?= localhost
export TEST_DB_PORT ?= 5432
export TEST_DB_USER ?= base_user
export TEST_DB_PASS ?= base_password
export TEST_DB_NAME ?= base_db

# Local API test configuration
export TEST_API_HOST ?= localhost
export TEST_API_PORT ?= 8080

# Start local services (database, kafka, migrations)
dev-up:
	@echo "Starting local dev services..."
	docker compose -f docker-compose.services.yml up -d --wait db kafka
	docker compose -f docker-compose.services.yml up flyway
	@echo "Local services ready!"

# Stop local services
dev-down:
	@echo "Stopping local dev services..."
	docker compose -f docker-compose.services.yml down -v
	@echo "Local services stopped!"

# Run unit tests locally (mocked - no DB needed)
dev-test-unit:
	@echo "Running unit tests locally..."
	go test -short ./internal/.../processor/... ./internal/.../service/...

# Run store tests locally (requires dev-up first)
dev-test-store:
	@echo "Running store tests locally..."
	go test -short ./internal/store/...

# Run integration tests locally (requires running server)
dev-test-integration:
	@echo "Running integration tests locally..."
	go test -v -race -tags=integration ./tests/...

# =============================================================================
# ISOLATED DOCKER TESTING (uses separate test DB - won't pollute local)
# =============================================================================

# Start isolated Docker test environment (all services including server)
docker-up:
	@echo "Starting isolated Docker test environment..."
	docker compose -f docker-compose.test.yml up -d --wait db kafka
	docker compose -f docker-compose.test.yml up flyway
	docker compose -f docker-compose.test.yml up -d --wait server
	@echo "Docker test environment ready!"

# Stop isolated Docker test environment
docker-down:
	@echo "Stopping Docker test environment..."
	docker compose -f docker-compose.test.yml down -v
	@echo "Docker test environment stopped!"

# Run unit tests in Docker (no DB needed)
docker-test-unit:
	@echo "Running unit tests in Docker..."
	docker compose -f docker-compose.test.yml run --rm --no-deps runner \
		go test -short ./internal/.../processor/... ./internal/.../service/...

# Run store tests in Docker (isolated DB)
docker-test-store:
	@echo "Running store tests in Docker..."
	docker compose -f docker-compose.test.yml run --rm --no-deps runner \
		go test -short ./internal/store/...

# Run integration tests in Docker (with test server)
docker-test-integration:
	@echo "Running integration tests in Docker..."
	docker compose -f docker-compose.test.yml run --rm --no-deps runner \
		go test -v -race -tags=integration ./tests/...

# Run all tests in isolated Docker environment
docker-test-all:
	@echo "=========================================="
	@echo "Running all tests in Docker environment"
	@echo "=========================================="
	@$(MAKE) docker-up
	@echo ""
	@echo ">>> Running unit tests..."
	@$(MAKE) docker-test-unit
	@echo ""
	@echo ">>> Running store tests..."
	@$(MAKE) docker-test-store
	@echo ""
	@echo ">>> Running integration tests..."
	@$(MAKE) docker-test-integration
	@echo ""
	@echo ">>> Cleaning up..."
	@$(MAKE) docker-down
	@echo "=========================================="
	@echo "All tests completed!"
	@echo "=========================================="

# View Docker test server logs (for debugging)
docker-logs:
	docker compose -f docker-compose.test.yml logs server

# Clean up all Docker test resources (volumes, images, containers)
docker-clean:
	@echo "Cleaning up Docker test resources..."
	docker compose -f docker-compose.test.yml down -v --rmi local --remove-orphans
	@echo "Cleanup complete!"

# =============================================================================
# HELP
# =============================================================================

help:
	@echo "Available commands:"
	@echo ""
	@echo "LOCAL DEVELOPMENT (uses local DB on port 5432):"
	@echo "  make dev-up               - Start local services (db, kafka, migrations)"
	@echo "  make dev-down             - Stop local services"
	@echo "  make dev-test-unit        - Run unit tests locally (no DB needed)"
	@echo "  make dev-test-store       - Run store tests locally (requires dev-up)"
	@echo "  make dev-test-integration - Run integration tests locally"
	@echo ""
	@echo "ISOLATED DOCKER TESTING (fully containerized, no exposed ports):"
	@echo "  make docker-up            - Start isolated Docker test environment"
	@echo "  make docker-down          - Stop Docker test environment"
	@echo "  make docker-test-unit     - Run unit tests in Docker"
	@echo "  make docker-test-store    - Run store tests in Docker"
	@echo "  make docker-test-integration - Run integration tests in Docker"
	@echo "  make docker-test-all      - Run ALL tests in Docker (recommended)"
	@echo "  make docker-logs          - View test server logs (for debugging)"
	@echo "  make docker-clean         - Clean up all Docker test resources"
