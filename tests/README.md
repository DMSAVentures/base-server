# API Testing Documentation

This directory contains comprehensive API integration tests for the base-server project. The testing infrastructure is inspired by the backend-challenge-v3 project and follows Go testing best practices.

## Table of Contents

- [Overview](#overview)
- [Test Structure](#test-structure)
- [Running Tests](#running-tests)
- [Test Coverage](#test-coverage)
- [Writing New Tests](#writing-new-tests)
- [Troubleshooting](#troubleshooting)

## Overview

The test suite includes:

- **Health Check Tests**: Basic connectivity and health endpoint validation
- **Authentication Tests**: User signup, login, JWT token validation
- **Campaign Management Tests**: CRUD operations for campaigns
- **Webhook Management Tests**: Webhook creation, update, delivery tracking
- Additional coverage for waitlist, rewards, referrals, and analytics endpoints

### Testing Approach

- **Integration Tests**: Tests require a running server and database
- **Build Tags**: Tests use `//go:build integration` tag to separate from unit tests
- **Real HTTP Requests**: Tests make actual HTTP calls to the API
- **Database State**: Tests interact with a real PostgreSQL database

## Test Structure

```
tests/
├── README.md           # This file
├── helpers.go          # Common test utilities and helper functions
├── health_test.go      # Health check endpoint tests
├── auth_test.go        # Authentication endpoint tests
├── campaign_test.go    # Campaign management tests
└── webhook_test.go     # Webhook management tests
```

### Helper Functions

The `helpers.go` file provides:

- **makeRequest()**: Make HTTP requests without authentication
- **makeAuthenticatedRequest()**: Make HTTP requests with JWT token
- **parseJSONResponse()**: Parse JSON response bodies
- **assertStatusCode()**: Assert HTTP status codes
- **createAuthenticatedUser()**: Create a test user and return auth token
- **generateTestEmail()**: Generate unique test email addresses
- **generateTestCampaignSlug()**: Generate unique campaign slugs
- **setupTestStore()**: Connect to test database

## Running Tests

### Prerequisites

1. **Start the test infrastructure** (database, Kafka):
   ```bash
   make test-db-up
   ```

2. **Start the API server** on the test port:
   ```bash
   # In a separate terminal
   go run main.go
   # Or set custom port: SERVER_PORT=8080 go run main.go
   ```

### Running API Tests

```bash
# Run all API integration tests
make test-api

# Run specific test file
go test -v -tags=integration ./tests/auth_test.go ./tests/helpers.go

# Run specific test function
go test -v -tags=integration ./tests -run TestAPI_Auth_EmailSignup

# Run with verbose output
go test -v -tags=integration ./tests/...
```

### Running All Tests

```bash
# Run all tests (unit + integration)
make test-all

# Run only integration tests (store + API)
make test-integration

# Run with coverage
make test-coverage
```

### Stopping Test Infrastructure

```bash
make test-db-down
```

## Environment Variables

Configure the test environment using these variables:

### Database Configuration

```bash
TEST_DB_HOST=localhost        # Database host (default: localhost)
TEST_DB_PORT=5432             # Database port (default: 5432)
TEST_DB_USER=base_user        # Database user (default: base_user)
TEST_DB_PASSWORD=base_password # Database password (default: base_password)
TEST_DB_NAME=base_db          # Database name (default: base_db)
```

### API Configuration

```bash
TEST_API_HOST=localhost       # API server host (default: localhost)
TEST_API_PORT=8080           # API server port (default: 8080)
```

### Example: Custom Configuration

```bash
TEST_API_PORT=9000 make test-api
```

## Test Coverage

### Current Coverage

| Domain | Test File | Tests | Coverage |
|--------|-----------|-------|----------|
| Health | `health_test.go` | 1 | Health endpoint |
| Authentication | `auth_test.go` | 20+ | Signup, login, JWT validation, user info |
| Campaigns | `campaign_test.go` | 35+ | Create, list, get, update, delete, status changes |
| Webhooks | `webhook_test.go` | 25+ | Create, list, get, update, delete, deliveries, test |

### Test Categories

#### Authentication Tests (`auth_test.go`)

- ✅ Email signup with valid data
- ✅ Email signup validation (missing fields, invalid email, short password)
- ✅ Email login with valid credentials
- ✅ Email login failure cases (wrong password, non-existent user, invalid format)
- ✅ Get user info with valid/invalid tokens
- ✅ JWT authentication middleware

#### Campaign Tests (`campaign_test.go`)

- ✅ Create campaigns (waitlist, referral, contest)
- ✅ Create campaign validation (missing fields, invalid types)
- ✅ List campaigns with pagination and filters
- ✅ Get campaign by ID
- ✅ Update campaign fields
- ✅ Update campaign status (draft, active, paused, completed)
- ✅ Delete campaigns
- ✅ Campaign not found scenarios

#### Webhook Tests (`webhook_test.go`)

- ✅ Create webhooks with events
- ✅ Create webhook validation (missing URL, missing events, invalid URL)
- ✅ List webhooks with campaign filtering
- ✅ Get webhook by ID
- ✅ Update webhook URL, events, status, retry settings
- ✅ Delete webhooks
- ✅ Get webhook deliveries with pagination
- ✅ Test webhook functionality

## Writing New Tests

### Test File Template

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

                if response["field"] != "expected_value" {
                    t.Error("Unexpected response")
                }
            },
        },
        {
            name: "failure case",
            request: map[string]interface{}{
                "invalid": "data",
            },
            expectedStatus: http.StatusBadRequest,
            validateFunc: func(t *testing.T, body []byte) {
                var errResp map[string]interface{}
                parseJSONResponse(t, body, &errResp)

                if errResp["error"] == nil {
                    t.Error("Expected error message in response")
                }
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

### Best Practices

1. **Use table-driven tests** with subtests for multiple scenarios
2. **Create unique test data** using helper functions (generateTestEmail, etc.)
3. **Clean up test data** where necessary (though soft deletes help)
4. **Test both success and failure cases** for each endpoint
5. **Validate response structure** not just status codes
6. **Use meaningful test names** that describe the scenario
7. **Test edge cases** (empty strings, invalid UUIDs, unauthorized access)

### Testing Authenticated Endpoints

```go
// Create and authenticate a user
token := createAuthenticatedUser(t)

// Make authenticated requests
resp, body := makeAuthenticatedRequest(t, http.MethodGet, "/api/protected/resource", nil, token)
```

### Testing Unauthenticated Endpoints

```go
// Make requests without authentication
resp, body := makeRequest(t, http.MethodPost, "/api/auth/login/email", requestData, nil)
```

### Testing with Custom Headers

```go
headers := map[string]string{
    "X-Custom-Header": "value",
}
resp, body := makeRequest(t, http.MethodGet, "/api/endpoint", nil, headers)
```

## Test Patterns

### Pattern 1: CRUD Operation Tests

```go
func TestAPI_Resource_Create(t *testing.T) { /* create tests */ }
func TestAPI_Resource_List(t *testing.T) { /* list with filters/pagination */ }
func TestAPI_Resource_GetByID(t *testing.T) { /* get single resource */ }
func TestAPI_Resource_Update(t *testing.T) { /* update operations */ }
func TestAPI_Resource_Delete(t *testing.T) { /* delete operations */ }
```

### Pattern 2: Validation Tests

Test all binding validations:
- Required fields
- Field format (email, UUID, etc.)
- Field length constraints
- Enum values
- Cross-field validation

### Pattern 3: Authorization Tests

For protected endpoints:
- Success with valid token
- Failure without token
- Failure with invalid token
- Failure with expired token (if applicable)

## Troubleshooting

### Common Issues

#### 1. "Connection refused" errors

**Cause**: API server is not running

**Solution**:
```bash
# Start the server in another terminal
go run main.go
```

#### 2. "Database connection failed"

**Cause**: Test database is not running

**Solution**:
```bash
make test-db-up
```

#### 3. "Unauthorized" errors in tests

**Cause**: Token generation or authentication logic changed

**Solution**: Check `createAuthenticatedUser()` helper function

#### 4. Tests fail due to duplicate data

**Cause**: Previous test data exists in database

**Solution**:
- Use unique generators (generateTestEmail, generateTestCampaignSlug)
- Or restart database: `make test-db-down && make test-db-up`

#### 5. "Context deadline exceeded"

**Cause**: API server is slow or unresponsive

**Solution**: Increase timeout in helpers.go or check server logs

### Debug Mode

Run tests with verbose output to see detailed logs:

```bash
go test -v -tags=integration ./tests/... -count=1
```

### Running Specific Tests

```bash
# Run only auth tests
go test -v -tags=integration ./tests/auth_test.go ./tests/helpers.go

# Run specific test function
go test -v -tags=integration ./tests -run TestAPI_Campaign_Create

# Run specific subtest
go test -v -tags=integration ./tests -run TestAPI_Campaign_Create/create_waitlist_campaign
```

## CI/CD Integration

For CI/CD pipelines, ensure:

1. Docker and Docker Compose are available
2. Database services start before tests
3. API server starts and is healthy
4. Tests run with appropriate timeouts
5. Cleanup happens after tests

Example GitHub Actions workflow:

```yaml
- name: Start test infrastructure
  run: make test-db-up

- name: Wait for database
  run: sleep 10

- name: Run migrations
  run: make migrate

- name: Start API server
  run: go run main.go &

- name: Wait for API server
  run: sleep 5

- name: Run tests
  run: make test-api

- name: Cleanup
  run: make test-db-down
```

## Additional Resources

- [Go Testing Package](https://pkg.go.dev/testing)
- [Table-Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests)
- [Gin Testing](https://github.com/gin-gonic/gin#testing)
- [Backend Reference Implementation](../backend-challenge-v3-mhrjql/backend)

## Contributing

When adding new API tests:

1. Follow existing patterns and conventions
2. Add tests to the appropriate file or create a new test file
3. Update this README with new test coverage
4. Ensure tests pass before committing
5. Use descriptive test names and comments

---

**Last Updated**: 2025-11-07
