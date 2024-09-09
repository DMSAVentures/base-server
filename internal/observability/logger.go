package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Field represents a key-value pair for observability.
type Field struct {
	Key   string
	Value interface{}
}

// MetricField represents a key-value pair for logging metrics.
type MetricField struct {
	Key   string
	Value interface{}
}

type ObservabilityContextKey string

const observabilityKey ObservabilityContextKey = "observability_fields"

// WithFields adds a set of observability fields to the context.
func WithFields(ctx context.Context, fields ...Field) context.Context {
	existingFields := getObservabilityFields(ctx)
	existingFields = append(existingFields, fields...)
	return context.WithValue(ctx, observabilityKey, existingFields)
}

// Get observability fields from context.
func getObservabilityFields(ctx context.Context) []Field {
	if fields, ok := ctx.Value(observabilityKey).([]Field); ok {
		return fields
	}
	return nil
}

// Merge fields from context and additional metric fields, avoiding duplicates.
func mergeFields(ctx context.Context, fields []MetricField) []zapcore.Field {
	fieldMap := make(map[string]zapcore.Field)

	// Add context-based fields to the map (ensures uniqueness).
	if ctxFields, ok := ctx.Value("observability_fields").([]zapcore.Field); ok {
		for _, field := range ctxFields {
			fieldMap[field.Key] = field
		}
	}

	// Add additional metric fields to the map.
	for _, field := range fields {
		fieldMap[field.Key] = zap.Any(field.Key, field.Value)
	}

	// Convert map to a slice of zapcore.Field.
	mergedFields := make([]zapcore.Field, 0, len(fieldMap))
	for _, field := range fieldMap {
		mergedFields = append(mergedFields, field)
	}

	return mergedFields
}

// Middleware to add observability fields to Gin context.
func Middleware(l *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create an initial context with some observability fields.
		ctx := context.Background()
		if c.Request.Header.Get("X-Request-ID") == "" {
			c.Request.Header.Set("X-Request-ID", fmt.Sprintf("req-%s", uuid.New().String()))
		}
		ctx = WithFields(ctx,
			Field{"request_id", c.Writer.Header().Get("X-Request-ID")},
			Field{"path", c.Request.URL.Path},
			Field{"method", c.Request.Method},
		)

		// Store the context in Gin context for later use.
		c.Request = c.Request.WithContext(ctx)

		// Process the request.
		start := time.Now()
		// Capture any panic and handle it.
		defer func() {
			if r := recover(); r != nil {
				// Handle the panic and log the error.
				l.Error(c.Request.Context(), "Recovered from panic", fmt.Errorf("reason: %+v", r))
				c.AbortWithStatus(500) // Respond with a 500 error.
			}

			// Skip additional logging for health check endpoint.
			if c.Request.URL.Path == "/health" {
				return
			}
			// Calculate the request latency.
			latency := time.Since(start)
			// Additional logging after request processing.
			ctx = WithFields(ctx, Field{"latency_ns", latency.Nanoseconds()})
			l.Info(ctx, "Request processed")

			// Emit additional metrics or logging.
			// For example, you could log the request method, status, and latency.
			status := c.Writer.Status() // Get the response status code.
			l.Metrics(c.Request.Context(),
				MetricField{"method", c.Request.Method},
				MetricField{"path", c.Request.URL.Path},
				MetricField{"status", status},
				MetricField{"latency", latency},
				MetricField{"request_id", c.Writer.Header().Get("X-Request-ID")},
			)
		}()
		c.Next() // Proceed with request handling.
	}
}

// Logger represents a custom logger with Zap integration.
type Logger struct {
	zapLogger *zap.Logger
}

// NewLogger creates a new instance of custom logger.
func NewLogger() *Logger {
	zapLogger, _ := zap.NewProduction()
	zapLogger = zapLogger.WithOptions(zap.AddCallerSkip(1))
	zapLogger = zapLogger.WithOptions(zap.AddStacktrace(zapcore.ErrorLevel))
	return &Logger{zapLogger: zapLogger}
}

// Create a logger with fields from context.
func (l *Logger) loggerFromContext(ctx context.Context) *zap.Logger {
	fields := getObservabilityFields(ctx)
	zapFields := make([]zapcore.Field, len(fields))

	for i, f := range fields {
		zapFields[i] = zap.Any(f.Key, f.Value)
	}

	return l.zapLogger.With(zapFields...)
}

// Info logs an informational message with context-based fields.
func (l *Logger) Info(ctx context.Context, msg string) {
	l.loggerFromContext(ctx).Info(msg)
}

// InfoWithError logs an informational message with context and an error.
func (l *Logger) InfoWithError(ctx context.Context, msg string, err error) {
	l.loggerFromContext(ctx).Info(msg, zap.Error(err))
}

// Error logs an error message with context-based fields.
func (l *Logger) Error(ctx context.Context, msg string, err error) {
	l.loggerFromContext(ctx).Error(msg, zap.Error(err))
}

// Warn logs a warning message with context-based fields.
func (l *Logger) Warn(ctx context.Context, msg string) {
	l.loggerFromContext(ctx).Warn(msg)
}

// Debug logs a debug message with context-based fields.
func (l *Logger) Debug(ctx context.Context, msg string) {
	l.loggerFromContext(ctx).Debug(msg)
}

// Fatal logs a fatal message with context-based fields.
func (l *Logger) Fatal(ctx context.Context, msg string, err error) {
	l.loggerFromContext(ctx).Fatal(msg, zap.Error(err))
}

// Metrics logs metrics-related information using custom MetricField type.
func (l *Logger) Metrics(ctx context.Context, fields ...MetricField) {
	mergeFields := mergeFields(ctx, fields)
	// Log with correct stack depth and context.
	l.zapLogger.Info("Metrics", mergeFields...)
}
