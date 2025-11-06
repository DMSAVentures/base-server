.PHONY: test test-setup test-teardown test-db-up test-db-down test-store test-verbose test-coverage

# Test database configuration
TEST_DB_HOST ?= localhost
TEST_DB_PORT ?= 5433
TEST_DB_USER ?= postgres
TEST_DB_PASSWORD ?= postgres
TEST_DB_NAME ?= test_db

# Export test database environment variables
export TEST_DB_HOST
export TEST_DB_PORT
export TEST_DB_USER
export TEST_DB_PASSWORD
export TEST_DB_NAME
export TEST_DB_TYPE ?= postgres

# Start the test database
test-db-up:
	@echo "Starting test database..."
	docker-compose -f docker-compose.test.yml up -d
	@echo "Waiting for database to be ready..."
	@sleep 3
	@echo "Test database is ready!"

# Stop the test database
test-db-down:
	@echo "Stopping test database..."
	docker-compose -f docker-compose.test.yml down -v
	@echo "Test database stopped!"

# Run all tests with test database
test: test-db-up
	@echo "Running tests..."
	go test ./internal/store/... -v -count=1
	@$(MAKE) test-db-down

# Run tests without verbose output
test-quiet: test-db-up
	@echo "Running tests..."
	go test ./internal/store/... -count=1
	@$(MAKE) test-db-down

# Run store tests only
test-store: test-db-up
	@echo "Running store tests..."
	go test ./internal/store -v -count=1
	@$(MAKE) test-db-down

# Run tests with coverage
test-coverage: test-db-up
	@echo "Running tests with coverage..."
	go test ./internal/store/... -cover -coverprofile=coverage.out -count=1
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@$(MAKE) test-db-down

# Run a specific test
test-one: test-db-up
	@echo "Running specific test: $(TEST)"
	go test ./internal/store -v -run $(TEST) -count=1
	@$(MAKE) test-db-down

# Run tests in watch mode (requires watchexec: brew install watchexec)
test-watch: test-db-up
	@echo "Running tests in watch mode..."
	watchexec -e go -w internal/store -- go test ./internal/store/... -v -count=1

# Clean up test artifacts
test-clean:
	rm -f coverage.out coverage.html

# Setup test environment (install dependencies)
test-setup:
	@echo "Setting up test environment..."
	go mod download
	@echo "Test environment ready!"

# Full test suite with setup and cleanup
test-full: test-setup test test-clean
	@echo "All tests completed!"

# Help command
help:
	@echo "Available commands:"
	@echo "  make test              - Run all tests with Docker database"
	@echo "  make test-quiet        - Run tests without verbose output"
	@echo "  make test-store        - Run store tests only"
	@echo "  make test-coverage     - Run tests with coverage report"
	@echo "  make test-one TEST=... - Run a specific test (e.g., make test-one TEST=TestStore_CreateUser)"
	@echo "  make test-watch        - Run tests in watch mode (requires watchexec)"
	@echo "  make test-db-up        - Start test database"
	@echo "  make test-db-down      - Stop test database"
	@echo "  make test-clean        - Clean up test artifacts"
	@echo "  make test-setup        - Setup test environment"
	@echo "  make test-full         - Run full test suite with setup and cleanup"
