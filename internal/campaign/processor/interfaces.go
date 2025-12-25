package processor

import (
	"base-server/internal/store"
	"context"

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
