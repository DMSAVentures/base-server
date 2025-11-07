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
Define domain-specific errors as package-level variables:
```go
var (
    ErrEmailAlreadyExists = errors.New("email already exists")
    ErrUserNotFound       = errors.New("user not found")
)
```

### Error Handling Pattern
```go
user, err := h.authProcessor.GetUserByExternalID(ctx, parsedUserID)
if err != nil {
    h.logger.Error(ctx, "failed to get user by external id", err)
    if errors.Is(err, store.ErrNotFound) {
        context.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
        return
    }
    context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
    return
}
```

## HTTP Handling

### Handler Structure
```go
type Handler struct {
    processor SomeProcessor
    logger    *observability.Logger
}

func (h *Handler) HandleSomeAction(c *gin.Context) {
    ctx := c.Request.Context()
    
    // Input validation
    var req RequestStruct
    if err := c.ShouldBindJSON(&req); err != nil {
        h.logger.Error(ctx, "failed to bind request", err)
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
        return
    }
    
    // Business logic
    result, err := h.processor.ProcessAction(ctx, req)
    if err != nil {
        h.logger.Error(ctx, "failed to process", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    // Response
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

## Common Patterns to Follow

1. **Always use context**: Pass context through all function calls
2. **Structured errors**: Define errors as package-level variables
3. **Dependency injection**: Use constructor functions for initialization
4. **Graceful shutdown**: Handle OS signals properly
5. **Request tracing**: Use request IDs for debugging
6. **Middleware composition**: Apply cross-cutting concerns via middleware
7. **Resource cleanup**: Use defer for cleanup operations
8. **Configuration validation**: Check all required env vars at startup