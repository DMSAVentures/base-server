package workers

import (
	"context"

	kafka "base-server/internal/clients/kafka"
)

// EventMessage is an alias for the Kafka event message type.
// This allows worker packages to reference EventMessage without importing kafka directly.
type EventMessage = kafka.EventMessage

// EventProcessor defines the interface for processing events from Kafka.
// Implementations should be idempotent as events may be redelivered on failure.
type EventProcessor interface {
	// Process handles a single event from Kafka.
	// Returns an error if processing fails, which will prevent offset commit
	// and cause the event to be redelivered.
	Process(ctx context.Context, event EventMessage) error

	// Name returns the processor name for logging and metrics.
	Name() string
}

// EventConsumer defines the interface for consuming events from Kafka.
type EventConsumer interface {
	// Start begins consuming events from Kafka and processing them.
	// Blocks until Stop is called.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the consumer.
	// Waits for in-flight events to complete before returning.
	Stop()
}
