package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"base-server/internal/observability"

	"github.com/google/uuid"
)

type AudioPipeline struct {
	id     string
	logger *observability.Logger

	// Source side (e.g., Twilio) - FIXED for pipeline lifetime
	sourceIn  <-chan []byte // Audio from source
	sourceOut chan []byte   // Audio to source

	// Sink side (e.g., Google AI, OpenAI) - SWAPPABLE
	sinkIn  chan []byte   // Audio to sink
	sinkOut <-chan []byte // Audio from sink
	sinkMu  sync.RWMutex  // Protects sink channels during swap

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics
	stats PipelineStats
	mu    sync.RWMutex

	// Configuration
	config PipelineConfig
}

type PipelineConfig struct {
	BufferSize           int           // Buffer size for internal channels
	MaxReconnectAttempts int           // Maximum reconnection attempts
	ReconnectDelay       time.Duration // Delay between reconnection attempts
	EnableMetrics        bool          // Enable metrics collection
}

type PipelineStats struct {
	BytesFromSource int64
	BytesToSource   int64
	BytesFromSink   int64
	BytesToSink     int64
	StartTime       time.Time
	EndTime         time.Time
	SinkSwaps       int // Number of times sink was swapped
}

func DefaultConfig() PipelineConfig {
	return PipelineConfig{
		BufferSize:           4096,
		MaxReconnectAttempts: 3,
		ReconnectDelay:       time.Second,
		EnableMetrics:        true,
	}
}

func NewAudioPipeline(sourceIn <-chan []byte, sourceOut chan []byte, logger *observability.Logger, config PipelineConfig) (*AudioPipeline, error) {
	if sourceIn == nil || sourceOut == nil {
		return nil, fmt.Errorf("source channels cannot be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &AudioPipeline{
		id:        uuid.New().String(),
		logger:    logger,
		sourceIn:  sourceIn,
		sourceOut: sourceOut,
		sinkIn:    make(chan []byte, config.BufferSize),
		ctx:       ctx,
		cancel:    cancel,
		config:    config,
		stats:     PipelineStats{StartTime: time.Now()},
	}, nil
}

func (p *AudioPipeline) ID() string {
	return p.id
}

// ConnectSink connects or reconnects the sink side of the pipeline
// This can be called multiple times to swap providers
func (p *AudioPipeline) ConnectSink(inbound chan []byte, outbound <-chan []byte) error {
	if inbound == nil || outbound == nil {
		return fmt.Errorf("sink channels cannot be nil")
	}

	p.sinkMu.Lock()
	defer p.sinkMu.Unlock()

	// If there was a previous sink, update swap count
	if p.sinkIn != nil {
		p.mu.Lock()
		p.stats.SinkSwaps++
		p.mu.Unlock()
		p.logger.Info(p.ctx, fmt.Sprintf("Swapping sink on pipeline %s (swap #%d)", p.id, p.stats.SinkSwaps))
	}

	p.sinkIn = inbound
	p.sinkOut = outbound
	return nil
}

func (p *AudioPipeline) Start(ctx context.Context) error {
	p.sinkMu.RLock()
	if p.sinkIn == nil || p.sinkOut == nil {
		p.sinkMu.RUnlock()
		return fmt.Errorf("sink not connected")
	}
	p.sinkMu.RUnlock()

	// Merge contexts
	pipelineCtx, cancel := context.WithCancel(ctx)
	p.ctx = pipelineCtx
	oldCancel := p.cancel
	p.cancel = func() {
		cancel()
		oldCancel()
	}

	p.logger.Info(ctx, fmt.Sprintf("Starting audio pipeline %s", p.id))

	// Start the bidirectional audio flow
	p.wg.Add(2)

	// Source -> Sink flow
	go p.forwardSourceToSink()

	// Sink -> Source flow
	go p.forwardSinkToSource()

	return nil
}

func (p *AudioPipeline) forwardSourceToSink() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Info(p.ctx, "Source->Sink flow stopped: context cancelled")
			return

		case audio, ok := <-p.sourceIn:
			if !ok {
				p.logger.Info(p.ctx, "Source input channel closed")
				// Close sink input to signal end of stream
				p.sinkMu.RLock()
				if p.sinkIn != nil {
					close(p.sinkIn)
				}
				p.sinkMu.RUnlock()
				return
			}

			// Update metrics
			if p.config.EnableMetrics {
				p.mu.Lock()
				p.stats.BytesFromSource += int64(len(audio))
				p.stats.BytesToSink += int64(len(audio))
				p.mu.Unlock()
			}

			// Forward to sink with non-blocking send
			p.sinkMu.RLock()
			sinkIn := p.sinkIn
			p.sinkMu.RUnlock()

			if sinkIn != nil {
				select {
				case sinkIn <- audio:
					// Successfully sent
				case <-time.After(100 * time.Millisecond):
					p.logger.Warn(p.ctx, "Sink input buffer full, dropping audio chunk")
				case <-p.ctx.Done():
					return
				}
			}
		}
	}
}

func (p *AudioPipeline) forwardSinkToSource() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Info(p.ctx, "Sink->Source flow stopped: context cancelled")
			return
		default:
			// Get current sink output channel
			p.sinkMu.RLock()
			sinkOut := p.sinkOut
			p.sinkMu.RUnlock()

			if sinkOut == nil {
				// No sink connected, wait a bit
				time.Sleep(100 * time.Millisecond)
				continue
			}

			select {
			case <-p.ctx.Done():
				return
			case audio, ok := <-sinkOut:
				if !ok {
					p.logger.Info(p.ctx, "Sink output channel closed")
					// Sink disconnected, but don't close source output
					// Pipeline continues and can accept a new sink
					p.sinkMu.Lock()
					p.sinkOut = nil
					p.sinkMu.Unlock()
					continue
				}

				// Update metrics
				if p.config.EnableMetrics {
					p.mu.Lock()
					p.stats.BytesFromSink += int64(len(audio))
					p.stats.BytesToSource += int64(len(audio))
					p.mu.Unlock()
				}

				// Forward to source with non-blocking send
				select {
				case p.sourceOut <- audio:
					// Successfully sent
				case <-time.After(100 * time.Millisecond):
					p.logger.Warn(p.ctx, "Source output buffer full, dropping audio chunk")
				case <-p.ctx.Done():
					return
				}
			}
		}
	}
}

func (p *AudioPipeline) Stop() {
	p.logger.Info(p.ctx, fmt.Sprintf("Stopping audio pipeline %s", p.id))

	// Cancel context to stop all goroutines
	p.cancel()

	// Wait for goroutines to finish
	p.wg.Wait()

	// Update end time
	p.mu.Lock()
	p.stats.EndTime = time.Now()
	p.mu.Unlock()

	// Close sink input if still open
	p.sinkMu.Lock()
	if p.sinkIn != nil {
		close(p.sinkIn)
	}
	p.sinkMu.Unlock()

	p.logger.Info(p.ctx, fmt.Sprintf("Audio pipeline %s stopped", p.id))
}

func (p *AudioPipeline) GetStats() PipelineStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := p.stats
	if stats.EndTime.IsZero() {
		stats.EndTime = time.Now()
	}
	return stats
}

// SwapSink swaps the sink while the pipeline is running
// This allows changing AI providers mid-call
func (p *AudioPipeline) SwapSink(inbound chan []byte, outbound <-chan []byte) error {
	p.logger.Info(p.ctx, fmt.Sprintf("Swapping sink on pipeline %s", p.id))
	return p.ConnectSink(inbound, outbound)
}
