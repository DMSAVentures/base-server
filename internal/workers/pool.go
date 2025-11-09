package workers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"base-server/internal/observability"
)

// ProcessingResult represents the result of processing an event.
type ProcessingResult struct {
	Event EventMessage
	Error error
}

// ResultCallback is called after each event is processed.
// The callback receives the event and any error that occurred.
type ResultCallback func(result ProcessingResult)

// WorkerPoolConfig holds configuration for the worker pool.
type WorkerPoolConfig struct {
	// NumWorkers is the number of concurrent workers to run.
	NumWorkers int

	// QueueSize is the size of the event queue buffer.
	// If the queue is full, Submit() will block.
	QueueSize int

	// DrainTimeout is the maximum time to wait for in-flight events
	// to complete during graceful shutdown.
	DrainTimeout time.Duration

	// OnResult is called after each event is processed (optional).
	// Used for tracking success/failure for offset commits.
	OnResult ResultCallback
}

// DefaultWorkerPoolConfig returns sensible defaults for a worker pool.
func DefaultWorkerPoolConfig() WorkerPoolConfig {
	return WorkerPoolConfig{
		NumWorkers:   10,
		QueueSize:    100,
		DrainTimeout: 30 * time.Second,
	}
}

// pool implements the WorkerPool interface.
type pool struct {
	config    WorkerPoolConfig
	processor EventProcessor
	logger    *observability.Logger

	// Event distribution
	eventChan chan EventMessage
	wg        sync.WaitGroup

	// Lifecycle management
	mu       sync.Mutex
	started  bool
	draining bool
	stopped  bool
	cancelFn context.CancelFunc
}

// NewWorkerPool creates a new worker pool for processing events.
func NewWorkerPool(
	config WorkerPoolConfig,
	processor EventProcessor,
	logger *observability.Logger,
) WorkerPool {
	if config.NumWorkers <= 0 {
		config.NumWorkers = DefaultWorkerPoolConfig().NumWorkers
	}
	if config.QueueSize <= 0 {
		config.QueueSize = DefaultWorkerPoolConfig().QueueSize
	}
	if config.DrainTimeout <= 0 {
		config.DrainTimeout = DefaultWorkerPoolConfig().DrainTimeout
	}

	return &pool{
		config:    config,
		processor: processor,
		logger:    logger,
		eventChan: make(chan EventMessage, config.QueueSize),
	}
}

// Start initializes the worker pool with N workers.
func (p *pool) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return fmt.Errorf("worker pool already started")
	}
	if p.stopped {
		return fmt.Errorf("worker pool already stopped")
	}

	// Create cancellable context for workers
	workerCtx, cancel := context.WithCancel(ctx)
	p.cancelFn = cancel
	p.started = true

	// Start worker goroutines
	for i := 0; i < p.config.NumWorkers; i++ {
		p.wg.Add(1)
		go p.worker(workerCtx, i)
	}

	p.logger.Info(ctx, fmt.Sprintf("Started %d workers for %s processor",
		p.config.NumWorkers, p.processor.Name()))

	return nil
}

// Submit adds an event to the worker pool for processing.
func (p *pool) Submit(ctx context.Context, event EventMessage) error {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return fmt.Errorf("worker pool not started")
	}
	if p.draining || p.stopped {
		p.mu.Unlock()
		return fmt.Errorf("worker pool is shutting down")
	}
	p.mu.Unlock()

	// Block until event can be queued or context cancelled
	select {
	case p.eventChan <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Drain stops accepting new events and waits for in-flight events to complete.
func (p *pool) Drain(ctx context.Context) error {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return fmt.Errorf("worker pool not started")
	}
	if p.draining {
		p.mu.Unlock()
		return fmt.Errorf("worker pool already draining")
	}
	p.draining = true
	p.mu.Unlock()

	p.logger.Info(ctx, fmt.Sprintf("Draining worker pool for %s processor, waiting for %d in-flight events",
		p.processor.Name(), len(p.eventChan)))

	// Close event channel to signal no more events will be submitted
	close(p.eventChan)

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	// Apply drain timeout
	drainCtx, cancel := context.WithTimeout(ctx, p.config.DrainTimeout)
	defer cancel()

	select {
	case <-done:
		p.logger.Info(ctx, fmt.Sprintf("Successfully drained worker pool for %s processor",
			p.processor.Name()))
		return nil
	case <-drainCtx.Done():
		p.logger.Warn(ctx, fmt.Sprintf("Drain timeout exceeded for %s processor, forcing shutdown",
			p.processor.Name()))
		p.Stop()
		return fmt.Errorf("drain timeout exceeded")
	}
}

// Stop immediately stops all workers.
func (p *pool) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		return
	}
	p.stopped = true

	if p.cancelFn != nil {
		p.cancelFn()
	}

	// Close channel if not already closed
	if !p.draining {
		close(p.eventChan)
	}
}

// worker is the main worker loop that processes events from the queue.
func (p *pool) worker(ctx context.Context, workerID int) {
	defer p.wg.Done()

	workerCtx := observability.WithFields(ctx,
		observability.Field{Key: "worker_id", Value: workerID},
		observability.Field{Key: "processor", Value: p.processor.Name()},
	)

	p.logger.Info(workerCtx, fmt.Sprintf("Worker %d started for %s processor",
		workerID, p.processor.Name()))

	for {
		select {
		case <-ctx.Done():
			p.logger.Info(workerCtx, fmt.Sprintf("Worker %d stopping: context cancelled",
				workerID))
			return

		case event, ok := <-p.eventChan:
			if !ok {
				p.logger.Info(workerCtx, fmt.Sprintf("Worker %d stopping: event channel closed",
					workerID))
				return
			}

			// Process the event
			eventCtx := observability.WithFields(workerCtx,
				observability.Field{Key: "event_id", Value: event.ID},
				observability.Field{Key: "event_type", Value: event.Type},
				observability.Field{Key: "account_id", Value: event.AccountID},
			)

			err := p.processor.Process(eventCtx, event)

			if err != nil {
				p.logger.Error(eventCtx, fmt.Sprintf("Worker %d failed to process event",
					workerID), err)
			} else {
				p.logger.Info(eventCtx, fmt.Sprintf("Worker %d successfully processed event",
					workerID))
			}

			// Notify result callback if configured
			if p.config.OnResult != nil {
				p.config.OnResult(ProcessingResult{
					Event: event,
					Error: err,
				})
			}
		}
	}
}
