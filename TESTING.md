# Testing Guide

This document describes how to run tests for the base-server project, including both store layer tests and API integration tests.

## Overview

The test suite includes:
- **Store Layer Tests**: Database operations and models
- **API Integration Tests**: End-to-end HTTP API testing
- **Table-driven tests** for comprehensive coverage
- **Real database migrations** to ensure schema compatibility
- **PostgreSQL & Kafka** (via Docker) for testing infrastructure

## Prerequisites

- Go 1.23+
- Docker and Docker Compose
- Make (optional, for convenience commands)

## Quick Start

### Store Tests (No Server Required)

Run database/store layer tests:

```bash
# Start test infrastructure (database, Kafka)
make test-db-up

# Run all store tests
make test

# Run tests with coverage report
make test-coverage

# Run a specific test
make test-one TEST=TestStore_CreateUser

# Stop test infrastructure
make test-db-down
```

### API Integration Tests (Requires Running Server)

Run end-to-end API tests:

**Terminal 1** - Start the server:
```bash
make test-db-up
go run main.go
```

**Terminal 2** - Run API tests:
```bash
make test-api
```

When done:
```bash
make test-db-down
```

### Manual Setup

If you prefer to run tests manually:

1. **Start test infrastructure:**
```bash
docker compose -f docker-compose.services.yml up -d
```

2. **Set environment variables:**
```bash
# Database
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5432
export TEST_DB_USER=base_user
export TEST_DB_PASSWORD=base_password
export TEST_DB_NAME=base_db

# API (for API tests)
export TEST_API_HOST=localhost
export TEST_API_PORT=8080
```

3. **Run tests:**
```bash
# Store tests
go test ./internal/store/... -v

# API tests (requires server running)
go test -v -tags=integration ./tests/...
```

4. **Stop test infrastructure:**
```bash
docker compose -f docker-compose.services.yml down
```

## Test Structure

### Store Layer Tests

Located in `internal/store/`:
- `testhelper.go` - Test infrastructure and utilities
- `emailauth_test.go` - Email authentication tests
- `oauth_test.go` - OAuth authentication tests
- `user_test.go` - User management tests
- `account_test.go` - Account CRUD tests
- `subscription_test.go` - Subscription management tests

### API Integration Tests

Located in `tests/`:
- `helpers.go` - Common test utilities and HTTP helpers
- `health_test.go` - Health check endpoint tests
- `auth_test.go` - Authentication API tests (20+ tests)
- `campaign_test.go` - Campaign management API tests (35+ tests)
- `webhook_test.go` - Webhook management API tests (25+ tests)
- `README.md` - Detailed API testing documentation

### Test Infrastructure

The test suite uses:
- **PostgreSQL** database running in Docker (port 5432)
- **Kafka** broker for event streaming (port 9092)
- **Flyway** for automatic database migrations
- Real HTTP requests to test the full API stack

## Running Tests

### Available Test Commands

| Command | Description |
|---------|-------------|
| `make test` | Run all store tests with database |
| `make test-api` | Run API integration tests (requires server) |
| `make test-integration` | Run all integration tests (store + API) |
| `make test-all` | Run all tests (unit + integration) |
| `make test-coverage` | Run tests with coverage report |
| `make test-store` | Run store tests only |
| `make test-db-up` | Start test infrastructure |
| `make test-db-down` | Stop test infrastructure |

### Store Layer Tests

Run all store tests:
```bash
make test
```

Run specific store test:
```bash
make test-one TEST=TestStore_CreateUser
```

### API Integration Tests

Run all API tests (requires server running):
```bash
make test-api
```

Run specific API test:
```bash
go test -v -tags=integration ./tests -run TestAPI_Auth_EmailSignup
```

Run specific test file:
```bash
go test -v -tags=integration ./tests/auth_test.go ./tests/helpers.go
```

### With Coverage

Generate a coverage report:
```bash
make test-coverage
```

This creates `coverage.html` which you can open in your browser.

## Test Configuration

### Environment Variables

#### Database Configuration

- `TEST_DB_HOST` - Database host (default: localhost)
- `TEST_DB_PORT` - Database port (default: 5432)
- `TEST_DB_USER` - Database user (default: base_user)
- `TEST_DB_PASSWORD` - Database password (default: base_password)
- `TEST_DB_NAME` - Database name (default: base_db)
- `TEST_DB_TYPE` - Database type (default: postgres)

#### API Test Configuration

- `TEST_API_HOST` - API server host (default: localhost)
- `TEST_API_PORT` - API server port (default: 8080)

### Custom Configuration

Run tests with custom configuration:
```bash
# Custom API port
TEST_API_PORT=9000 make test-api

# Custom database
TEST_DB_HOST=custom-host TEST_DB_PORT=5433 make test
```

## Writing Tests

### Store Layer Tests

All store tests follow the table-driven pattern:

```go
func TestStore_SomeFunction(t *testing.T) {
    testDB := SetupTestDB(t, TestDBTypePostgres)
    defer testDB.Close()

    ctx := context.Background()

    tests := []struct {
        name     string
        setup    func(t *testing.T) // Setup test data
        wantErr  bool
        validate func(t *testing.T) // Validate results
    }{
        {
            name: "successful case",
            setup: func(t *testing.T) {
                // Create test data
            },
            wantErr: false,
            validate: func(t *testing.T) {
                // Verify results
            },
        },
        {
            name: "error case",
            setup: func(t *testing.T) {
                // Setup for error
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            testDB.Truncate(t) // Clean database
            tt.setup(t)

            // Run test
            result, err := testDB.Store.SomeFunction(ctx, params)

            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }

            if !tt.wantErr && tt.validate != nil {
                tt.validate(t)
            }
        })
    }
}
```

### Test Helpers

The `testhelper.go` provides utilities:

- `SetupTestDB(t, dbType)` - Create a test database instance
- `testDB.Truncate(t, tables...)` - Clear table data
- `testDB.GetDB()` - Get raw database connection
- `testDB.MustExec(t, query, args...)` - Execute SQL
- `testDB.WithContext()` - Get test context

### Store Helper Functions

Create helper functions for common test data:

```go
func createTestUser(t *testing.T, testDB *TestDB, firstName, lastName string) (User, error) {
    t.Helper()
    var user User
    query := `INSERT INTO users (first_name, last_name) VALUES ($1, $2) RETURNING id, first_name, last_name`
    err := testDB.GetDB().Get(&user, query, firstName, lastName)
    return user, err
}
```

### API Integration Tests

API tests use the `//go:build integration` tag and follow a similar table-driven pattern:

```go
//go:build integration
// +build integration

package tests

import (
    "net/http"
    "testing"
)

func TestAPI_YourFeature(t *testing.T) {
    token := createAuthenticatedUser(t)

    tests := []struct {
        name           string
        request        map[string]interface{}
        expectedStatus int
        validateFunc   func(t *testing.T, body []byte)
    }{
        {
            name: "success case",
            request: map[string]interface{}{
                "field": "value",
            },
            expectedStatus: http.StatusOK,
            validateFunc: func(t *testing.T, body []byte) {
                var response map[string]interface{}
                parseJSONResponse(t, body, &response)
                // Add assertions
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            resp, body := makeAuthenticatedRequest(t, http.MethodPost, "/api/path", tt.request, token)
            assertStatusCode(t, resp, tt.expectedStatus)

            if tt.validateFunc != nil {
                tt.validateFunc(t, body)
            }
        })
    }
}
```

### API Test Helper Functions

The `tests/helpers.go` provides utilities for API testing:

**HTTP Request Helpers:**
- `makeRequest(t, method, path, body, headers)` - Unauthenticated request
- `makeAuthenticatedRequest(t, method, path, body, token)` - Authenticated request
- `parseJSONResponse(t, body, &target)` - Parse JSON response
- `assertStatusCode(t, resp, expectedStatus)` - Assert status code

**Test Data Helpers:**
- `createAuthenticatedUser(t)` - Create user and return JWT token
- `generateTestEmail()` - Generate unique email address
- `generateTestCampaignSlug()` - Generate unique campaign slug

**Example Usage:**

```go
// Create authenticated user
token := createAuthenticatedUser(t)

// Make authenticated request
resp, body := makeAuthenticatedRequest(t, http.MethodGet, "/api/protected/user", nil, token)

// Assert response
assertStatusCode(t, resp, http.StatusOK)

var user map[string]interface{}
parseJSONResponse(t, body, &user)
```

For detailed API testing documentation, see [tests/README.md](tests/README.md).

## Best Practices

1. **Use `t.Helper()`** in helper functions to get better error reporting
2. **Truncate tables** between test cases to ensure isolation
3. **Test both success and error cases** for comprehensive coverage
4. **Use descriptive test names** that explain what's being tested
5. **Validate all important fields** in success cases
6. **Check error types** in error cases (e.g., `errors.Is(err, ErrNotFound)`)
7. **Use setup functions** to create test data instead of duplicating code
8. **Clean up resources** with `defer` to ensure cleanup even on test failure

## Troubleshooting

### Database Connection Issues

If tests fail to connect to the database:

1. Check if Docker is running:
```bash
docker ps
```

2. Check if the test database is running:
```bash
docker-compose -f docker-compose.test.yml ps
```

3. Check database logs:
```bash
docker-compose -f docker-compose.test.yml logs test-postgres
```

4. Restart the test database:
```bash
make test-db-down
make test-db-up
```

### Migration Failures

If migrations fail to run:

1. Check migration files exist:
```bash
ls -la migrations/
```

2. Verify migration SQL syntax
3. Check database logs for specific errors

### Port Conflicts

If port 5433 is already in use:

1. Change the port in `docker-compose.test.yml`
2. Update the `TEST_DB_PORT` environment variable
3. Restart the test database

### Slow Tests

If tests are running slowly:

1. Use `make test-quiet` instead of `make test` to reduce output
2. Run specific test files instead of all tests
3. Check if the database container has sufficient resources

## CI/CD Integration

### GitHub Actions

Example workflow:

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: postgres
        ports:
          - 5433:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Run tests
        env:
          TEST_DB_HOST: localhost
          TEST_DB_PORT: 5433
          TEST_DB_USER: postgres
          TEST_DB_PASSWORD: postgres
        run: go test ./internal/store/... -v -cover
```

## Coverage Goals

We aim for:
- **80%+ overall coverage** for the store layer
- **100% coverage** for critical CRUD operations
- **Error case coverage** for all database operations

Check current coverage:
```bash
make test-coverage
open coverage.html
```

## Additional Resources

- [Go Testing Package](https://pkg.go.dev/testing)
- [Table-Driven Tests in Go](https://go.dev/wiki/TableDrivenTests)
- [PostgreSQL Docker Image](https://hub.docker.com/_/postgres)
