package blast

import (
	"context"
	"fmt"
	"time"

	"base-server/internal/observability"
	"base-server/internal/store"
	"base-server/internal/webhooks/events"

	"github.com/google/uuid"
)

// SchedulerStore defines the database operations required by BlastScheduler
type SchedulerStore interface {
	GetScheduledBlasts(ctx context.Context, beforeTime time.Time) ([]store.EmailBlast, error)
	UpdateEmailBlastStatus(ctx context.Context, blastID uuid.UUID, status string, errorMessage *string) (store.EmailBlast, error)
	GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (store.Campaign, error)
}

// BlastScheduler periodically checks for scheduled blasts and triggers them
type BlastScheduler struct {
	store           SchedulerStore
	eventDispatcher *events.EventDispatcher
	logger          *observability.Logger
	checkInterval   time.Duration
	stopChan        chan struct{}
}

// NewBlastScheduler creates a new blast scheduler
func NewBlastScheduler(
	store SchedulerStore,
	eventDispatcher *events.EventDispatcher,
	logger *observability.Logger,
	checkInterval time.Duration,
) *BlastScheduler {
	if checkInterval <= 0 {
		checkInterval = 30 * time.Second
	}

	return &BlastScheduler{
		store:           store,
		eventDispatcher: eventDispatcher,
		logger:          logger,
		checkInterval:   checkInterval,
		stopChan:        make(chan struct{}),
	}
}

// Start begins the scheduler loop
func (s *BlastScheduler) Start(ctx context.Context) {
	s.logger.Info(ctx, fmt.Sprintf("Starting blast scheduler with %v interval", s.checkInterval))

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	// Run immediately on start
	s.checkScheduledBlasts(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info(ctx, "Blast scheduler stopping: context cancelled")
			return
		case <-s.stopChan:
			s.logger.Info(ctx, "Blast scheduler stopping: stop signal received")
			return
		case <-ticker.C:
			s.checkScheduledBlasts(ctx)
		}
	}
}

// Stop signals the scheduler to stop
func (s *BlastScheduler) Stop() {
	close(s.stopChan)
}

// checkScheduledBlasts checks for blasts that are scheduled and ready to send
func (s *BlastScheduler) checkScheduledBlasts(ctx context.Context) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "operation", Value: "check_scheduled_blasts"},
	)

	// Get blasts scheduled for now or earlier
	blasts, err := s.store.GetScheduledBlasts(ctx, time.Now())
	if err != nil {
		s.logger.Error(ctx, "Failed to get scheduled blasts", err)
		return
	}

	if len(blasts) == 0 {
		return
	}

	s.logger.Info(ctx, fmt.Sprintf("Found %d scheduled blasts ready to send", len(blasts)))

	for _, blast := range blasts {
		blastCtx := observability.WithFields(ctx,
			observability.Field{Key: "blast_id", Value: blast.ID},
			observability.Field{Key: "campaign_id", Value: blast.CampaignID},
		)

		// Update status to processing
		_, err := s.store.UpdateEmailBlastStatus(blastCtx, blast.ID, string(store.EmailBlastStatusProcessing), nil)
		if err != nil {
			s.logger.Error(blastCtx, "Failed to update blast status to processing", err)
			continue
		}

		// Get campaign to find account ID
		campaign, err := s.store.GetCampaignByID(blastCtx, blast.CampaignID)
		if err != nil {
			s.logger.Error(blastCtx, "Failed to get campaign for blast", err)
			errMsg := fmt.Sprintf("failed to get campaign: %v", err)
			_, _ = s.store.UpdateEmailBlastStatus(blastCtx, blast.ID, string(store.EmailBlastStatusFailed), &errMsg)
			continue
		}

		// Dispatch blast.started event
		err = s.eventDispatcher.DispatchBlastStarted(blastCtx, campaign.AccountID, blast.CampaignID, blast.ID)
		if err != nil {
			s.logger.Error(blastCtx, "Failed to dispatch blast started event", err)
			errMsg := fmt.Sprintf("failed to dispatch blast started event: %v", err)
			_, _ = s.store.UpdateEmailBlastStatus(blastCtx, blast.ID, string(store.EmailBlastStatusFailed), &errMsg)
			continue
		}

		s.logger.Info(blastCtx, "Triggered scheduled blast")
	}
}
