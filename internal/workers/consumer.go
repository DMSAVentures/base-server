package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"base-server/internal/observability"

	kafkago "github.com/segmentio/kafka-go"
)

// ConsumerConfig holds configuration for the Kafka event consumer.
type ConsumerConfig struct {
	// Brokers is the list of Kafka broker addresses.
	Brokers []string

	// ConsumerGroup is the Kafka consumer group ID.
	ConsumerGroup string

	// Topic is the Kafka topic to consume from.
	Topic string

	// NumWorkers is the number of concurrent workers.
	NumWorkers int

	// QueueSize is the buffer size for the event channel.
	QueueSize int

	// DrainTimeout is the maximum time to wait for in-flight events during shutdown.
	DrainTimeout time.Duration
}

// DefaultConsumerConfig returns sensible defaults for a consumer.
func DefaultConsumerConfig(brokers []string, consumerGroup, topic string) ConsumerConfig {
	return ConsumerConfig{
		Brokers:       brokers,
		ConsumerGroup: consumerGroup,
		Topic:         topic,
		NumWorkers:    10,
		QueueSize:     100,
		DrainTimeout:  30 * time.Second,
	}
}

// eventWithMsg pairs an event with its Kafka message for offset tracking.
type eventWithMsg struct {
	event EventMessage
	msg   kafkago.Message
}

// consumer implements the EventConsumer interface.
type consumer struct {
	config    ConsumerConfig
	reader    *kafkago.Reader
	processor EventProcessor
	logger    *observability.Logger

	// Event channel for worker distribution
	eventCh chan eventWithMsg

	// Lifecycle management
	cancelFetch context.CancelFunc // cancels the fetch context
	doneCh      chan struct{}      // closed when Start() returns
	stopping    atomic.Bool
	stopOnce    sync.Once
}

// NewConsumer creates a new Kafka event consumer.
func NewConsumer(
	config ConsumerConfig,
	processor EventProcessor,
	logger *observability.Logger,
) EventConsumer {
	// Apply defaults
	if config.NumWorkers <= 0 {
		config.NumWorkers = 10
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 100
	}
	if config.DrainTimeout <= 0 {
		config.DrainTimeout = 30 * time.Second
	}

	c := &consumer{
		config:    config,
		processor: processor,
		logger:    logger,
		eventCh:   make(chan eventWithMsg, config.QueueSize),
		doneCh:    make(chan struct{}),
	}

	// Create Kafka reader
	c.reader = kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        config.Brokers,
		Topic:          config.Topic,
		GroupID:        config.ConsumerGroup,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		StartOffset:    kafkago.FirstOffset,
		CommitInterval: 0, // Manual commit
	})

	ctx := observability.WithFields(context.Background(),
		observability.Field{Key: "processor", Value: processor.Name()},
		observability.Field{Key: "consumer_group", Value: config.ConsumerGroup},
		observability.Field{Key: "topic", Value: config.Topic},
		observability.Field{Key: "num_workers", Value: config.NumWorkers},
	)
	logger.Info(ctx, fmt.Sprintf("Initialized consumer for %s processor", processor.Name()))

	return c
}

// Start begins consuming events and blocks until Stop is called.
func (c *consumer) Start(ctx context.Context) error {
	defer close(c.doneCh)

	// Create cancellable context with logging fields
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelFetch = cancel
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "consumer_group", Value: c.config.ConsumerGroup},
		observability.Field{Key: "topic", Value: c.config.Topic},
		observability.Field{Key: "processor", Value: c.processor.Name()},
	)

	c.logger.Info(ctx, fmt.Sprintf("Starting consumer for %s with %d workers",
		c.processor.Name(), c.config.NumWorkers))

	// Start workers - they process until eventCh is closed
	var workerWg sync.WaitGroup
	for i := 0; i < c.config.NumWorkers; i++ {
		workerWg.Add(1)
		go c.worker(&workerWg, i, ctx)
	}

	// Fetch loop - runs until Stop() cancels the context
	c.fetchLoop(ctx)

	// Shutdown: close channel and wait for workers to drain
	close(c.eventCh)

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		workerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info(ctx, "All workers finished processing")
	case <-time.After(c.config.DrainTimeout):
		c.logger.Warn(ctx, "Drain timeout - some events may not have completed")
	}

	// Close Kafka reader
	if err := c.reader.Close(); err != nil {
		c.logger.Error(ctx, "Failed to close Kafka reader", err)
	}

	c.logger.Info(ctx, fmt.Sprintf("Consumer stopped for %s", c.processor.Name()))
	return nil
}

// fetchLoop fetches messages from Kafka until context is cancelled.
func (c *consumer) fetchLoop(ctx context.Context) {
	for {
		// Check if stopping
		if c.stopping.Load() {
			return
		}

		// Fetch message (blocks until message available or context cancelled)
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if c.stopping.Load() || ctx.Err() != nil {
				return // Clean shutdown
			}
			c.logger.Error(ctx, "Failed to fetch message from Kafka", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// Parse event
		var event EventMessage
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			c.logger.Error(ctx, "Failed to unmarshal event, skipping", err)
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}

		// Send to workers (blocks if queue full)
		select {
		case c.eventCh <- eventWithMsg{event: event, msg: msg}:
		case <-ctx.Done():
			return
		}
	}
}

// worker processes events from the channel until it's closed.
func (c *consumer) worker(wg *sync.WaitGroup, id int, ctx context.Context) {
	defer wg.Done()

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "worker_id", Value: id},
	)

	c.logger.Info(ctx, fmt.Sprintf("Worker %d started for %s processor", id, c.processor.Name()))

	for e := range c.eventCh {
		eventCtx := observability.WithFields(ctx,
			observability.Field{Key: "event_id", Value: e.event.ID},
			observability.Field{Key: "event_type", Value: e.event.Type},
		)

		// Process the event (no context cancellation - always completes)
		err := c.processor.Process(eventCtx, e.event)

		if err != nil {
			c.logger.Error(eventCtx, "Failed to process event", err)
			// Don't commit - will be redelivered on restart
		} else if c.reader != nil {
			// Commit offset on success
			if commitErr := c.reader.CommitMessages(context.Background(), e.msg); commitErr != nil {
				c.logger.Error(eventCtx, "Failed to commit offset", commitErr)
			}
		}
	}

	c.logger.Info(ctx, fmt.Sprintf("Worker %d stopped", id))
}

// Stop gracefully shuts down the consumer.
// It signals the fetch loop to stop, waits for in-flight events to complete,
// and returns only after full shutdown.
func (c *consumer) Stop() {
	c.stopOnce.Do(func() {
		logCtx := observability.WithFields(context.Background(),
			observability.Field{Key: "processor", Value: c.processor.Name()},
		)
		c.logger.Info(logCtx, fmt.Sprintf("Stopping consumer for %s", c.processor.Name()))

		// Signal stopping
		c.stopping.Store(true)

		// Cancel fetch context to unblock FetchMessage
		if c.cancelFetch != nil {
			c.cancelFetch()
		}

		// Wait for Start() to complete (which waits for workers)
		<-c.doneCh
	})
}
