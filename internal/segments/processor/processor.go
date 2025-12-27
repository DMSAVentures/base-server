package processor

//go:generate go run go.uber.org/mock/mockgen@latest -source=processor.go -destination=mocks_test.go -package=processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"

	"github.com/google/uuid"
)

// SegmentStore defines the database operations required by SegmentProcessor
type SegmentStore interface {
	GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (store.Campaign, error)
	CreateSegment(ctx context.Context, params store.CreateSegmentParams) (store.Segment, error)
	GetSegmentByID(ctx context.Context, segmentID uuid.UUID) (store.Segment, error)
	GetSegmentsByCampaign(ctx context.Context, campaignID uuid.UUID) ([]store.Segment, error)
	GetActiveSegmentsByCampaign(ctx context.Context, campaignID uuid.UUID) ([]store.Segment, error)
	UpdateSegment(ctx context.Context, segmentID uuid.UUID, params store.UpdateSegmentParams) (store.Segment, error)
	DeleteSegment(ctx context.Context, segmentID uuid.UUID) error
	UpdateSegmentCachedCount(ctx context.Context, segmentID uuid.UUID, count int) error
	CountUsersMatchingCriteria(ctx context.Context, campaignID uuid.UUID, criteria store.SegmentFilterCriteria) (int, error)
	GetUsersMatchingCriteria(ctx context.Context, campaignID uuid.UUID, criteria store.SegmentFilterCriteria, limit, offset int) ([]store.WaitlistUser, error)
}

var (
	ErrSegmentNotFound  = errors.New("segment not found")
	ErrCampaignNotFound = errors.New("campaign not found")
	ErrUnauthorized     = errors.New("unauthorized access to segment")
	ErrInvalidCriteria  = errors.New("invalid filter criteria")
	ErrSegmentInUse     = errors.New("segment is in use by an email blast")
)

type SegmentProcessor struct {
	store  SegmentStore
	logger *observability.Logger
}

func New(store SegmentStore, logger *observability.Logger) SegmentProcessor {
	return SegmentProcessor{
		store:  store,
		logger: logger,
	}
}

// CreateSegmentRequest represents a request to create a segment
type CreateSegmentRequest struct {
	Name           string
	Description    *string
	FilterCriteria store.SegmentFilterCriteria
}

// CreateSegment creates a new segment for a campaign
func (p *SegmentProcessor) CreateSegment(ctx context.Context, accountID, campaignID uuid.UUID, req CreateSegmentRequest) (store.Segment, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Segment{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return store.Segment{}, err
	}

	if campaign.AccountID != accountID {
		return store.Segment{}, ErrUnauthorized
	}

	// Convert filter criteria to JSONB
	filterCriteriaJSON := filterCriteriaToJSONB(req.FilterCriteria)

	params := store.CreateSegmentParams{
		CampaignID:     campaignID,
		Name:           req.Name,
		Description:    req.Description,
		FilterCriteria: filterCriteriaJSON,
	}

	segment, err := p.store.CreateSegment(ctx, params)
	if err != nil {
		p.logger.Error(ctx, "failed to create segment", err)
		return store.Segment{}, err
	}

	// Calculate and cache user count
	count, err := p.store.CountUsersMatchingCriteria(ctx, campaignID, req.FilterCriteria)
	if err != nil {
		p.logger.Error(ctx, "failed to count users for segment", err)
	} else {
		if updateErr := p.store.UpdateSegmentCachedCount(ctx, segment.ID, count); updateErr != nil {
			p.logger.Error(ctx, "failed to update segment cached count", updateErr)
		}
		segment.CachedUserCount = count
	}

	p.logger.Info(ctx, "segment created successfully")
	return segment, nil
}

// GetSegment retrieves a segment by ID
func (p *SegmentProcessor) GetSegment(ctx context.Context, accountID, campaignID, segmentID uuid.UUID) (store.Segment, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "segment_id", Value: segmentID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Segment{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return store.Segment{}, err
	}

	if campaign.AccountID != accountID {
		return store.Segment{}, ErrUnauthorized
	}

	segment, err := p.store.GetSegmentByID(ctx, segmentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Segment{}, ErrSegmentNotFound
		}
		p.logger.Error(ctx, "failed to get segment", err)
		return store.Segment{}, err
	}

	// Verify segment belongs to the campaign
	if segment.CampaignID != campaignID {
		return store.Segment{}, ErrUnauthorized
	}

	return segment, nil
}

// ListSegments retrieves all segments for a campaign
func (p *SegmentProcessor) ListSegments(ctx context.Context, accountID, campaignID uuid.UUID) ([]store.Segment, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return nil, err
	}

	if campaign.AccountID != accountID {
		return nil, ErrUnauthorized
	}

	segments, err := p.store.GetSegmentsByCampaign(ctx, campaignID)
	if err != nil {
		p.logger.Error(ctx, "failed to list segments", err)
		return nil, err
	}

	// Ensure segments is never null
	if segments == nil {
		segments = []store.Segment{}
	}

	return segments, nil
}

// UpdateSegmentRequest represents a request to update a segment
type UpdateSegmentRequest struct {
	Name           *string
	Description    *string
	FilterCriteria *store.SegmentFilterCriteria
	Status         *string
}

// UpdateSegment updates a segment
func (p *SegmentProcessor) UpdateSegment(ctx context.Context, accountID, campaignID, segmentID uuid.UUID, req UpdateSegmentRequest) (store.Segment, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "segment_id", Value: segmentID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Segment{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return store.Segment{}, err
	}

	if campaign.AccountID != accountID {
		return store.Segment{}, ErrUnauthorized
	}

	// Verify segment exists and belongs to campaign
	existingSegment, err := p.store.GetSegmentByID(ctx, segmentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Segment{}, ErrSegmentNotFound
		}
		p.logger.Error(ctx, "failed to get segment", err)
		return store.Segment{}, err
	}

	if existingSegment.CampaignID != campaignID {
		return store.Segment{}, ErrUnauthorized
	}

	// Validate status if provided
	if req.Status != nil && !isValidSegmentStatus(*req.Status) {
		return store.Segment{}, ErrInvalidCriteria
	}

	var filterCriteriaJSON *store.JSONB
	if req.FilterCriteria != nil {
		jsonb := filterCriteriaToJSONB(*req.FilterCriteria)
		filterCriteriaJSON = &jsonb
	}

	params := store.UpdateSegmentParams{
		Name:           req.Name,
		Description:    req.Description,
		FilterCriteria: filterCriteriaJSON,
		Status:         req.Status,
	}

	segment, err := p.store.UpdateSegment(ctx, segmentID, params)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Segment{}, ErrSegmentNotFound
		}
		p.logger.Error(ctx, "failed to update segment", err)
		return store.Segment{}, err
	}

	// Recalculate cached count if filter criteria changed
	if req.FilterCriteria != nil {
		count, countErr := p.store.CountUsersMatchingCriteria(ctx, campaignID, *req.FilterCriteria)
		if countErr != nil {
			p.logger.Error(ctx, "failed to count users for segment", countErr)
		} else {
			if updateErr := p.store.UpdateSegmentCachedCount(ctx, segment.ID, count); updateErr != nil {
				p.logger.Error(ctx, "failed to update segment cached count", updateErr)
			}
			segment.CachedUserCount = count
		}
	}

	p.logger.Info(ctx, "segment updated successfully")
	return segment, nil
}

// DeleteSegment soft deletes a segment
func (p *SegmentProcessor) DeleteSegment(ctx context.Context, accountID, campaignID, segmentID uuid.UUID) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "segment_id", Value: segmentID.String()},
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

	// Verify segment exists and belongs to campaign
	existingSegment, err := p.store.GetSegmentByID(ctx, segmentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrSegmentNotFound
		}
		p.logger.Error(ctx, "failed to get segment", err)
		return err
	}

	if existingSegment.CampaignID != campaignID {
		return ErrUnauthorized
	}

	err = p.store.DeleteSegment(ctx, segmentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrSegmentNotFound
		}
		p.logger.Error(ctx, "failed to delete segment", err)
		return err
	}

	p.logger.Info(ctx, "segment deleted successfully")
	return nil
}

// PreviewSegmentRequest represents a request to preview a segment
type PreviewSegmentRequest struct {
	FilterCriteria store.SegmentFilterCriteria
	SampleLimit    int
}

// SegmentPreview represents the preview response
type SegmentPreview struct {
	Count       int                  `json:"count"`
	SampleUsers []store.WaitlistUser `json:"sample_users"`
}

// PreviewSegment previews segment matching without saving
func (p *SegmentProcessor) PreviewSegment(ctx context.Context, accountID, campaignID uuid.UUID, req PreviewSegmentRequest) (SegmentPreview, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return SegmentPreview{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return SegmentPreview{}, err
	}

	if campaign.AccountID != accountID {
		return SegmentPreview{}, ErrUnauthorized
	}

	// Default sample limit
	if req.SampleLimit <= 0 {
		req.SampleLimit = 10
	}
	if req.SampleLimit > 100 {
		req.SampleLimit = 100
	}

	// Count matching users
	count, err := p.store.CountUsersMatchingCriteria(ctx, campaignID, req.FilterCriteria)
	if err != nil {
		p.logger.Error(ctx, "failed to count users", err)
		return SegmentPreview{}, err
	}

	// Get sample users
	users, err := p.store.GetUsersMatchingCriteria(ctx, campaignID, req.FilterCriteria, req.SampleLimit, 0)
	if err != nil {
		p.logger.Error(ctx, "failed to get sample users", err)
		return SegmentPreview{}, err
	}

	if users == nil {
		users = []store.WaitlistUser{}
	}

	return SegmentPreview{
		Count:       count,
		SampleUsers: users,
	}, nil
}

// RefreshSegmentCount refreshes the cached user count for a segment
func (p *SegmentProcessor) RefreshSegmentCount(ctx context.Context, accountID, campaignID, segmentID uuid.UUID) (int, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "segment_id", Value: segmentID.String()},
	)

	// Get the segment
	segment, err := p.GetSegment(ctx, accountID, campaignID, segmentID)
	if err != nil {
		return 0, err
	}

	// Parse filter criteria
	criteria, err := store.ParseFilterCriteria(segment.FilterCriteria)
	if err != nil {
		p.logger.Error(ctx, "failed to parse filter criteria", err)
		return 0, ErrInvalidCriteria
	}

	// Count matching users
	count, err := p.store.CountUsersMatchingCriteria(ctx, campaignID, criteria)
	if err != nil {
		p.logger.Error(ctx, "failed to count users", err)
		return 0, err
	}

	// Update cached count
	if err := p.store.UpdateSegmentCachedCount(ctx, segmentID, count); err != nil {
		p.logger.Error(ctx, "failed to update cached count", err)
		return 0, err
	}

	p.logger.Info(ctx, "segment count refreshed successfully")
	return count, nil
}

// Helper functions

func filterCriteriaToJSONB(criteria store.SegmentFilterCriteria) store.JSONB {
	jsonb := make(store.JSONB)

	if len(criteria.Statuses) > 0 {
		jsonb["statuses"] = criteria.Statuses
	}
	if len(criteria.Sources) > 0 {
		jsonb["sources"] = criteria.Sources
	}
	if criteria.EmailVerified != nil {
		jsonb["email_verified"] = *criteria.EmailVerified
	}
	if criteria.HasReferrals != nil {
		jsonb["has_referrals"] = *criteria.HasReferrals
	}
	if criteria.MinReferrals != nil {
		jsonb["min_referrals"] = *criteria.MinReferrals
	}
	if criteria.MinPosition != nil {
		jsonb["min_position"] = *criteria.MinPosition
	}
	if criteria.MaxPosition != nil {
		jsonb["max_position"] = *criteria.MaxPosition
	}
	if criteria.DateFrom != nil {
		jsonb["date_from"] = criteria.DateFrom.Format("2006-01-02T15:04:05Z07:00")
	}
	if criteria.DateTo != nil {
		jsonb["date_to"] = criteria.DateTo.Format("2006-01-02T15:04:05Z07:00")
	}
	if len(criteria.CustomFields) > 0 {
		jsonb["custom_fields"] = criteria.CustomFields
	}

	return jsonb
}

func isValidSegmentStatus(status string) bool {
	validStatuses := map[string]bool{
		"active":   true,
		"archived": true,
	}
	return validStatuses[status]
}
