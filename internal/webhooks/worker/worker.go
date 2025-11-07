package worker

import (
	"base-server/internal/observability"
	"base-server/internal/webhooks/service"
	"context"
	"time"
)

// WebhookWorker handles background webhook retry processing
type WebhookWorker struct {
	webhookService *service.WebhookService
	logger         *observability.Logger
	stopChan       chan bool
	interval       time.Duration
}

// New creates a new WebhookWorker
func New(webhookService *service.WebhookService, logger *observability.Logger, interval time.Duration) *WebhookWorker {
	return &WebhookWorker{
		webhookService: webhookService,
		logger:         logger,
		stopChan:       make(chan bool),
		interval:       interval,
	}
}

// Start begins the background worker
func (w *WebhookWorker) Start(ctx context.Context) {
	w.logger.Info(ctx, "Starting webhook retry worker")

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Process immediately on start
	w.processRetries(ctx)

	for {
		select {
		case <-ticker.C:
			w.processRetries(ctx)
		case <-w.stopChan:
			w.logger.Info(ctx, "Stopping webhook retry worker")
			return
		case <-ctx.Done():
			w.logger.Info(ctx, "Context cancelled, stopping webhook retry worker")
			return
		}
	}
}

// Stop stops the background worker
func (w *WebhookWorker) Stop() {
	close(w.stopChan)
}

// processRetries processes pending webhook retries
func (w *WebhookWorker) processRetries(ctx context.Context) {
	w.logger.Info(ctx, "Processing webhook retries")

	// Process up to 100 pending deliveries at a time
	err := w.webhookService.RetryFailedDeliveries(ctx, 100)
	if err != nil {
		w.logger.Error(ctx, "failed to process webhook retries", err)
	}

	w.logger.Info(ctx, "Finished processing webhook retries")
}
