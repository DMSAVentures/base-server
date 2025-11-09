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

// EventConsumer defines the interface for consuming events from Kafka
// and distributing them to a worker pool.
type EventConsumer interface {
	// Start begins consuming events from Kafka and processing them.
	// Blocks until context is cancelled or an unrecoverable error occurs.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the consumer, draining in-flight events.
	Stop()
}

// WorkerPool defines the interface for managing a pool of event processing workers.
type WorkerPool interface {
	// Start initializes the worker pool with N workers.
	// Each worker will process events by calling the EventProcessor.
	Start(ctx context.Context) error

	// Submit adds an event to the worker pool for processing.
	// Blocks if the event queue is full.
	Submit(ctx context.Context, event EventMessage) error

	// Drain stops accepting new events and waits for in-flight events to complete.
	// Returns after all workers have finished processing or context is cancelled.
	Drain(ctx context.Context) error

	// Stop immediately stops all workers.
	Stop()
}
