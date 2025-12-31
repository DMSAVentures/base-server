package blast

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"base-server/internal/email"
	"base-server/internal/observability"
	"base-server/internal/store"
	"base-server/internal/webhooks/events"
	"base-server/internal/workers"

	"github.com/google/uuid"
)

// BlastStore defines the database operations required by BlastEventProcessor
type BlastStore interface {
	GetEmailBlastByID(ctx context.Context, blastID uuid.UUID) (store.EmailBlast, error)
	GetSegmentByID(ctx context.Context, segmentID uuid.UUID) (store.Segment, error)
	GetBlastEmailTemplateByID(ctx context.Context, templateID uuid.UUID) (store.BlastEmailTemplate, error)
	GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (store.Campaign, error)
	UpdateEmailBlastStatus(ctx context.Context, blastID uuid.UUID, status string, errorMessage *string) (store.EmailBlast, error)
	UpdateEmailBlastTotalRecipients(ctx context.Context, blastID uuid.UUID, totalRecipients int) error
	UpdateEmailBlastProgressWithSent(ctx context.Context, blastID uuid.UUID, sentCount int, currentBatch int) error
	CreateBlastRecipientsFromMultipleSegments(ctx context.Context, blastID uuid.UUID, segmentIDs []uuid.UUID, batchSize int) (int, error)
	GetBlastRecipientsByBatch(ctx context.Context, blastID uuid.UUID, batchNumber int) ([]store.BlastRecipient, error)
	UpdateBlastRecipientStatus(ctx context.Context, recipientID uuid.UUID, status string, emailLogID *uuid.UUID, errorMessage *string) error
	CountBlastRecipientsByStatus(ctx context.Context, blastID uuid.UUID, status string) (int, error)
}

// BlastEventProcessor implements the EventProcessor interface for blast events.
// It handles the async processing of email blasts: creating recipients, sending batches, etc.
type BlastEventProcessor struct {
	store           BlastStore
	emailService    *email.EmailService
	eventDispatcher *events.EventDispatcher
	logger          *observability.Logger
}

// NewBlastEventProcessor creates a new blast event processor.
func NewBlastEventProcessor(
	store BlastStore,
	emailService *email.EmailService,
	eventDispatcher *events.EventDispatcher,
	logger *observability.Logger,
) workers.EventProcessor {
	return &BlastEventProcessor{
		store:           store,
		emailService:    emailService,
		eventDispatcher: eventDispatcher,
		logger:          logger,
	}
}

// Name returns the processor name for logging and metrics.
func (p *BlastEventProcessor) Name() string {
	return "blast"
}

// Process handles a single blast event from Kafka.
func (p *BlastEventProcessor) Process(ctx context.Context, event workers.EventMessage) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "event_id", Value: event.ID},
		observability.Field{Key: "event_type", Value: event.Type},
		observability.Field{Key: "account_id", Value: event.AccountID},
	)

	p.logger.Info(ctx, fmt.Sprintf("Processing blast event %s", event.Type))

	switch event.Type {
	case events.EventBlastStarted:
		return p.handleBlastStarted(ctx, event)
	case events.EventBlastBatchSend:
		return p.handleBlastBatch(ctx, event)
	case events.EventBlastCompleted:
		return p.handleBlastCompleted(ctx, event)
	default:
		// Ignore events we don't handle
		p.logger.Info(ctx, fmt.Sprintf("Ignoring unhandled event type %s", event.Type))
		return nil
	}
}

// handleBlastStarted initializes the blast by:
// 1. Creating blast recipients from all segments (with deduplication)
// 2. Updating blast status to "sending"
// 3. Dispatching first batch event
func (p *BlastEventProcessor) handleBlastStarted(ctx context.Context, event workers.EventMessage) error {
	// Parse event data
	blastID, accountID, err := p.parseBlastEventData(event)
	if err != nil {
		return err
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "blast_id", Value: blastID},
	)

	// Get blast
	blast, err := p.store.GetEmailBlastByID(ctx, blastID)
	if err != nil {
		return fmt.Errorf("failed to get blast: %w", err)
	}

	// Verify blast is in processing status
	if blast.Status != string(store.EmailBlastStatusProcessing) {
		p.logger.Info(ctx, fmt.Sprintf("Blast is not in processing status, skipping (status: %s)", blast.Status))
		return nil
	}

	// Create blast recipients in bulk
	batchSize := blast.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	// Create recipients from all segments with deduplication
	totalRecipients, err := p.store.CreateBlastRecipientsFromMultipleSegments(ctx, blastID, blast.SegmentIDs, batchSize)
	if err != nil {
		return p.failBlast(ctx, blastID, fmt.Errorf("failed to create blast recipients: %w", err))
	}

	if totalRecipients == 0 {
		// No recipients - complete the blast immediately
		_, err = p.store.UpdateEmailBlastStatus(ctx, blastID, string(store.EmailBlastStatusCompleted), nil)
		if err != nil {
			return fmt.Errorf("failed to update blast status: %w", err)
		}
		p.logger.Info(ctx, "Blast completed with no recipients")
		return nil
	}

	// Update total recipients count
	err = p.store.UpdateEmailBlastTotalRecipients(ctx, blastID, totalRecipients)
	if err != nil {
		return p.failBlast(ctx, blastID, fmt.Errorf("failed to update total recipients: %w", err))
	}

	// Update status to sending
	_, err = p.store.UpdateEmailBlastStatus(ctx, blastID, string(store.EmailBlastStatusSending), nil)
	if err != nil {
		return p.failBlast(ctx, blastID, fmt.Errorf("failed to update blast status: %w", err))
	}

	// Dispatch first batch
	err = p.eventDispatcher.DispatchBlastBatchSend(ctx, accountID, blastID, 1)
	if err != nil {
		return p.failBlast(ctx, blastID, fmt.Errorf("failed to dispatch first batch: %w", err))
	}

	p.logger.Info(ctx, fmt.Sprintf("Blast started with %d recipients, dispatched batch 1", totalRecipients))
	return nil
}

// handleBlastBatch processes a batch of recipients:
// 1. Get recipients for the batch
// 2. Send emails to each recipient
// 3. Update recipient statuses
// 4. Either dispatch next batch or complete blast
func (p *BlastEventProcessor) handleBlastBatch(ctx context.Context, event workers.EventMessage) error {
	blastID, accountID, err := p.parseBlastEventData(event)
	if err != nil {
		return err
	}

	// Parse batch number
	batchNumber := 1
	if bn, ok := event.Data["batch_number"].(float64); ok {
		batchNumber = int(bn)
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "blast_id", Value: blastID},
		observability.Field{Key: "batch_number", Value: batchNumber},
	)

	// Get blast
	blast, err := p.store.GetEmailBlastByID(ctx, blastID)
	if err != nil {
		return fmt.Errorf("failed to get blast: %w", err)
	}

	// Check if blast is still in sending status (could be paused/cancelled)
	if blast.Status != string(store.EmailBlastStatusSending) {
		p.logger.Info(ctx, fmt.Sprintf("Blast is not in sending status, skipping batch (status: %s)", blast.Status))
		return nil
	}

	// Get template
	template, err := p.store.GetBlastEmailTemplateByID(ctx, blast.BlastTemplateID)
	if err != nil {
		return p.failBlast(ctx, blastID, fmt.Errorf("failed to get template: %w", err))
	}

	// Get recipients for this batch
	recipients, err := p.store.GetBlastRecipientsByBatch(ctx, blastID, batchNumber)
	if err != nil {
		return p.failBlast(ctx, blastID, fmt.Errorf("failed to get batch recipients: %w", err))
	}

	if len(recipients) == 0 {
		// No more recipients - check if blast should complete
		return p.checkBlastCompletion(ctx, blastID, accountID)
	}

	// Process each recipient
	sentCount := 0
	for _, recipient := range recipients {
		// Check if blast is still sending (in case it was paused mid-batch)
		if batchNumber > 1 && sentCount%10 == 0 {
			blast, err = p.store.GetEmailBlastByID(ctx, blastID)
			if err != nil || blast.Status != string(store.EmailBlastStatusSending) {
				p.logger.Info(ctx, "Blast is no longer sending, stopping batch processing")
				return nil
			}
		}

		err = p.sendBlastEmail(ctx, recipient, blast, template)
		if err != nil {
			// Log error but continue with other recipients
			p.logger.Error(ctx, fmt.Sprintf("Failed to send email to %s", recipient.Email), err)
			errMsg := err.Error()
			_ = p.store.UpdateBlastRecipientStatus(ctx, recipient.ID, string(store.BlastRecipientStatusFailed), nil, &errMsg)
		} else {
			_ = p.store.UpdateBlastRecipientStatus(ctx, recipient.ID, string(store.BlastRecipientStatusSent), nil, nil)
			sentCount++
		}
	}

	// Update blast progress
	err = p.store.UpdateEmailBlastProgressWithSent(ctx, blastID, blast.SentCount+sentCount, batchNumber)
	if err != nil {
		p.logger.Error(ctx, "Failed to update blast progress", err)
	}

	p.logger.Info(ctx, fmt.Sprintf("Batch %d completed: sent %d emails", batchNumber, sentCount))

	// Check if there are more batches
	nextBatchRecipients, err := p.store.GetBlastRecipientsByBatch(ctx, blastID, batchNumber+1)
	if err != nil {
		return p.failBlast(ctx, blastID, fmt.Errorf("failed to check next batch: %w", err))
	}

	if len(nextBatchRecipients) > 0 {
		// Dispatch next batch
		err = p.eventDispatcher.DispatchBlastBatchSend(ctx, accountID, blastID, batchNumber+1)
		if err != nil {
			return p.failBlast(ctx, blastID, fmt.Errorf("failed to dispatch next batch: %w", err))
		}
	} else {
		// All batches complete
		return p.checkBlastCompletion(ctx, blastID, accountID)
	}

	return nil
}

// handleBlastCompleted marks the blast as completed
func (p *BlastEventProcessor) handleBlastCompleted(ctx context.Context, event workers.EventMessage) error {
	blastID, _, err := p.parseBlastEventData(event)
	if err != nil {
		return err
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "blast_id", Value: blastID},
	)

	// Update blast status to completed
	now := time.Now()
	blast, err := p.store.GetEmailBlastByID(ctx, blastID)
	if err != nil {
		return fmt.Errorf("failed to get blast: %w", err)
	}

	if blast.Status == string(store.EmailBlastStatusSending) {
		_, err = p.store.UpdateEmailBlastStatus(ctx, blastID, string(store.EmailBlastStatusCompleted), nil)
		if err != nil {
			return fmt.Errorf("failed to update blast status: %w", err)
		}
		p.logger.Info(ctx, fmt.Sprintf("Blast completed at %v", now))
	}

	return nil
}

// sendBlastEmail sends a single email for the blast
func (p *BlastEventProcessor) sendBlastEmail(ctx context.Context, recipient store.BlastRecipient, blast store.EmailBlast, template store.BlastEmailTemplate) error {
	// Prepare template data
	data := email.TemplateData{
		Email: recipient.Email,
		// TODO: Add more template variables like first_name, position, etc.
		// This would require fetching the WaitlistUser data
	}

	// Render and send email
	err := p.emailService.SendCustomTemplateEmail(ctx, recipient.Email, blast.Subject, template.HTMLBody, data)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// failBlast marks a blast as failed with an error message
func (p *BlastEventProcessor) failBlast(ctx context.Context, blastID uuid.UUID, err error) error {
	errMsg := err.Error()
	_, updateErr := p.store.UpdateEmailBlastStatus(ctx, blastID, string(store.EmailBlastStatusFailed), &errMsg)
	if updateErr != nil {
		p.logger.Error(ctx, "Failed to update blast status to failed", updateErr)
	}
	return err
}

// checkBlastCompletion checks if all recipients have been processed and completes the blast
func (p *BlastEventProcessor) checkBlastCompletion(ctx context.Context, blastID, accountID uuid.UUID) error {
	// Check if there are any pending recipients
	pendingCount, err := p.store.CountBlastRecipientsByStatus(ctx, blastID, string(store.BlastRecipientStatusPending))
	if err != nil {
		return fmt.Errorf("failed to count pending recipients: %w", err)
	}

	if pendingCount == 0 {
		// All done - dispatch completion event
		err = p.eventDispatcher.DispatchBlastCompleted(ctx, accountID, blastID)
		if err != nil {
			return fmt.Errorf("failed to dispatch completion event: %w", err)
		}
	}

	return nil
}

// parseBlastEventData extracts blast_id and account_id from event data
func (p *BlastEventProcessor) parseBlastEventData(event workers.EventMessage) (blastID, accountID uuid.UUID, err error) {
	// Parse account ID
	accountID, err = uuid.Parse(event.AccountID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid account_id: %w", err)
	}

	// Parse event data
	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("failed to marshal event data: %w", err)
	}

	var eventData struct {
		BlastID string `json:"blast_id"`
	}
	if err := json.Unmarshal(dataBytes, &eventData); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	blastID, err = uuid.Parse(eventData.BlastID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid blast_id: %w", err)
	}

	return blastID, accountID, nil
}
