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

// CreateCampaignRequest represents a request to create a campaign
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

// CreateCampaign creates a new campaign for an account
func (p *CampaignProcessor) CreateCampaign(ctx context.Context, accountID uuid.UUID, req CreateCampaignRequest) (store.Campaign, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_slug", Value: req.Slug},
	)

	// Validate campaign type
	if !isValidCampaignType(req.Type) {
		return store.Campaign{}, ErrInvalidCampaignType
	}

	// Check if slug already exists for this account
	existingCampaign, err := p.store.GetCampaignBySlug(ctx, accountID, req.Slug)
	if err == nil && existingCampaign.ID != uuid.Nil {
		return store.Campaign{}, ErrSlugAlreadyExists
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		p.logger.Error(ctx, "failed to check slug existence", err)
		return store.Campaign{}, err
	}

	// Set defaults for JSONB fields if not provided
	if req.FormConfig == nil {
		req.FormConfig = store.JSONB{}
	}
	if req.ReferralConfig == nil {
		req.ReferralConfig = store.JSONB{}
	}
	if req.EmailConfig == nil {
		req.EmailConfig = store.JSONB{}
	}
	if req.BrandingConfig == nil {
		req.BrandingConfig = store.JSONB{}
	}

	params := store.CreateCampaignParams{
		AccountID:        accountID,
		Name:             req.Name,
		Slug:             req.Slug,
		Description:      req.Description,
		Type:             req.Type,
		FormConfig:       req.FormConfig,
		ReferralConfig:   req.ReferralConfig,
		EmailConfig:      req.EmailConfig,
		BrandingConfig:   req.BrandingConfig,
		PrivacyPolicyURL: req.PrivacyPolicyURL,
		TermsURL:         req.TermsURL,
		MaxSignups:       req.MaxSignups,
	}

	campaign, err := p.store.CreateCampaign(ctx, params)
	if err != nil {
		p.logger.Error(ctx, "failed to create campaign", err)
		return store.Campaign{}, err
	}

	p.logger.Info(ctx, "campaign created successfully")
	return campaign, nil
}

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

// UpdateCampaignRequest represents a request to update a campaign
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

// UpdateCampaign updates a campaign
func (p *CampaignProcessor) UpdateCampaign(ctx context.Context, accountID, campaignID uuid.UUID, req UpdateCampaignRequest) (store.Campaign, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	params := store.UpdateCampaignParams{
		Name:             req.Name,
		Description:      req.Description,
		FormConfig:       req.FormConfig,
		ReferralConfig:   req.ReferralConfig,
		EmailConfig:      req.EmailConfig,
		BrandingConfig:   req.BrandingConfig,
		PrivacyPolicyURL: req.PrivacyPolicyURL,
		TermsURL:         req.TermsURL,
		MaxSignups:       req.MaxSignups,
	}

	campaign, err := p.store.UpdateCampaign(ctx, accountID, campaignID, params)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Campaign{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to update campaign", err)
		return store.Campaign{}, err
	}

	p.logger.Info(ctx, "campaign updated successfully")
	return campaign, nil
}

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

// UpdateReferralConfigRequest represents a request to update referral configuration
type UpdateReferralConfigRequest struct {
	PositionsPerReferral int  `json:"positions_per_referral" binding:"min=1,max=100"`
	VerifiedOnly         bool `json:"verified_only"`
}

// UpdateReferralConfig updates the referral configuration for a campaign
// This allows configuring how many positions a user jumps per referral
func (p *CampaignProcessor) UpdateReferralConfig(ctx context.Context, accountID, campaignID uuid.UUID, req UpdateReferralConfigRequest) (store.Campaign, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "positions_per_referral", Value: req.PositionsPerReferral},
	)

	// Get existing campaign to preserve other referral config settings
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

	// Update referral config with new values
	referralConfig := campaign.ReferralConfig
	if referralConfig == nil {
		referralConfig = store.JSONB{}
	}
	referralConfig["positions_per_referral"] = req.PositionsPerReferral
	referralConfig["verified_only"] = req.VerifiedOnly

	// Update campaign
	params := store.UpdateCampaignParams{
		ReferralConfig: referralConfig,
	}

	updatedCampaign, err := p.store.UpdateCampaign(ctx, accountID, campaignID, params)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Campaign{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to update referral config", err)
		return store.Campaign{}, err
	}

	p.logger.Info(ctx, "referral config updated successfully")
	return updatedCampaign, nil
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
