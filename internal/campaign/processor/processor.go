package processor

//go:generate go run go.uber.org/mock/mockgen@latest -source=processor.go -destination=mocks_test.go -package=processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"base-server/internal/tiers"
	"context"
	"errors"

	"github.com/google/uuid"
)

// CampaignStore defines the database operations required by CampaignProcessor
type CampaignStore interface {
	// Campaign CRUD
	GetCampaignBySlug(ctx context.Context, accountID uuid.UUID, slug string) (store.Campaign, error)
	CreateCampaign(ctx context.Context, params store.CreateCampaignParams) (store.Campaign, error)
	GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (store.Campaign, error)
	GetCampaignWithSettings(ctx context.Context, campaignID uuid.UUID) (store.Campaign, error)
	ListCampaigns(ctx context.Context, params store.ListCampaignsParams) (store.ListCampaignsResult, error)
	LoadCampaignsSettings(ctx context.Context, campaigns []store.Campaign) ([]store.Campaign, error)
	UpdateCampaign(ctx context.Context, accountID, campaignID uuid.UUID, params store.UpdateCampaignParams) (store.Campaign, error)
	UpdateCampaignStatus(ctx context.Context, accountID, campaignID uuid.UUID, status string) (store.Campaign, error)
	DeleteCampaign(ctx context.Context, accountID, campaignID uuid.UUID) error
	CountCampaignsByAccountID(ctx context.Context, accountID uuid.UUID) (int, error)

	// Email Settings
	UpsertCampaignEmailSettings(ctx context.Context, params store.CreateCampaignEmailSettingsParams) (store.CampaignEmailSettings, error)
	GetCampaignEmailSettings(ctx context.Context, campaignID uuid.UUID) (store.CampaignEmailSettings, error)

	// Branding Settings
	UpsertCampaignBrandingSettings(ctx context.Context, params store.CreateCampaignBrandingSettingsParams) (store.CampaignBrandingSettings, error)
	GetCampaignBrandingSettings(ctx context.Context, campaignID uuid.UUID) (store.CampaignBrandingSettings, error)

	// Form Settings
	UpsertCampaignFormSettings(ctx context.Context, params store.CreateCampaignFormSettingsParams) (store.CampaignFormSettings, error)
	GetCampaignFormSettings(ctx context.Context, campaignID uuid.UUID) (store.CampaignFormSettings, error)

	// Referral Settings
	UpsertCampaignReferralSettings(ctx context.Context, params store.CreateCampaignReferralSettingsParams) (store.CampaignReferralSettings, error)
	GetCampaignReferralSettings(ctx context.Context, campaignID uuid.UUID) (store.CampaignReferralSettings, error)

	// Form Fields
	ReplaceCampaignFormFields(ctx context.Context, campaignID uuid.UUID, fields []store.CreateCampaignFormFieldParams) ([]store.CampaignFormField, error)
	GetCampaignFormFields(ctx context.Context, campaignID uuid.UUID) ([]store.CampaignFormField, error)

	// Share Messages
	ReplaceCampaignShareMessages(ctx context.Context, campaignID uuid.UUID, messages []store.CreateCampaignShareMessageParams) ([]store.CampaignShareMessage, error)
	GetCampaignShareMessages(ctx context.Context, campaignID uuid.UUID) ([]store.CampaignShareMessage, error)

	// Tracking Integrations
	ReplaceCampaignTrackingIntegrations(ctx context.Context, campaignID uuid.UUID, integrations []store.CreateCampaignTrackingIntegrationParams) ([]store.CampaignTrackingIntegration, error)
	GetCampaignTrackingIntegrations(ctx context.Context, campaignID uuid.UUID) ([]store.CampaignTrackingIntegration, error)
}

var (
	ErrCampaignNotFound      = errors.New("campaign not found")
	ErrSlugAlreadyExists     = errors.New("slug already exists")
	ErrInvalidCampaignStatus = errors.New("invalid campaign status")
	ErrInvalidCampaignType   = errors.New("invalid campaign type")
	ErrUnauthorized          = errors.New("unauthorized access to campaign")
	ErrCampaignLimitReached  = errors.New("campaign limit reached for your plan")
)

type CampaignProcessor struct {
	store       CampaignStore
	tierService *tiers.TierService
	logger      *observability.Logger
}

func New(store CampaignStore, tierService *tiers.TierService, logger *observability.Logger) CampaignProcessor {
	return CampaignProcessor{
		store:       store,
		tierService: tierService,
		logger:      logger,
	}
}

// CampaignSettingsParams contains all campaign settings for create/update operations
type CampaignSettingsParams struct {
	EmailSettings        *EmailSettingsParams
	BrandingSettings     *BrandingSettingsParams
	FormSettings         *FormSettingsParams
	ReferralSettings     *ReferralSettingsParams
	FormFields           []FormFieldParams
	ShareMessages        []ShareMessageParams
	TrackingIntegrations []TrackingIntegrationParams
}

// CreateCampaignParams represents parameters for creating a campaign
type CreateCampaignParams struct {
	Name             string
	Slug             string
	Description      *string
	Type             string
	PrivacyPolicyURL *string
	TermsURL         *string
	MaxSignups       *int
	Settings         CampaignSettingsParams
}

// EmailSettingsParams represents email settings parameters
type EmailSettingsParams struct {
	FromName             *string
	FromEmail            *string
	ReplyTo              *string
	VerificationRequired bool
	SendWelcomeEmail     bool
}

// BrandingSettingsParams represents branding settings parameters
type BrandingSettingsParams struct {
	LogoURL      *string
	PrimaryColor *string
	FontFamily   *string
	CustomDomain *string
}

// FormSettingsParams represents form settings parameters
type FormSettingsParams struct {
	CaptchaEnabled  bool
	CaptchaProvider *string
	CaptchaSiteKey  *string
	DoubleOptIn     bool
	Design          map[string]any
	SuccessTitle    *string
	SuccessMessage  *string
}

// ReferralSettingsParams represents referral settings parameters
type ReferralSettingsParams struct {
	Enabled                 bool
	PointsPerReferral       int
	VerifiedOnly            bool
	PositionsToJump         int
	ReferrerPositionsToJump int
	SharingChannels         []string
}

// FormFieldParams represents a form field parameters
type FormFieldParams struct {
	Name              string
	FieldType         string
	Label             string
	Placeholder       *string
	Required          bool
	ValidationPattern *string
	Options           []string
	DisplayOrder      int
}

// ShareMessageParams represents a share message parameters
type ShareMessageParams struct {
	Channel string
	Message string
}

// TrackingIntegrationParams represents a tracking integration parameters
type TrackingIntegrationParams struct {
	IntegrationType string
	Enabled         bool
	TrackingID      string
	TrackingLabel   *string
}

// CreateCampaign creates a new campaign for an account
func (p *CampaignProcessor) CreateCampaign(ctx context.Context, accountID uuid.UUID, params CreateCampaignParams) (store.Campaign, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_slug", Value: params.Slug},
	)

	// Validate campaign type
	if !isValidCampaignType(params.Type) {
		return store.Campaign{}, ErrInvalidCampaignType
	}

	// Check campaign limit
	campaignLimit, err := p.tierService.GetLimitByAccountID(ctx, accountID, "campaigns")
	if err != nil {
		p.logger.Error(ctx, "failed to get campaign limit", err)
		return store.Campaign{}, err
	}

	// If limit is not nil (not unlimited), check against current count
	if campaignLimit != nil {
		currentCount, err := p.store.CountCampaignsByAccountID(ctx, accountID)
		if err != nil {
			p.logger.Error(ctx, "failed to count campaigns", err)
			return store.Campaign{}, err
		}

		if currentCount >= *campaignLimit {
			ctx = observability.WithFields(ctx,
				observability.Field{Key: "current_count", Value: currentCount},
				observability.Field{Key: "limit", Value: *campaignLimit},
			)
			p.logger.Warn(ctx, "campaign limit reached")
			return store.Campaign{}, ErrCampaignLimitReached
		}
	}

	// Check if slug already exists for this account
	existingCampaign, err := p.store.GetCampaignBySlug(ctx, accountID, params.Slug)
	if err == nil && existingCampaign.ID != uuid.Nil {
		return store.Campaign{}, ErrSlugAlreadyExists
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		p.logger.Error(ctx, "failed to check slug existence", err)
		return store.Campaign{}, err
	}

	storeParams := store.CreateCampaignParams{
		AccountID:        accountID,
		Name:             params.Name,
		Slug:             params.Slug,
		Description:      params.Description,
		Type:             params.Type,
		PrivacyPolicyURL: params.PrivacyPolicyURL,
		TermsURL:         params.TermsURL,
		MaxSignups:       params.MaxSignups,
	}

	campaign, err := p.store.CreateCampaign(ctx, storeParams)
	if err != nil {
		p.logger.Error(ctx, "failed to create campaign", err)
		return store.Campaign{}, err
	}

	// Upsert settings
	if err := p.upsertCampaignSettings(ctx, campaign.ID, params.Settings); err != nil {
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

// upsertCampaignSettings creates or updates all settings for a campaign
func (p *CampaignProcessor) upsertCampaignSettings(ctx context.Context, campaignID uuid.UUID, settings CampaignSettingsParams) error {
	// Upsert email settings
	if settings.EmailSettings != nil {
		_, err := p.store.UpsertCampaignEmailSettings(ctx, store.CreateCampaignEmailSettingsParams{
			CampaignID:           campaignID,
			FromName:             settings.EmailSettings.FromName,
			FromEmail:            settings.EmailSettings.FromEmail,
			ReplyTo:              settings.EmailSettings.ReplyTo,
			VerificationRequired: settings.EmailSettings.VerificationRequired,
			SendWelcomeEmail:     settings.EmailSettings.SendWelcomeEmail,
		})
		if err != nil {
			return err
		}
	}

	// Upsert branding settings
	if settings.BrandingSettings != nil {
		_, err := p.store.UpsertCampaignBrandingSettings(ctx, store.CreateCampaignBrandingSettingsParams{
			CampaignID:   campaignID,
			LogoURL:      settings.BrandingSettings.LogoURL,
			PrimaryColor: settings.BrandingSettings.PrimaryColor,
			FontFamily:   settings.BrandingSettings.FontFamily,
			CustomDomain: settings.BrandingSettings.CustomDomain,
		})
		if err != nil {
			return err
		}
	}

	// Upsert form settings
	if settings.FormSettings != nil {
		var captchaProvider *store.CaptchaProvider
		if settings.FormSettings.CaptchaProvider != nil {
			cp := store.CaptchaProvider(*settings.FormSettings.CaptchaProvider)
			captchaProvider = &cp
		}
		// Ensure Design is never nil (database column has NOT NULL constraint)
		design := store.JSONB{}
		if settings.FormSettings.Design != nil {
			design = store.JSONB(settings.FormSettings.Design)
		}
		_, err := p.store.UpsertCampaignFormSettings(ctx, store.CreateCampaignFormSettingsParams{
			CampaignID:      campaignID,
			CaptchaEnabled:  settings.FormSettings.CaptchaEnabled,
			CaptchaProvider: captchaProvider,
			CaptchaSiteKey:  settings.FormSettings.CaptchaSiteKey,
			DoubleOptIn:     settings.FormSettings.DoubleOptIn,
			Design:          design,
			SuccessTitle:    settings.FormSettings.SuccessTitle,
			SuccessMessage:  settings.FormSettings.SuccessMessage,
		})
		if err != nil {
			return err
		}
	}

	// Upsert referral settings
	if settings.ReferralSettings != nil {
		sharingChannels := make([]store.SharingChannel, len(settings.ReferralSettings.SharingChannels))
		for i, ch := range settings.ReferralSettings.SharingChannels {
			sharingChannels[i] = store.SharingChannel(ch)
		}
		_, err := p.store.UpsertCampaignReferralSettings(ctx, store.CreateCampaignReferralSettingsParams{
			CampaignID:              campaignID,
			Enabled:                 settings.ReferralSettings.Enabled,
			PointsPerReferral:       settings.ReferralSettings.PointsPerReferral,
			VerifiedOnly:            settings.ReferralSettings.VerifiedOnly,
			PositionsToJump:         settings.ReferralSettings.PositionsToJump,
			ReferrerPositionsToJump: settings.ReferralSettings.ReferrerPositionsToJump,
			SharingChannels:         sharingChannels,
		})
		if err != nil {
			return err
		}
	}

	// Replace form fields
	if len(settings.FormFields) > 0 {
		fields := make([]store.CreateCampaignFormFieldParams, len(settings.FormFields))
		for i, f := range settings.FormFields {
			fields[i] = store.CreateCampaignFormFieldParams{
				CampaignID:        campaignID,
				Name:              f.Name,
				FieldType:         store.FormFieldType(f.FieldType),
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

	// Replace share messages
	if len(settings.ShareMessages) > 0 {
		messages := make([]store.CreateCampaignShareMessageParams, len(settings.ShareMessages))
		for i, m := range settings.ShareMessages {
			messages[i] = store.CreateCampaignShareMessageParams{
				CampaignID: campaignID,
				Channel:    store.SharingChannel(m.Channel),
				Message:    m.Message,
			}
		}
		_, err := p.store.ReplaceCampaignShareMessages(ctx, campaignID, messages)
		if err != nil {
			return err
		}
	}

	// Replace tracking integrations
	if len(settings.TrackingIntegrations) > 0 {
		integrations := make([]store.CreateCampaignTrackingIntegrationParams, len(settings.TrackingIntegrations))
		for i, t := range settings.TrackingIntegrations {
			integrations[i] = store.CreateCampaignTrackingIntegrationParams{
				CampaignID:      campaignID,
				IntegrationType: store.TrackingIntegrationType(t.IntegrationType),
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

// UpdateCampaignParams represents parameters for updating a campaign
type UpdateCampaignParams struct {
	Name             *string
	Description      *string
	LaunchDate       *string
	EndDate          *string
	PrivacyPolicyURL *string
	TermsURL         *string
	MaxSignups       *int
	Settings         CampaignSettingsParams
}

// UpdateCampaign updates a campaign
func (p *CampaignProcessor) UpdateCampaign(ctx context.Context, accountID, campaignID uuid.UUID, params UpdateCampaignParams) (store.Campaign, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	storeParams := store.UpdateCampaignParams{
		Name:             params.Name,
		Description:      params.Description,
		PrivacyPolicyURL: params.PrivacyPolicyURL,
		TermsURL:         params.TermsURL,
		MaxSignups:       params.MaxSignups,
	}

	_, err := p.store.UpdateCampaign(ctx, accountID, campaignID, storeParams)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Campaign{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to update campaign", err)
		return store.Campaign{}, err
	}

	// Upsert settings
	if err := p.upsertCampaignSettings(ctx, campaignID, params.Settings); err != nil {
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
