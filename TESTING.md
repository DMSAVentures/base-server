# Testing Guide

This document describes how to run unit tests for the base-server project, specifically for the store layer.

## Overview

The test suite uses:
- **PostgreSQL** (via Docker) for testing database operations
- **Table-driven tests** for comprehensive coverage
- **Real database migrations** to ensure schema compatibility
- **Isolated test databases** for each test run

## Prerequisites

- Go 1.23+
- Docker and Docker Compose
- Make (optional, for convenience commands)

## Quick Start

### Using Make (Recommended)

The easiest way to run tests is using the provided Makefile:

```bash
# Run all store tests
make test

# Run tests without verbose output
make test-quiet

# Run tests with coverage report
make test-coverage

# Run a specific test
make test-one TEST=TestStore_CreateUser
```

### Manual Setup

If you prefer to run tests manually:

1. **Start the test database:**
```bash
docker-compose -f docker-compose.test.yml up -d
```

2. **Set environment variables:**
```bash
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5433
export TEST_DB_USER=postgres
export TEST_DB_PASSWORD=postgres
export TEST_DB_TYPE=postgres
```

3. **Run tests:**
```bash
go test ./internal/store/... -v
```

4. **Stop the test database:**
```bash
docker-compose -f docker-compose.test.yml down -v
```

## Test Structure

### Test Files

- `internal/store/testhelper.go` - Test infrastructure and utilities
- `internal/store/emailauth_test.go` - Email authentication tests
- `internal/store/oauth_test.go` - OAuth authentication tests
- `internal/store/user_test.go` - User management tests
- `internal/store/account_test.go` - Account CRUD tests
- `internal/store/subscription_test.go` - Subscription management tests

### Test Database

The test suite uses a PostgreSQL database running in Docker on port 5433 (to avoid conflicts with development databases on port 5432).

Each test run:
1. Creates a new test database with a unique name
2. Runs all migrations to set up the schema
3. Executes the tests
4. Cleans up and drops the test database

## Running Tests

### All Tests

Run all store tests with verbose output:
```bash
make test
```

Or using go directly:
```bash
go test ./internal/store/... -v
```

### Specific Test File

Run tests from a specific file:
```bash
go test ./internal/store -run TestStore_CreateUser -v
```

### Specific Test Case

Run a specific test case:
```bash
make test-one TEST=TestStore_CreateAccount
```

Or:
```bash
go test ./internal/store -run TestStore_CreateAccount/create_account_with_all_fields -v
```

### With Coverage

Generate a coverage report:
```bash
make test-coverage
```

This creates `coverage.html` which you can open in your browser.

### Watch Mode

Run tests automatically when files change (requires [watchexec](https://github.com/watchexec/watchexec)):
```bash
make test-watch
```

## Test Configuration

### Environment Variables

The test suite respects the following environment variables:

- `TEST_DB_HOST` - Database host (default: localhost)
- `TEST_DB_PORT` - Database port (default: 5433)
- `TEST_DB_USER` - Database user (default: postgres)
- `TEST_DB_PASSWORD` - Database password (default: postgres)
- `TEST_DB_NAME` - Database name prefix (default: test_db)
- `TEST_DB_TYPE` - Database type (default: postgres)

### Custom Database

To use a different PostgreSQL instance:
```bash
export TEST_DB_HOST=custom-host
export TEST_DB_PORT=5432
export TEST_DB_USER=testuser
export TEST_DB_PASSWORD=testpass
go test ./internal/store/... -v
```

## Writing Tests

### Table-Driven Tests

All tests follow the table-driven pattern:

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

### Helper Functions

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
