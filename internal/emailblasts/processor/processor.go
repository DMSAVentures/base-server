package processor

//go:generate go run go.uber.org/mock/mockgen@latest -source=processor.go -destination=mocks_test.go -package=processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// EmailBlastStore defines the database operations required by EmailBlastProcessor
type EmailBlastStore interface {
	GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (store.Campaign, error)
	GetSegmentByID(ctx context.Context, segmentID uuid.UUID) (store.Segment, error)
	GetEmailTemplateByID(ctx context.Context, templateID uuid.UUID) (store.EmailTemplate, error)
	CreateEmailBlast(ctx context.Context, params store.CreateEmailBlastParams) (store.EmailBlast, error)
	GetEmailBlastByID(ctx context.Context, blastID uuid.UUID) (store.EmailBlast, error)
	GetEmailBlastsByCampaign(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]store.EmailBlast, error)
	CountEmailBlastsByCampaign(ctx context.Context, campaignID uuid.UUID) (int, error)
	UpdateEmailBlast(ctx context.Context, blastID uuid.UUID, params store.UpdateEmailBlastParams) (store.EmailBlast, error)
	DeleteEmailBlast(ctx context.Context, blastID uuid.UUID) error
	UpdateEmailBlastStatus(ctx context.Context, blastID uuid.UUID, status string, errorMessage *string) (store.EmailBlast, error)
	UpdateEmailBlastTotalRecipients(ctx context.Context, blastID uuid.UUID, totalRecipients int) error
	ScheduleBlast(ctx context.Context, blastID uuid.UUID, scheduledAt time.Time) (store.EmailBlast, error)
	GetBlastRecipientsByBlast(ctx context.Context, blastID uuid.UUID, limit, offset int) ([]store.BlastRecipient, error)
	CountBlastRecipientsByBlast(ctx context.Context, blastID uuid.UUID) (int, error)
	GetBlastRecipientStats(ctx context.Context, blastID uuid.UUID) (store.BlastRecipientStats, error)
	CountUsersMatchingCriteria(ctx context.Context, campaignID uuid.UUID, criteria store.SegmentFilterCriteria) (int, error)
	GetUsersForBlast(ctx context.Context, campaignID uuid.UUID, criteria store.SegmentFilterCriteria) ([]store.WaitlistUser, error)
	CreateBlastRecipientsBulk(ctx context.Context, blastID uuid.UUID, users []store.WaitlistUser, batchSize int) error
}

var (
	ErrBlastNotFound       = errors.New("email blast not found")
	ErrSegmentNotFound     = errors.New("segment not found")
	ErrTemplateNotFound    = errors.New("email template not found")
	ErrCampaignNotFound    = errors.New("campaign not found")
	ErrUnauthorized        = errors.New("unauthorized access to email blast")
	ErrBlastCannotModify   = errors.New("blast cannot be modified in current status")
	ErrBlastCannotDelete   = errors.New("blast cannot be deleted in current status")
	ErrBlastCannotStart    = errors.New("blast cannot be started in current status")
	ErrBlastCannotPause    = errors.New("blast cannot be paused in current status")
	ErrBlastCannotResume   = errors.New("blast cannot be resumed in current status")
	ErrBlastCannotCancel   = errors.New("blast cannot be cancelled in current status")
	ErrInvalidScheduleTime = errors.New("scheduled time must be in the future")
	ErrNoRecipients        = errors.New("segment has no matching users")
)

type EmailBlastProcessor struct {
	store  EmailBlastStore
	logger *observability.Logger
}

func New(store EmailBlastStore, logger *observability.Logger) EmailBlastProcessor {
	return EmailBlastProcessor{
		store:  store,
		logger: logger,
	}
}

// CreateEmailBlastRequest represents a request to create an email blast
type CreateEmailBlastRequest struct {
	Name                  string
	SegmentID             uuid.UUID
	TemplateID            uuid.UUID
	Subject               string
	ScheduledAt           *time.Time
	BatchSize             int
	SendThrottlePerSecond *int
}

// CreateEmailBlast creates a new email blast for a campaign
func (p *EmailBlastProcessor) CreateEmailBlast(ctx context.Context, accountID, campaignID uuid.UUID, userID *uuid.UUID, req CreateEmailBlastRequest) (store.EmailBlast, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailBlast{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return store.EmailBlast{}, err
	}

	if campaign.AccountID != accountID {
		return store.EmailBlast{}, ErrUnauthorized
	}

	// Verify segment exists and belongs to campaign
	segment, err := p.store.GetSegmentByID(ctx, req.SegmentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailBlast{}, ErrSegmentNotFound
		}
		p.logger.Error(ctx, "failed to get segment", err)
		return store.EmailBlast{}, err
	}

	if segment.CampaignID != campaignID {
		return store.EmailBlast{}, ErrUnauthorized
	}

	// Verify template exists and belongs to campaign
	template, err := p.store.GetEmailTemplateByID(ctx, req.TemplateID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailBlast{}, ErrTemplateNotFound
		}
		p.logger.Error(ctx, "failed to get template", err)
		return store.EmailBlast{}, err
	}

	if template.CampaignID != campaignID {
		return store.EmailBlast{}, ErrUnauthorized
	}

	// Validate scheduled time if provided
	if req.ScheduledAt != nil && req.ScheduledAt.Before(time.Now()) {
		return store.EmailBlast{}, ErrInvalidScheduleTime
	}

	// Default batch size
	batchSize := req.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	params := store.CreateEmailBlastParams{
		CampaignID:            campaignID,
		SegmentID:             req.SegmentID,
		TemplateID:            req.TemplateID,
		Name:                  req.Name,
		Subject:               req.Subject,
		ScheduledAt:           req.ScheduledAt,
		BatchSize:             batchSize,
		SendThrottlePerSecond: req.SendThrottlePerSecond,
		CreatedBy:             userID,
	}

	blast, err := p.store.CreateEmailBlast(ctx, params)
	if err != nil {
		p.logger.Error(ctx, "failed to create email blast", err)
		return store.EmailBlast{}, err
	}

	p.logger.Info(ctx, "email blast created successfully")
	return blast, nil
}

// GetEmailBlast retrieves an email blast by ID
func (p *EmailBlastProcessor) GetEmailBlast(ctx context.Context, accountID, campaignID, blastID uuid.UUID) (store.EmailBlast, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "blast_id", Value: blastID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailBlast{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return store.EmailBlast{}, err
	}

	if campaign.AccountID != accountID {
		return store.EmailBlast{}, ErrUnauthorized
	}

	blast, err := p.store.GetEmailBlastByID(ctx, blastID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailBlast{}, ErrBlastNotFound
		}
		p.logger.Error(ctx, "failed to get email blast", err)
		return store.EmailBlast{}, err
	}

	// Verify blast belongs to the campaign
	if blast.CampaignID != campaignID {
		return store.EmailBlast{}, ErrUnauthorized
	}

	return blast, nil
}

// ListEmailBlastsRequest represents a request to list email blasts
type ListEmailBlastsRequest struct {
	Page  int
	Limit int
}

// ListEmailBlastsResponse represents the response for listing email blasts
type ListEmailBlastsResponse struct {
	Blasts     []store.EmailBlast `json:"blasts"`
	Total      int                `json:"total"`
	Page       int                `json:"page"`
	Limit      int                `json:"limit"`
	TotalPages int                `json:"total_pages"`
}

// ListEmailBlasts retrieves email blasts for a campaign with pagination
func (p *EmailBlastProcessor) ListEmailBlasts(ctx context.Context, accountID, campaignID uuid.UUID, req ListEmailBlastsRequest) (ListEmailBlastsResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ListEmailBlastsResponse{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return ListEmailBlastsResponse{}, err
	}

	if campaign.AccountID != accountID {
		return ListEmailBlastsResponse{}, ErrUnauthorized
	}

	// Default pagination
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 25
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	offset := (req.Page - 1) * req.Limit

	blasts, err := p.store.GetEmailBlastsByCampaign(ctx, campaignID, req.Limit, offset)
	if err != nil {
		p.logger.Error(ctx, "failed to list email blasts", err)
		return ListEmailBlastsResponse{}, err
	}

	if blasts == nil {
		blasts = []store.EmailBlast{}
	}

	total, err := p.store.CountEmailBlastsByCampaign(ctx, campaignID)
	if err != nil {
		p.logger.Error(ctx, "failed to count email blasts", err)
		return ListEmailBlastsResponse{}, err
	}

	totalPages := (total + req.Limit - 1) / req.Limit

	return ListEmailBlastsResponse{
		Blasts:     blasts,
		Total:      total,
		Page:       req.Page,
		Limit:      req.Limit,
		TotalPages: totalPages,
	}, nil
}

// UpdateEmailBlastRequest represents a request to update an email blast
type UpdateEmailBlastRequest struct {
	Name      *string
	Subject   *string
	BatchSize *int
}

// UpdateEmailBlast updates an email blast (only if in draft status)
func (p *EmailBlastProcessor) UpdateEmailBlast(ctx context.Context, accountID, campaignID, blastID uuid.UUID, req UpdateEmailBlastRequest) (store.EmailBlast, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "blast_id", Value: blastID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailBlast{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return store.EmailBlast{}, err
	}

	if campaign.AccountID != accountID {
		return store.EmailBlast{}, ErrUnauthorized
	}

	// Verify blast exists and belongs to campaign
	existingBlast, err := p.store.GetEmailBlastByID(ctx, blastID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailBlast{}, ErrBlastNotFound
		}
		p.logger.Error(ctx, "failed to get email blast", err)
		return store.EmailBlast{}, err
	}

	if existingBlast.CampaignID != campaignID {
		return store.EmailBlast{}, ErrUnauthorized
	}

	// Only allow updates if blast is in draft status
	if existingBlast.Status != string(store.EmailBlastStatusDraft) {
		return store.EmailBlast{}, ErrBlastCannotModify
	}

	params := store.UpdateEmailBlastParams{
		Name:      req.Name,
		Subject:   req.Subject,
		BatchSize: req.BatchSize,
	}

	blast, err := p.store.UpdateEmailBlast(ctx, blastID, params)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailBlast{}, ErrBlastNotFound
		}
		p.logger.Error(ctx, "failed to update email blast", err)
		return store.EmailBlast{}, err
	}

	p.logger.Info(ctx, "email blast updated successfully")
	return blast, nil
}

// DeleteEmailBlast soft deletes an email blast
func (p *EmailBlastProcessor) DeleteEmailBlast(ctx context.Context, accountID, campaignID, blastID uuid.UUID) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "blast_id", Value: blastID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return err
	}

	if campaign.AccountID != accountID {
		return ErrUnauthorized
	}

	// Verify blast exists and belongs to campaign
	existingBlast, err := p.store.GetEmailBlastByID(ctx, blastID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrBlastNotFound
		}
		p.logger.Error(ctx, "failed to get email blast", err)
		return err
	}

	if existingBlast.CampaignID != campaignID {
		return ErrUnauthorized
	}

	err = p.store.DeleteEmailBlast(ctx, blastID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrBlastCannotDelete
		}
		p.logger.Error(ctx, "failed to delete email blast", err)
		return err
	}

	p.logger.Info(ctx, "email blast deleted successfully")
	return nil
}

// ScheduleBlast schedules a blast for future sending
func (p *EmailBlastProcessor) ScheduleBlast(ctx context.Context, accountID, campaignID, blastID uuid.UUID, scheduledAt time.Time) (store.EmailBlast, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "blast_id", Value: blastID.String()},
	)

	// Validate scheduled time
	if scheduledAt.Before(time.Now()) {
		return store.EmailBlast{}, ErrInvalidScheduleTime
	}

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailBlast{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return store.EmailBlast{}, err
	}

	if campaign.AccountID != accountID {
		return store.EmailBlast{}, ErrUnauthorized
	}

	// Verify blast exists and is in draft status
	existingBlast, err := p.store.GetEmailBlastByID(ctx, blastID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailBlast{}, ErrBlastNotFound
		}
		p.logger.Error(ctx, "failed to get email blast", err)
		return store.EmailBlast{}, err
	}

	if existingBlast.CampaignID != campaignID {
		return store.EmailBlast{}, ErrUnauthorized
	}

	if existingBlast.Status != string(store.EmailBlastStatusDraft) {
		return store.EmailBlast{}, ErrBlastCannotModify
	}

	blast, err := p.store.ScheduleBlast(ctx, blastID, scheduledAt)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailBlast{}, ErrBlastNotFound
		}
		p.logger.Error(ctx, "failed to schedule blast", err)
		return store.EmailBlast{}, err
	}

	p.logger.Info(ctx, "email blast scheduled successfully")
	return blast, nil
}

// SendBlastNow starts sending a blast immediately
func (p *EmailBlastProcessor) SendBlastNow(ctx context.Context, accountID, campaignID, blastID uuid.UUID) (store.EmailBlast, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "blast_id", Value: blastID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailBlast{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return store.EmailBlast{}, err
	}

	if campaign.AccountID != accountID {
		return store.EmailBlast{}, ErrUnauthorized
	}

	// Verify blast exists and is in a valid status to send
	existingBlast, err := p.store.GetEmailBlastByID(ctx, blastID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailBlast{}, ErrBlastNotFound
		}
		p.logger.Error(ctx, "failed to get email blast", err)
		return store.EmailBlast{}, err
	}

	if existingBlast.CampaignID != campaignID {
		return store.EmailBlast{}, ErrUnauthorized
	}

	if existingBlast.Status != string(store.EmailBlastStatusDraft) && existingBlast.Status != string(store.EmailBlastStatusScheduled) {
		return store.EmailBlast{}, ErrBlastCannotStart
	}

	// Get segment filter criteria
	segment, err := p.store.GetSegmentByID(ctx, existingBlast.SegmentID)
	if err != nil {
		p.logger.Error(ctx, "failed to get segment", err)
		return store.EmailBlast{}, err
	}

	// Parse filter criteria
	criteria, err := store.ParseFilterCriteria(segment.FilterCriteria)
	if err != nil {
		p.logger.Error(ctx, "failed to parse filter criteria", err)
		return store.EmailBlast{}, err
	}

	// Get matching users and create recipients
	users, err := p.store.GetUsersForBlast(ctx, campaignID, criteria)
	if err != nil {
		p.logger.Error(ctx, "failed to get users for blast", err)
		return store.EmailBlast{}, err
	}

	if len(users) == 0 {
		return store.EmailBlast{}, ErrNoRecipients
	}

	// Create blast recipients
	err = p.store.CreateBlastRecipientsBulk(ctx, blastID, users, existingBlast.BatchSize)
	if err != nil {
		p.logger.Error(ctx, "failed to create blast recipients", err)
		return store.EmailBlast{}, err
	}

	// Update total recipients
	err = p.store.UpdateEmailBlastTotalRecipients(ctx, blastID, len(users))
	if err != nil {
		p.logger.Error(ctx, "failed to update total recipients", err)
		return store.EmailBlast{}, err
	}

	// Update status to processing
	blast, err := p.store.UpdateEmailBlastStatus(ctx, blastID, string(store.EmailBlastStatusProcessing), nil)
	if err != nil {
		p.logger.Error(ctx, "failed to update blast status", err)
		return store.EmailBlast{}, err
	}

	p.logger.Info(ctx, "email blast started successfully")
	return blast, nil
}

// PauseBlast pauses a sending blast
func (p *EmailBlastProcessor) PauseBlast(ctx context.Context, accountID, campaignID, blastID uuid.UUID) (store.EmailBlast, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "blast_id", Value: blastID.String()},
	)

	// Verify and get existing blast
	existingBlast, err := p.GetEmailBlast(ctx, accountID, campaignID, blastID)
	if err != nil {
		return store.EmailBlast{}, err
	}

	if existingBlast.Status != string(store.EmailBlastStatusSending) && existingBlast.Status != string(store.EmailBlastStatusProcessing) {
		return store.EmailBlast{}, ErrBlastCannotPause
	}

	blast, err := p.store.UpdateEmailBlastStatus(ctx, blastID, string(store.EmailBlastStatusPaused), nil)
	if err != nil {
		p.logger.Error(ctx, "failed to pause blast", err)
		return store.EmailBlast{}, err
	}

	p.logger.Info(ctx, "email blast paused successfully")
	return blast, nil
}

// ResumeBlast resumes a paused blast
func (p *EmailBlastProcessor) ResumeBlast(ctx context.Context, accountID, campaignID, blastID uuid.UUID) (store.EmailBlast, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "blast_id", Value: blastID.String()},
	)

	// Verify and get existing blast
	existingBlast, err := p.GetEmailBlast(ctx, accountID, campaignID, blastID)
	if err != nil {
		return store.EmailBlast{}, err
	}

	if existingBlast.Status != string(store.EmailBlastStatusPaused) {
		return store.EmailBlast{}, ErrBlastCannotResume
	}

	blast, err := p.store.UpdateEmailBlastStatus(ctx, blastID, string(store.EmailBlastStatusSending), nil)
	if err != nil {
		p.logger.Error(ctx, "failed to resume blast", err)
		return store.EmailBlast{}, err
	}

	p.logger.Info(ctx, "email blast resumed successfully")
	return blast, nil
}

// CancelBlast cancels a blast
func (p *EmailBlastProcessor) CancelBlast(ctx context.Context, accountID, campaignID, blastID uuid.UUID) (store.EmailBlast, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "blast_id", Value: blastID.String()},
	)

	// Verify and get existing blast
	existingBlast, err := p.GetEmailBlast(ctx, accountID, campaignID, blastID)
	if err != nil {
		return store.EmailBlast{}, err
	}

	// Can cancel from most non-terminal states
	validStatuses := map[string]bool{
		string(store.EmailBlastStatusDraft):      true,
		string(store.EmailBlastStatusScheduled):  true,
		string(store.EmailBlastStatusProcessing): true,
		string(store.EmailBlastStatusSending):    true,
		string(store.EmailBlastStatusPaused):     true,
	}

	if !validStatuses[existingBlast.Status] {
		return store.EmailBlast{}, ErrBlastCannotCancel
	}

	blast, err := p.store.UpdateEmailBlastStatus(ctx, blastID, string(store.EmailBlastStatusCancelled), nil)
	if err != nil {
		p.logger.Error(ctx, "failed to cancel blast", err)
		return store.EmailBlast{}, err
	}

	p.logger.Info(ctx, "email blast cancelled successfully")
	return blast, nil
}

// BlastAnalytics represents analytics for an email blast
type BlastAnalytics struct {
	BlastID         uuid.UUID  `json:"blast_id"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	TotalRecipients int        `json:"total_recipients"`
	Sent            int        `json:"sent"`
	Delivered       int        `json:"delivered"`
	Opened          int        `json:"opened"`
	Clicked         int        `json:"clicked"`
	Bounced         int        `json:"bounced"`
	Failed          int        `json:"failed"`
	OpenRate        float64    `json:"open_rate"`
	ClickRate       float64    `json:"click_rate"`
	BounceRate      float64    `json:"bounce_rate"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	DurationSeconds *int       `json:"duration_seconds,omitempty"`
}

// GetBlastAnalytics retrieves analytics for an email blast
func (p *EmailBlastProcessor) GetBlastAnalytics(ctx context.Context, accountID, campaignID, blastID uuid.UUID) (BlastAnalytics, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "blast_id", Value: blastID.String()},
	)

	blast, err := p.GetEmailBlast(ctx, accountID, campaignID, blastID)
	if err != nil {
		return BlastAnalytics{}, err
	}

	stats, err := p.store.GetBlastRecipientStats(ctx, blastID)
	if err != nil {
		p.logger.Error(ctx, "failed to get blast recipient stats", err)
		return BlastAnalytics{}, err
	}

	analytics := BlastAnalytics{
		BlastID:         blast.ID,
		Name:            blast.Name,
		Status:          blast.Status,
		TotalRecipients: blast.TotalRecipients,
		Sent:            stats.Sent + stats.Delivered + stats.Opened + stats.Clicked,
		Delivered:       stats.Delivered + stats.Opened + stats.Clicked,
		Opened:          stats.Opened + stats.Clicked,
		Clicked:         stats.Clicked,
		Bounced:         stats.Bounced,
		Failed:          stats.Failed,
		StartedAt:       blast.StartedAt,
		CompletedAt:     blast.CompletedAt,
	}

	// Calculate rates
	if analytics.Sent > 0 {
		analytics.OpenRate = float64(analytics.Opened) / float64(analytics.Sent) * 100
		analytics.ClickRate = float64(analytics.Clicked) / float64(analytics.Sent) * 100
		analytics.BounceRate = float64(analytics.Bounced) / float64(analytics.Sent) * 100
	}

	// Calculate duration
	if blast.StartedAt != nil && blast.CompletedAt != nil {
		duration := int(blast.CompletedAt.Sub(*blast.StartedAt).Seconds())
		analytics.DurationSeconds = &duration
	}

	return analytics, nil
}

// ListBlastRecipientsRequest represents a request to list blast recipients
type ListBlastRecipientsRequest struct {
	Page  int
	Limit int
}

// ListBlastRecipientsResponse represents the response for listing blast recipients
type ListBlastRecipientsResponse struct {
	Recipients []store.BlastRecipient `json:"recipients"`
	Total      int                    `json:"total"`
	Page       int                    `json:"page"`
	Limit      int                    `json:"limit"`
	TotalPages int                    `json:"total_pages"`
}

// ListBlastRecipients retrieves recipients for a blast with pagination
func (p *EmailBlastProcessor) ListBlastRecipients(ctx context.Context, accountID, campaignID, blastID uuid.UUID, req ListBlastRecipientsRequest) (ListBlastRecipientsResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "blast_id", Value: blastID.String()},
	)

	// Verify access
	_, err := p.GetEmailBlast(ctx, accountID, campaignID, blastID)
	if err != nil {
		return ListBlastRecipientsResponse{}, err
	}

	// Default pagination
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 25
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	offset := (req.Page - 1) * req.Limit

	recipients, err := p.store.GetBlastRecipientsByBlast(ctx, blastID, req.Limit, offset)
	if err != nil {
		p.logger.Error(ctx, "failed to list blast recipients", err)
		return ListBlastRecipientsResponse{}, err
	}

	if recipients == nil {
		recipients = []store.BlastRecipient{}
	}

	total, err := p.store.CountBlastRecipientsByBlast(ctx, blastID)
	if err != nil {
		p.logger.Error(ctx, "failed to count blast recipients", err)
		return ListBlastRecipientsResponse{}, err
	}

	totalPages := (total + req.Limit - 1) / req.Limit

	return ListBlastRecipientsResponse{
		Recipients: recipients,
		Total:      total,
		Page:       req.Page,
		Limit:      req.Limit,
		TotalPages: totalPages,
	}, nil
}
