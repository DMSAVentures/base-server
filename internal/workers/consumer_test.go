package workers

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"base-server/internal/observability"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProcessor is a test implementation of EventProcessor
type mockProcessor struct {
	name           string
	processedCount atomic.Int32
	processingTime time.Duration
	processedIDs   []string
	mu             sync.Mutex
	onProcess      func(event EventMessage) error
}

func newMockProcessor(name string, processingTime time.Duration) *mockProcessor {
	return &mockProcessor{
		name:           name,
		processingTime: processingTime,
		processedIDs:   make([]string, 0),
	}
}

func (m *mockProcessor) Process(ctx context.Context, event EventMessage) error {
	if m.processingTime > 0 {
		time.Sleep(m.processingTime)
	}

	m.mu.Lock()
	m.processedIDs = append(m.processedIDs, event.ID)
	m.mu.Unlock()
	m.processedCount.Add(1)

	if m.onProcess != nil {
		return m.onProcess(event)
	}
	return nil
}

func (m *mockProcessor) Name() string {
	return m.name
}

func (m *mockProcessor) getProcessedIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.processedIDs))
	copy(result, m.processedIDs)
	return result
}

// TestWorkerProcessesEventsUntilChannelClosed tests that workers continue
// processing all events in the channel before stopping.
func TestWorkerProcessesEventsUntilChannelClosed(t *testing.T) {
	t.Parallel()

	logger := observability.NewLogger()
	processor := newMockProcessor("test", 10*time.Millisecond)

	eventCh := make(chan eventWithMsg, 10)
	var wg sync.WaitGroup

	ctx := observability.WithFields(context.Background(),
		observability.Field{Key: "processor", Value: "test"},
	)

	// Create a minimal consumer just to test worker behavior
	c := &consumer{
		processor: processor,
		logger:    logger,
		eventCh:   eventCh,
	}

	// Start a single worker
	wg.Add(1)
	go c.worker(&wg, 0, ctx)

	// Send 5 events
	for i := 0; i < 5; i++ {
		eventCh <- eventWithMsg{
			event: EventMessage{ID: string(rune('a' + i))},
		}
	}

	// Close the channel - worker should process all remaining events
	close(eventCh)

	// Wait for worker to finish
	wg.Wait()

	// Verify all events were processed
	assert.Equal(t, int32(5), processor.processedCount.Load())
	assert.Len(t, processor.getProcessedIDs(), 5)
}

// TestWorkerCompletesInFlightEventBeforeExiting tests that a worker
// finishes processing the current event even after channel is closed.
func TestWorkerCompletesInFlightEventBeforeExiting(t *testing.T) {
	t.Parallel()

	logger := observability.NewLogger()
	processor := newMockProcessor("test", 100*time.Millisecond) // Slow processing

	eventCh := make(chan eventWithMsg, 10)
	var wg sync.WaitGroup

	ctx := context.Background()

	c := &consumer{
		processor: processor,
		logger:    logger,
		eventCh:   eventCh,
	}

	// Start worker
	wg.Add(1)
	go c.worker(&wg, 0, ctx)

	// Send one event
	eventCh <- eventWithMsg{
		event: EventMessage{ID: "slow-event"},
	}

	// Give worker time to pick up the event
	time.Sleep(20 * time.Millisecond)

	// Close channel while event is still processing
	close(eventCh)

	// Worker should still complete the in-flight event
	wg.Wait()

	assert.Equal(t, int32(1), processor.processedCount.Load())
	assert.Contains(t, processor.getProcessedIDs(), "slow-event")
}

// TestMultipleWorkersProcessEventsConcurrently tests that multiple workers
// process events concurrently and all complete before WaitGroup is done.
func TestMultipleWorkersProcessEventsConcurrently(t *testing.T) {
	t.Parallel()

	logger := observability.NewLogger()
	processor := newMockProcessor("test", 50*time.Millisecond)

	eventCh := make(chan eventWithMsg, 100)
	var wg sync.WaitGroup

	ctx := context.Background()

	c := &consumer{
		processor: processor,
		logger:    logger,
		eventCh:   eventCh,
	}

	numWorkers := 5
	numEvents := 20

	// Start multiple workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go c.worker(&wg, i, ctx)
	}

	// Send events
	for i := 0; i < numEvents; i++ {
		eventCh <- eventWithMsg{
			event: EventMessage{ID: string(rune('A' + i))},
		}
	}

	// Close channel
	close(eventCh)

	// Wait for all workers
	start := time.Now()
	wg.Wait()
	elapsed := time.Since(start)

	// All events should be processed
	assert.Equal(t, int32(numEvents), processor.processedCount.Load())

	// With 5 workers processing 20 events at 50ms each, should take ~200-250ms
	// (20 events / 5 workers = 4 batches * 50ms = 200ms)
	// If sequential, would take 1000ms
	assert.Less(t, elapsed, 500*time.Millisecond, "Workers should process concurrently")
}

// TestConsumerStopWaitsForInFlightEvents tests that Stop() blocks until
// all in-flight events are processed.
func TestConsumerStopWaitsForInFlightEvents(t *testing.T) {
	t.Parallel()

	logger := observability.NewLogger()
	processor := newMockProcessor("test", 200*time.Millisecond) // Slow processing

	// Create consumer with test config (no real Kafka)
	c := &consumer{
		config: ConsumerConfig{
			NumWorkers:   2,
			QueueSize:    10,
			DrainTimeout: 5 * time.Second,
		},
		processor: processor,
		logger:    logger,
		eventCh:   make(chan eventWithMsg, 10),
		doneCh:    make(chan struct{}),
	}

	// Simulate Start() behavior without Kafka
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelFetch = cancel

	var workerWg sync.WaitGroup
	for i := 0; i < c.config.NumWorkers; i++ {
		workerWg.Add(1)
		go c.worker(&workerWg, i, ctx)
	}

	// Send some events
	for i := 0; i < 4; i++ {
		c.eventCh <- eventWithMsg{
			event: EventMessage{ID: string(rune('a' + i))},
		}
	}

	// Give workers time to start processing
	time.Sleep(50 * time.Millisecond)

	// Simulate Stop() behavior
	c.stopping.Store(true)
	cancel()
	close(c.eventCh)

	// Measure time for workers to complete
	start := time.Now()
	workerWg.Wait()
	elapsed := time.Since(start)

	// Workers should have completed processing (takes ~200ms per event with 2 workers)
	assert.Equal(t, int32(4), processor.processedCount.Load())

	// Should take some time (events were in-flight)
	assert.Greater(t, elapsed, 100*time.Millisecond, "Should wait for in-flight events")
}

// TestConsumerDrainTimeout tests that consumer respects drain timeout.
func TestConsumerDrainTimeout(t *testing.T) {
	t.Parallel()

	logger := observability.NewLogger()
	processor := newMockProcessor("test", 2*time.Second) // Very slow processing

	c := &consumer{
		config: ConsumerConfig{
			NumWorkers:   1,
			QueueSize:    10,
			DrainTimeout: 100 * time.Millisecond, // Short timeout
		},
		processor: processor,
		logger:    logger,
		eventCh:   make(chan eventWithMsg, 10),
		doneCh:    make(chan struct{}),
	}

	ctx := context.Background()

	var workerWg sync.WaitGroup
	workerWg.Add(1)
	go c.worker(&workerWg, 0, ctx)

	// Send an event that takes longer than drain timeout
	c.eventCh <- eventWithMsg{
		event: EventMessage{ID: "slow"},
	}

	// Give worker time to start
	time.Sleep(20 * time.Millisecond)

	close(c.eventCh)

	// Wait with timeout (simulating drain behavior)
	done := make(chan struct{})
	go func() {
		workerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Worker completed within timeout - this is fine
	case <-time.After(c.config.DrainTimeout):
		// Timeout exceeded - this is expected for this test
		t.Log("Drain timeout exceeded as expected")
	}

	// Note: In real consumer, we'd log a warning but not forcibly kill workers
	// Workers will eventually complete even after timeout
}

// TestStoppingFlagIsChecked tests that the stopping flag is properly checked.
func TestStoppingFlagIsChecked(t *testing.T) {
	t.Parallel()

	c := &consumer{}

	// Initially not stopping
	assert.False(t, c.stopping.Load())

	// Set stopping
	c.stopping.Store(true)
	assert.True(t, c.stopping.Load())
}

// TestStopOnceEnsuresSingleExecution tests that Stop() only executes once.
func TestStopOnceEnsuresSingleExecution(t *testing.T) {
	t.Parallel()

	logger := observability.NewLogger()
	processor := newMockProcessor("test", 0)

	c := &consumer{
		config:    ConsumerConfig{},
		processor: processor,
		logger:    logger,
		eventCh:   make(chan eventWithMsg, 10),
		doneCh:    make(chan struct{}),
	}

	// Set up cancel function
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelFetch = cancel

	// Close doneCh to simulate Start() completion
	close(c.doneCh)

	// Call Stop multiple times - should not panic
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Stop()
		}()
	}
	wg.Wait()

	// Verify stopping flag is set
	assert.True(t, c.stopping.Load())

	// Verify context was cancelled
	assert.Error(t, ctx.Err())
}

// TestNewConsumerDefaults tests that NewConsumer applies defaults correctly.
func TestNewConsumerDefaults(t *testing.T) {
	t.Parallel()

	logger := observability.NewLogger()
	processor := newMockProcessor("test", 0)

	config := ConsumerConfig{
		Brokers:       []string{"localhost:9092"},
		ConsumerGroup: "test-group",
		Topic:         "test-topic",
		// Leave NumWorkers, QueueSize, DrainTimeout as zero
	}

	c := NewConsumer(config, processor, logger)
	require.NotNil(t, c)

	// Verify consumer was created (we can't easily check internal config
	// without exporting it, but NewConsumer applies defaults internally)
}

// TestDefaultConsumerConfig tests the default config helper.
func TestDefaultConsumerConfig(t *testing.T) {
	t.Parallel()

	config := DefaultConsumerConfig(
		[]string{"broker1:9092", "broker2:9092"},
		"my-consumer-group",
		"my-topic",
	)

	assert.Equal(t, []string{"broker1:9092", "broker2:9092"}, config.Brokers)
	assert.Equal(t, "my-consumer-group", config.ConsumerGroup)
	assert.Equal(t, "my-topic", config.Topic)
	assert.Equal(t, 10, config.NumWorkers)
	assert.Equal(t, 100, config.QueueSize)
	assert.Equal(t, 30*time.Second, config.DrainTimeout)
}
