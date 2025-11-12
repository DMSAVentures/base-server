package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrCampaignNotFound      = errors.New("campaign not found")
	ErrSlugAlreadyExists     = errors.New("slug already exists")
	ErrInvalidCampaignStatus = errors.New("invalid campaign status")
	ErrInvalidCampaignType   = errors.New("invalid campaign type")
	ErrUnauthorized          = errors.New("unauthorized access to campaign")
)

type CampaignProcessor struct {
	store  CampaignStore
	logger *observability.Logger
}

func New(store CampaignStore, logger *observability.Logger) CampaignProcessor {
	return CampaignProcessor{
		store:  store,
		logger: logger,
	}
}

// CreateCampaignRequest represents a request to create a campaign (DEPRECATED - use CreateCampaignRequestV2)
// This is kept for compatibility but handlers should use CreateCampaignV2
type CreateCampaignRequest struct {
	Name             string
	Slug             string
	Description      *string
	Type             string
	FormConfig       store.JSONB
	ReferralConfig   store.JSONB
	EmailConfig      store.JSONB
	BrandingConfig   store.JSONB
	PrivacyPolicyURL *string
	TermsURL         *string
	MaxSignups       *int
}

// CreateCampaign is DEPRECATED - handlers now use CreateCampaignV2 with typed configs
// This method is kept for backwards compatibility but should not be used

// GetCampaign retrieves a campaign by ID
func (p *CampaignProcessor) GetCampaign(ctx context.Context, accountID, campaignID uuid.UUID) (store.Campaign, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Campaign{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return store.Campaign{}, err
	}

	// Verify campaign belongs to account
	if campaign.AccountID != accountID {
		return store.Campaign{}, ErrUnauthorized
	}

	return campaign, nil
}

// GetPublicCampaign retrieves a campaign by ID without authentication (for public form rendering)
func (p *CampaignProcessor) GetPublicCampaign(ctx context.Context, campaignID uuid.UUID) (store.Campaign, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Campaign{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get public campaign", err)
		return store.Campaign{}, err
	}

	return campaign, nil
}

// ListCampaigns retrieves campaigns with pagination and filters
func (p *CampaignProcessor) ListCampaigns(ctx context.Context, accountID uuid.UUID, status, campaignType *string, page, limit int) (store.ListCampaignsResult, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "page", Value: page},
		observability.Field{Key: "limit", Value: limit},
	)

	// Validate filters if provided
	if status != nil && !isValidCampaignStatus(*status) {
		return store.ListCampaignsResult{}, ErrInvalidCampaignStatus
	}
	if campaignType != nil && !isValidCampaignType(*campaignType) {
		return store.ListCampaignsResult{}, ErrInvalidCampaignType
	}

	// Set default pagination values
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	params := store.ListCampaignsParams{
		AccountID: accountID,
		Status:    status,
		Type:      campaignType,
		Page:      page,
		Limit:     limit,
	}

	result, err := p.store.ListCampaigns(ctx, params)
	if err != nil {
		p.logger.Error(ctx, "failed to list campaigns", err)
		return store.ListCampaignsResult{}, err
	}

	// Ensure campaigns is never null - return empty array instead
	if result.Campaigns == nil {
		result.Campaigns = []store.Campaign{}
	}

	return result, nil
}

// UpdateCampaignRequest represents a request to update a campaign (DEPRECATED - use UpdateCampaignRequestV2)
// This is kept for compatibility but handlers should use UpdateCampaignV2
type UpdateCampaignRequest struct {
	Name             *string
	Description      *string
	LaunchDate       *string
	EndDate          *string
	FormConfig       store.JSONB
	ReferralConfig   store.JSONB
	EmailConfig      store.JSONB
	BrandingConfig   store.JSONB
	PrivacyPolicyURL *string
	TermsURL         *string
	MaxSignups       *int
}

// UpdateCampaign is DEPRECATED - handlers now use UpdateCampaignV2 with typed configs
// This method is kept for backwards compatibility but should not be used

// UpdateCampaignStatus updates a campaign's status
func (p *CampaignProcessor) UpdateCampaignStatus(ctx context.Context, accountID, campaignID uuid.UUID, status string) (store.Campaign, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "new_status", Value: status},
	)

	// Validate status
	if !isValidCampaignStatus(status) {
		return store.Campaign{}, ErrInvalidCampaignStatus
	}

	campaign, err := p.store.UpdateCampaignStatus(ctx, accountID, campaignID, status)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Campaign{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to update campaign status", err)
		return store.Campaign{}, err
	}

	p.logger.Info(ctx, "campaign status updated successfully")
	return campaign, nil
}

// DeleteCampaign soft deletes a campaign
func (p *CampaignProcessor) DeleteCampaign(ctx context.Context, accountID, campaignID uuid.UUID) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	err := p.store.DeleteCampaign(ctx, accountID, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to delete campaign", err)
		return err
	}

	p.logger.Info(ctx, "campaign deleted successfully")
	return nil
}

// Helper functions

func isValidCampaignStatus(status string) bool {
	validStatuses := map[string]bool{
		store.CampaignStatusDraft:     true,
		store.CampaignStatusActive:    true,
		store.CampaignStatusPaused:    true,
		store.CampaignStatusCompleted: true,
	}
	return validStatuses[status]
}

func isValidCampaignType(campaignType string) bool {
	validTypes := map[string]bool{
		store.CampaignTypeWaitlist: true,
		store.CampaignTypeReferral: true,
		store.CampaignTypeContest:  true,
	}
	return validTypes[campaignType]
}
