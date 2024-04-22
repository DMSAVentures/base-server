package observability

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Field represents a key-value pair for observability.
type Field struct {
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

// Middleware to add observability fields to Gin context.
func Middleware(l *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create an initial context with some observability fields.
		ctx := context.Background()
		ctx = WithFields(ctx,
			Field{"request_id", c.Writer.Header().Get("X-Request-ID")},
			Field{"path", c.Request.URL.Path},
			Field{"method", c.Request.Method},
		)

		// Store the context in Gin context for later use.
		c.Request = c.Request.WithContext(ctx)

		// Process the request.
		start := time.Now()
		c.Next() // Proceed with request handling.
		latency := time.Since(start)

		// Additional logging after request processing.
		ctx = WithFields(ctx, Field{"latency_ns", latency.Nanoseconds()})
		l.Info(ctx, "Request processed")
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
