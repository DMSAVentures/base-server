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
	PrivacyPolicyURL *string
	TermsURL         *string
	MaxSignups       *int

	// Settings
	EmailSettings        *EmailSettingsInput
	BrandingSettings     *BrandingSettingsInput
	FormSettings         *FormSettingsInput
	ReferralSettings     *ReferralSettingsInput
	FormFields           []FormFieldInput
	ShareMessages        []ShareMessageInput
	TrackingIntegrations []TrackingIntegrationInput
}

// EmailSettingsInput represents email settings input
type EmailSettingsInput struct {
	FromName             *string
	FromEmail            *string
	ReplyTo              *string
	VerificationRequired bool
	SendWelcomeEmail     bool
}

// BrandingSettingsInput represents branding settings input
type BrandingSettingsInput struct {
	LogoURL      *string
	PrimaryColor *string
	FontFamily   *string
	CustomDomain *string
}

// FormSettingsInput represents form settings input
type FormSettingsInput struct {
	CaptchaEnabled  bool
	CaptchaProvider *store.CaptchaProvider
	CaptchaSiteKey  *string
	DoubleOptIn     bool
	Design          store.JSONB
	SuccessTitle    *string
	SuccessMessage  *string
}

// ReferralSettingsInput represents referral settings input
type ReferralSettingsInput struct {
	Enabled                 bool
	PointsPerReferral       int
	VerifiedOnly            bool
	PositionsToJump         int
	ReferrerPositionsToJump int
	SharingChannels         []store.SharingChannel
}

// FormFieldInput represents a form field input
type FormFieldInput struct {
	Name              string
	FieldType         store.FormFieldType
	Label             string
	Placeholder       *string
	Required          bool
	ValidationPattern *string
	Options           []string
	DisplayOrder      int
}

// ShareMessageInput represents a share message input
type ShareMessageInput struct {
	Channel store.SharingChannel
	Message string
}

// TrackingIntegrationInput represents a tracking integration input
type TrackingIntegrationInput struct {
	IntegrationType store.TrackingIntegrationType
	Enabled         bool
	TrackingID      string
	TrackingLabel   *string
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

	params := store.CreateCampaignParams{
		AccountID:        accountID,
		Name:             req.Name,
		Slug:             req.Slug,
		Description:      req.Description,
		Type:             req.Type,
		PrivacyPolicyURL: req.PrivacyPolicyURL,
		TermsURL:         req.TermsURL,
		MaxSignups:       req.MaxSignups,
	}

	campaign, err := p.store.CreateCampaign(ctx, params)
	if err != nil {
		p.logger.Error(ctx, "failed to create campaign", err)
		return store.Campaign{}, err
	}

	// Create settings
	if err := p.createCampaignSettings(ctx, campaign.ID, req); err != nil {
		p.logger.Error(ctx, "failed to create campaign settings", err)
		return store.Campaign{}, err
	}

	// Load campaign with settings
	campaignWithSettings, err := p.store.GetCampaignWithSettings(ctx, campaign.ID)
	if err != nil {
		p.logger.Error(ctx, "failed to load campaign with settings", err)
		return store.Campaign{}, err
	}

	p.logger.Info(ctx, "campaign created successfully")
	return campaignWithSettings, nil
}

// createCampaignSettings creates all settings for a campaign
func (p *CampaignProcessor) createCampaignSettings(ctx context.Context, campaignID uuid.UUID, req CreateCampaignRequest) error {
	// Create email settings
	if req.EmailSettings != nil {
		_, err := p.store.UpsertCampaignEmailSettings(ctx, store.CreateCampaignEmailSettingsParams{
			CampaignID:           campaignID,
			FromName:             req.EmailSettings.FromName,
			FromEmail:            req.EmailSettings.FromEmail,
			ReplyTo:              req.EmailSettings.ReplyTo,
			VerificationRequired: req.EmailSettings.VerificationRequired,
			SendWelcomeEmail:     req.EmailSettings.SendWelcomeEmail,
		})
		if err != nil {
			return err
		}
	}

	// Create branding settings
	if req.BrandingSettings != nil {
		_, err := p.store.UpsertCampaignBrandingSettings(ctx, store.CreateCampaignBrandingSettingsParams{
			CampaignID:   campaignID,
			LogoURL:      req.BrandingSettings.LogoURL,
			PrimaryColor: req.BrandingSettings.PrimaryColor,
			FontFamily:   req.BrandingSettings.FontFamily,
			CustomDomain: req.BrandingSettings.CustomDomain,
		})
		if err != nil {
			return err
		}
	}

	// Create form settings
	if req.FormSettings != nil {
		_, err := p.store.UpsertCampaignFormSettings(ctx, store.CreateCampaignFormSettingsParams{
			CampaignID:      campaignID,
			CaptchaEnabled:  req.FormSettings.CaptchaEnabled,
			CaptchaProvider: req.FormSettings.CaptchaProvider,
			CaptchaSiteKey:  req.FormSettings.CaptchaSiteKey,
			DoubleOptIn:     req.FormSettings.DoubleOptIn,
			Design:          req.FormSettings.Design,
			SuccessTitle:    req.FormSettings.SuccessTitle,
			SuccessMessage:  req.FormSettings.SuccessMessage,
		})
		if err != nil {
			return err
		}
	}

	// Create referral settings
	if req.ReferralSettings != nil {
		_, err := p.store.UpsertCampaignReferralSettings(ctx, store.CreateCampaignReferralSettingsParams{
			CampaignID:              campaignID,
			Enabled:                 req.ReferralSettings.Enabled,
			PointsPerReferral:       req.ReferralSettings.PointsPerReferral,
			VerifiedOnly:            req.ReferralSettings.VerifiedOnly,
			PositionsToJump:         req.ReferralSettings.PositionsToJump,
			ReferrerPositionsToJump: req.ReferralSettings.ReferrerPositionsToJump,
			SharingChannels:         req.ReferralSettings.SharingChannels,
		})
		if err != nil {
			return err
		}
	}

	// Create form fields
	if len(req.FormFields) > 0 {
		fields := make([]store.CreateCampaignFormFieldParams, len(req.FormFields))
		for i, f := range req.FormFields {
			fields[i] = store.CreateCampaignFormFieldParams{
				CampaignID:        campaignID,
				Name:              f.Name,
				FieldType:         f.FieldType,
				Label:             f.Label,
				Placeholder:       f.Placeholder,
				Required:          f.Required,
				ValidationPattern: f.ValidationPattern,
				Options:           f.Options,
				DisplayOrder:      f.DisplayOrder,
			}
		}
		_, err := p.store.ReplaceCampaignFormFields(ctx, campaignID, fields)
		if err != nil {
			return err
		}
	}

	// Create share messages
	if len(req.ShareMessages) > 0 {
		messages := make([]store.CreateCampaignShareMessageParams, len(req.ShareMessages))
		for i, m := range req.ShareMessages {
			messages[i] = store.CreateCampaignShareMessageParams{
				CampaignID: campaignID,
				Channel:    m.Channel,
				Message:    m.Message,
			}
		}
		_, err := p.store.ReplaceCampaignShareMessages(ctx, campaignID, messages)
		if err != nil {
			return err
		}
	}

	// Create tracking integrations
	if len(req.TrackingIntegrations) > 0 {
		integrations := make([]store.CreateCampaignTrackingIntegrationParams, len(req.TrackingIntegrations))
		for i, t := range req.TrackingIntegrations {
			integrations[i] = store.CreateCampaignTrackingIntegrationParams{
				CampaignID:      campaignID,
				IntegrationType: t.IntegrationType,
				Enabled:         t.Enabled,
				TrackingID:      t.TrackingID,
				TrackingLabel:   t.TrackingLabel,
			}
		}
		_, err := p.store.ReplaceCampaignTrackingIntegrations(ctx, campaignID, integrations)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetCampaign retrieves a campaign by ID with all settings
func (p *CampaignProcessor) GetCampaign(ctx context.Context, accountID, campaignID uuid.UUID) (store.Campaign, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	campaign, err := p.store.GetCampaignWithSettings(ctx, campaignID)
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

	campaign, err := p.store.GetCampaignWithSettings(ctx, campaignID)
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
	PrivacyPolicyURL *string
	TermsURL         *string
	MaxSignups       *int

	// Settings
	EmailSettings        *EmailSettingsInput
	BrandingSettings     *BrandingSettingsInput
	FormSettings         *FormSettingsInput
	ReferralSettings     *ReferralSettingsInput
	FormFields           []FormFieldInput
	ShareMessages        []ShareMessageInput
	TrackingIntegrations []TrackingIntegrationInput
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
		PrivacyPolicyURL: req.PrivacyPolicyURL,
		TermsURL:         req.TermsURL,
		MaxSignups:       req.MaxSignups,
	}

	_, err := p.store.UpdateCampaign(ctx, accountID, campaignID, params)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Campaign{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to update campaign", err)
		return store.Campaign{}, err
	}

	// Update settings using CreateCampaignRequest format for reuse
	settingsReq := CreateCampaignRequest{
		EmailSettings:        req.EmailSettings,
		BrandingSettings:     req.BrandingSettings,
		FormSettings:         req.FormSettings,
		ReferralSettings:     req.ReferralSettings,
		FormFields:           req.FormFields,
		ShareMessages:        req.ShareMessages,
		TrackingIntegrations: req.TrackingIntegrations,
	}

	if err := p.createCampaignSettings(ctx, campaignID, settingsReq); err != nil {
		p.logger.Error(ctx, "failed to update campaign settings", err)
		return store.Campaign{}, err
	}

	// Load campaign with settings
	campaignWithSettings, err := p.store.GetCampaignWithSettings(ctx, campaignID)
	if err != nil {
		p.logger.Error(ctx, "failed to load campaign with settings", err)
		return store.Campaign{}, err
	}

	p.logger.Info(ctx, "campaign updated successfully")
	return campaignWithSettings, nil
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

	_, err := p.store.UpdateCampaignStatus(ctx, accountID, campaignID, status)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Campaign{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to update campaign status", err)
		return store.Campaign{}, err
	}

	// Load campaign with settings
	campaign, err := p.store.GetCampaignWithSettings(ctx, campaignID)
	if err != nil {
		p.logger.Error(ctx, "failed to load campaign with settings", err)
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
