# Go Base Server - Developer Guidelines

## Project Structure

### Directory Organization
```
base-server/
├── internal/               # Private application code
│   ├── ai-capabilities/    # AI and LLM integrations
│   │   ├── handler/       # HTTP handlers for AI endpoints
│   │   └── processor/     # Business logic for AI operations
│   ├── api/               # API setup and route registration
│   ├── auth/              # Authentication & authorization
│   │   ├── handler/       # HTTP handlers for auth endpoints
│   │   └── processor/     # Business logic for auth operations
│   ├── clients/           # External service clients
│   │   ├── googleoauth/   # Google OAuth client
│   │   ├── mail/          # Email service client (Resend)
│   │   └── openai/        # OpenAI API client
│   ├── email/             # Email service abstraction
│   ├── money/             # Billing and payment domain
│   │   ├── billing/       # Billing operations
│   │   │   ├── handler/   # HTTP handlers for billing
│   │   │   └── processor/ # Business logic for billing
│   │   ├── products/      # Product management service
│   │   └── subscriptions/ # Subscription management service
│   ├── apierrors/         # Centralized API error handling
│   ├── observability/     # Logging and monitoring
│   └── store/             # Database layer and models
├── migrations/            # SQL migration files (Flyway format)
├── main.go               # Application entry point
└── go.mod                # Go module definition
```

### Package Patterns
- **Handler**: HTTP request handling, input validation, response formatting
- **Processor**: Business logic, orchestration, external service calls
- **Service**: Domain-specific business operations
- **Store**: Database operations and models
- **Client**: External API integrations
- **APIErrors**: Centralized API error handling, sanitization, and logging

## Code Conventions

### Naming Conventions
- **Variables**: camelCase (e.g., `userID`, `stripeCustomerID`)
- **Functions/Methods**: 
  - Exported: PascalCase (e.g., `HandleEmailLogin`, `CreateUser`)
  - Private: camelCase (e.g., `validateToken`, `hashPassword`)
- **Structs**: PascalCase (e.g., `AuthProcessor`, `EmailSignupRequest`)
- **Interfaces**: PascalCase with descriptive names (e.g., `Store`, `BillingProcessor`)
- **Constants**: PascalCase for exported, camelCase for private
- **Error Variables**: `Err` prefix (e.g., `ErrEmailAlreadyExists`, `ErrUserNotFound`)

### Import Organization
```go
import (
    // Standard library
    "context"
    "errors"
    "fmt"
    
    // Internal packages (with aliases for clarity)
    aiHandler "base-server/internal/ai-capabilities/handler"
    "base-server/internal/auth/processor"
    
    // Third-party packages
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "go.uber.org/zap"
)
```

## Architecture Patterns

### Dependency Injection
All components use constructor-based dependency injection:
```go
func New(store store.Store, authConfig AuthConfig, logger *observability.Logger) AuthProcessor {
    return AuthProcessor{
        store:      store,
        authConfig: authConfig,
        logger:     logger,
    }
}
```

### Layered Architecture
1. **HTTP Layer** (handlers): Request/response handling
2. **Business Layer** (processors/services): Core business logic
3. **Data Layer** (store): Database operations

### Context Usage
Always pass context through the call chain for:
- Request tracing (request_id)
- Structured logging fields
- Cancellation propagation

```go
ctx = observability.WithFields(ctx, observability.Field{Key: "user_id", Value: userID})
```

## Error Handling

### Error Definition
Define domain-specific errors as package-level variables in processor/service packages:
```go
var (
    ErrEmailAlreadyExists = errors.New("email already exists")
    ErrUserNotFound       = errors.New("user not found")
    ErrCampaignNotFound   = errors.New("campaign not found")
)
```

### Centralized API Error Handling
**CRITICAL:** All API handlers MUST use the `internal/apierrors` package for error responses. This ensures:
- Internal details (SQL errors, database schema, stack traces) never leak to clients
- Consistent error response format across all endpoints
- Two-tier logging: detailed errors in processor, correlation info in API layer

### Processor Layer Error Handling
Processors should log errors with full context before returning them:
```go
func (p *Processor) GetUserByExternalID(ctx context.Context, userID uuid.UUID) (User, error) {
    // Enrich context with operation-specific fields
    ctx = observability.WithFields(ctx,
        observability.Field{Key: "user_id", Value: userID},
        observability.Field{Key: "operation", Value: "get_user"},
    )

    user, err := p.store.GetUserByExternalID(ctx, userID)
    if err != nil {
        // Log detailed error with full context (user_id, operation, etc.)
        p.logger.Error(ctx, "failed to get user by external id", err)
        return User{}, err
    }
    return user, nil
}
```

### Handler Layer Error Handling
Handlers use `apierrors` for all error responses:

**For processor/business logic errors:**
```go
import "base-server/internal/apierrors"

func (h *Handler) HandleGetUser(c *gin.Context) {
    ctx := c.Request.Context()
    userID := parseUserID(c) // your parsing logic

    user, err := h.processor.GetUserByExternalID(ctx, userID)
    if err != nil {
        // Processor already logged detailed error with full context
        // apierrors maps the error and logs correlation info (request_id, status_code, error_code)
        apierrors.RespondWithError(c, err)
        return
    }

    c.JSON(http.StatusOK, user)
}
```

**For validation errors:**
```go
func (h *Handler) HandleCreateUser(c *gin.Context) {
    var req CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        // Handles validation errors with structured field-level details
        apierrors.RespondWithValidationError(c, err)
        return
    }

    user, err := h.processor.CreateUser(c.Request.Context(), req)
    if err != nil {
        apierrors.RespondWithError(c, err)
        return
    }

    c.JSON(http.StatusCreated, user)
}
```

### Two-Tier Logging Strategy
The error handling system uses a two-tier logging approach to balance debugging capability with log efficiency:

**Tier 1 - Processor Layer (Error Level):**
- Logs detailed errors with full enriched context
- Includes domain-specific fields (user_id, campaign_id, account_id, etc.)
- Contains root cause information
- Used for debugging and root cause analysis

**Tier 2 - API Layer (Info Level):**
- Logs minimal correlation information
- Includes request_id, status_code, error_code, error_message
- Used to correlate API responses with processor logs
- No duplicate error logging

This approach ensures you can trace from an API response back to the detailed processor logs using the request_id.

### Error Response Format
All API errors return this consistent JSON structure:
```json
{
  "error": "User-friendly error message",
  "code": "ERROR_CODE"
}
```

Error codes use `UPPER_SNAKE_CASE` format (e.g., `USER_NOT_FOUND`, `EMAIL_EXISTS`, `INVALID_INPUT`).

### Adding New Error Mappings
When adding new processor errors, update `internal/apierrors/mapper.go`:
```go
func MapError(err error) *APIError {
    switch {
    case errors.Is(err, yourProcessor.ErrYourNewError):
        return NotFound(CodeYourError, "User-friendly message")
    // ... other mappings
    default:
        return InternalError(err) // Sanitizes unknown errors
    }
}
```

Define the error code constant in `internal/apierrors/errors.go`:
```go
const (
    // ... existing codes
    CodeYourError = "YOUR_ERROR"
)
```

## HTTP Handling

### Handler Structure
```go
import (
    "base-server/internal/apierrors"
    "base-server/internal/observability"
    "github.com/gin-gonic/gin"
)

type Handler struct {
    processor SomeProcessor
    logger    *observability.Logger
}

func (h *Handler) HandleSomeAction(c *gin.Context) {
    ctx := c.Request.Context()

    // Input validation - use apierrors for validation errors
    var req RequestStruct
    if err := c.ShouldBindJSON(&req); err != nil {
        apierrors.RespondWithValidationError(c, err)
        return
    }

    // Business logic - processor logs detailed errors
    result, err := h.processor.ProcessAction(ctx, req)
    if err != nil {
        // Use apierrors for all business logic errors
        // Processor already logged detailed error with full context
        apierrors.RespondWithError(c, err)
        return
    }

    // Success response
    c.JSON(http.StatusOK, result)
}
```

### Request Validation
Use Gin's binding tags for validation:
```go
type EmailSignupRequest struct {
    FirstName string `json:"first_name" binding:"required"`
    LastName  string `json:"last_name" binding:"required"`
    Email     string `json:"email" binding:"required,email"`
    Password  string `json:"password" binding:"required,min=8"`
}
```

### Routing Pattern
```go
func (a *API) RegisterRoutes() {
    apiGroup := a.router.Group("/api")
    {
        authGroup := apiGroup.Group("/auth")
        authGroup.POST("/login/email", a.authHandler.HandleEmailLogin)
        
        protectedGroup := apiGroup.Group("/protected", a.authHandler.HandleJWTMiddleware)
        {
            protectedGroup.GET("/user", a.authHandler.GetUserInfo)
        }
    }
}
```

## Database Operations

### Store Pattern
```go
type Store struct {
    db     *sqlx.DB
    logger *observability.Logger
}

func (s Store) GetUserByExternalID(ctx context.Context, externalID uuid.UUID) (User, error) {
    var user User
    query := `SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL`
    err := s.db.GetContext(ctx, &user, query, externalID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return User{}, ErrNotFound
        }
        return User{}, err
    }
    return user, nil
}
```

### Update/Delete Operations Pattern
**IMPORTANT:** All UPDATE and DELETE operations MUST check rows affected and return `ErrNotFound` if no rows were modified.

```go
func (s *Store) UpdateSomething(ctx context.Context, id uuid.UUID, value string) error {
    result, err := s.db.ExecContext(ctx, sqlUpdateSomething, value, id)
    if err != nil {
        s.logger.Error(ctx, "failed to update something", err)
        return fmt.Errorf("failed to update something: %w", err)
    }

    rowsAffected, err := result.RowsAffected()
    if err != nil {
        s.logger.Error(ctx, "failed to get rows affected", err)
        return fmt.Errorf("failed to get rows affected: %w", err)
    }

    if rowsAffected == 0 {
        return ErrNotFound
    }

    return nil
}

func (s *Store) DeleteSomething(ctx context.Context, id uuid.UUID) error {
    result, err := s.db.ExecContext(ctx, sqlDeleteSomething, id)
    if err != nil {
        s.logger.Error(ctx, "failed to delete something", err)
        return fmt.Errorf("failed to delete something: %w", err)
    }

    rowsAffected, err := result.RowsAffected()
    if err != nil {
        s.logger.Error(ctx, "failed to get rows affected", err)
        return fmt.Errorf("failed to get rows affected: %w", err)
    }

    if rowsAffected == 0 {
        return ErrNotFound
    }

    return nil
}
```

**Exception:** Operations that are idempotent by design (e.g., incrementing counters, adding to sets) may skip the rows affected check if the operation can safely succeed on non-existent records.

### Migration Format
Use Flyway-compatible SQL migrations:
- Filename: `V{version}__{description}.sql`
- Example: `V0001__init.sql`
- Always use UUID with `uuid-ossp` extension
- Include `created_at`, `updated_at`, `deleted_at` for soft deletes

## Logging & Observability

### Structured Logging
Use the custom Logger with context-based fields:
```go
logger := observability.NewLogger()
ctx = observability.WithFields(ctx, 
    observability.Field{Key: "user_id", Value: userID},
    observability.Field{Key: "operation", Value: "signup"},
)
logger.Info(ctx, "User signed up successfully")
logger.Error(ctx, "Failed to create user", err)
```

### Middleware
Apply observability middleware for automatic request tracking:
```go
r.Use(observability.Middleware(logger))
```

## External Services

### Client Pattern
```go
type Client struct {
    apiKey string
    logger *observability.Logger
}

func NewClient(apiKey string, logger *observability.Logger) *Client {
    return &Client{
        apiKey: apiKey,
        logger: logger,
    }
}
```

### Service Integration
- **Stripe**: Payment processing and subscriptions
- **Resend**: Email sending
- **Google OAuth**: Authentication
- **OpenAI/Gemini**: AI capabilities
- **Twilio**: Voice and messaging
- **Kafka**: Event streaming for webhook delivery (AWS MSK or local)

## Configuration

### Environment Variables
Required environment variables (loaded from `env.local` in development):
```bash
# Database
DB_HOST=
DB_USERNAME=
DB_PASSWORD=
DB_NAME=

# Authentication
JWT_SECRET=
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_REDIRECT_URI=

# Services
STRIPE_SECRET_KEY=
STRIPE_WEBHOOK_SECRET=
RESEND_API_KEY=
OPENAI_API_KEY=
GOOGLE_AI_API_KEY=

# Kafka (Event Streaming)
KAFKA_BROKERS=                         # Comma-separated list (e.g., localhost:9092 or AWS MSK brokers)
KAFKA_TOPIC=webhook-events             # Optional, defaults to 'webhook-events'
KAFKA_CONSUMER_GROUP=webhook-consumers # Optional, defaults to 'webhook-consumers'

# Application
WEBAPP_URI=
SERVER_PORT=
DEFAULT_EMAIL_SENDER_ADDRESS=
```

### Environment Check
```go
if os.Getenv("GO_ENV") != "production" {
    err := godotenv.Load("env.local")
    // Development-specific configuration
}
```

## Build & Deployment

### Build Commands
```bash
# Local build
go build -o server main.go

# Run locally
go run main.go

# Docker build
docker build -t base-server .

# Run with Docker Compose (for services)
docker-compose -f docker-compose.services.yml up
```

### Database Migrations
```bash
# Build migration container
docker build -t flyway-migrate -f dbmigrator.dockerfile .

# Run migrations
docker run --platform linux/amd64 --rm \
  -e DB_HOST=$DB_HOST \
  -e DB_USERNAME=$DB_USERNAME \
  -e DB_PASSWORD=$DB_PASSWORD \
  flyway-migrate
```

### Docker Configuration
- Multi-stage build for minimal image size
- Alpine Linux base for production
- Health check endpoint at `/health`

## Security Patterns

### Authentication
- JWT tokens for session management
- bcrypt for password hashing
- Cookie-based token storage with HttpOnly flag

### Input Validation
- Always validate and sanitize input
- Use Gin's binding tags
- Return generic error messages to clients

### Context Security
```go
// Set secure cookie in production
if os.Getenv("GO_ENV") == "production" {
    c.SetCookie("token", token, 86400, "/", domain, true, true) // Secure, HttpOnly
}
```

## WebSocket Support

### WebSocket API
Separate WebSocket server running on `SERVER_PORT + 1`:
```go
websocketServer := &http.Server{
    Addr:         fmt.Sprintf(":%d", parsedServerPort+1),
    Handler:      x,
    ReadTimeout:  0,  // No timeout for WebSocket
    WriteTimeout: 0,
}
```

## Testing Requirements

**CRITICAL:** All code changes MUST include corresponding tests. Always verify tests pass before considering work complete.

### Test Levels

| Level | Location | Purpose | Command |
|-------|----------|---------|---------|
| **Store Tests** | `internal/store/*_test.go` | Database operations | `make test-store` |
| **Unit Tests** | `internal/*/processor/*_test.go` | Business logic with mocks | `make test-unit` |
| **Integration Tests** | `tests/*_test.go` | Full API endpoint testing | `make test-integration` |

### When to Add Tests

1. **New Store Methods**: Add tests in `internal/store/*_test.go`
   - Test success cases, error cases, edge cases
   - Use valid enum values (check migrations for valid values)

2. **New Processor Methods**: Add tests in `internal/*/processor/*_test.go`
   - Use gomock for mocking store dependencies
   - Regenerate mocks if interface changes: `go generate ./...`

3. **New API Endpoints**: Add tests in `tests/*_test.go`
   - Integration tests use real HTTP requests
   - Each test must use `t.Parallel()` for parallel execution
   - Create unique test data (UUIDs) to avoid conflicts

### Running Tests

```bash
# Start required services (database, kafka)
make services-up

# Run all test levels
make test-unit         # No DB needed - uses mocks
make test-store        # Requires DB (services-up)
make test-integration  # Requires running server

# Run specific test
go test -run TestFunctionName ./path/to/package/...

# Run with verbose output
go test -v ./internal/store/...
```

### Test Patterns

**Store Test Example:**
```go
func TestStore_CreateUser(t *testing.T) {
    t.Parallel()
    testDB := SetupTestDB(t, TestDBTypePostgres)

    user, err := testDB.Store.CreateUser(ctx, params)
    require.NoError(t, err)
    assert.Equal(t, expected, user.Email)
}
```

**Processor Test with Mocks:**
```go
func TestProcessor_GetUser(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockStore := NewMockStore(ctrl)
    mockStore.EXPECT().GetUser(gomock.Any(), userID).Return(user, nil)

    processor := New(mockStore, logger)
    result, err := processor.GetUser(ctx, userID)

    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

**Integration Test Example:**
```go
func TestAPI_CreateCampaign(t *testing.T) {
    t.Parallel()
    token := createAuthenticatedTestUser(t)

    resp, body := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", req, token)
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
}
```

### Before Completing Any Task

1. **Run relevant tests** to verify changes work:
   ```bash
   make test-unit        # Always run
   make test-store       # If store code changed
   make test-integration # If API behavior changed
   ```

2. **Fix any failing tests** before considering work complete

3. **Update mocks** if interfaces changed:
   ```bash
   go generate ./...
   ```

## Common Patterns to Follow

1. **Always use context**: Pass context through all function calls
2. **Centralized error handling**: Always use `apierrors.RespondWithError()` and `apierrors.RespondWithValidationError()` in handlers
3. **Two-tier logging**: Processors log detailed errors, API layer logs correlation info
4. **Structured errors**: Define errors as package-level variables
5. **Dependency injection**: Use constructor functions for initialization
6. **Graceful shutdown**: Handle OS signals properly
7. **Request tracing**: Use request IDs for debugging
8. **Middleware composition**: Apply cross-cutting concerns via middleware
9. **Resource cleanup**: Use defer for cleanup operations
10. **Configuration validation**: Check all required env vars at startup
11. **Always add tests**: Every code change must include corresponding tests
12. **Verify tests pass**: Run `make test-unit`, `make test-store`, `make test-integration` before completing work