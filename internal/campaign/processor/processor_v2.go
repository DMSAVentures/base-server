package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"

	"github.com/google/uuid"
)

// FormConfigRequest represents form configuration in API requests
type FormConfigRequest struct {
	CaptchaEnabled *bool                `json:"captcha_enabled,omitempty"`
	DoubleOptIn    *bool                `json:"double_opt_in,omitempty"`
	CustomCSS      *string              `json:"custom_css,omitempty"`
	Fields         []FormFieldRequest   `json:"fields,omitempty"`
}

// FormFieldRequest represents a form field in API requests
type FormFieldRequest struct {
	Name         string       `json:"name"`
	Type         string       `json:"type"`
	Label        string       `json:"label"`
	Placeholder  *string      `json:"placeholder,omitempty"`
	Required     bool         `json:"required"`
	Options      []string     `json:"options,omitempty"`
	Validation   store.JSONB  `json:"validation,omitempty"`
	DisplayOrder int          `json:"display_order"`
}

// ReferralConfigRequest represents referral configuration in API requests
type ReferralConfigRequest struct {
	Enabled             *bool        `json:"enabled,omitempty"`
	PointsPerReferral   *int         `json:"points_per_referral,omitempty"`
	VerifiedOnly        *bool        `json:"verified_only,omitempty"`
	SharingChannels     []string     `json:"sharing_channels,omitempty"`
	CustomShareMessages store.JSONB  `json:"custom_share_messages,omitempty"`
}

// EmailConfigRequest represents email configuration in API requests
type EmailConfigRequest struct {
	FromName             *string `json:"from_name,omitempty"`
	FromEmail            *string `json:"from_email,omitempty"`
	ReplyTo              *string `json:"reply_to,omitempty"`
	VerificationRequired *bool   `json:"verification_required,omitempty"`
}

// BrandingConfigRequest represents branding configuration in API requests
type BrandingConfigRequest struct {
	LogoURL      *string `json:"logo_url,omitempty"`
	PrimaryColor *string `json:"primary_color,omitempty"`
	FontFamily   *string `json:"font_family,omitempty"`
	CustomDomain *string `json:"custom_domain,omitempty"`
}

// CreateCampaignRequestV2 represents a request to create a campaign (new version with normalized configs)
type CreateCampaignRequestV2 struct {
	Name             string
	Slug             string
	Description      *string
	Type             string
	FormConfig       FormConfigRequest
	ReferralConfig   ReferralConfigRequest
	EmailConfig      EmailConfigRequest
	BrandingConfig   BrandingConfigRequest
	PrivacyPolicyURL *string
	TermsURL         *string
	MaxSignups       *int
}

// UpdateCampaignRequestV2 represents a request to update a campaign (new version with normalized configs)
type UpdateCampaignRequestV2 struct {
	Name             *string
	Description      *string
	LaunchDate       *string
	EndDate          *string
	FormConfig       *FormConfigRequest
	ReferralConfig   *ReferralConfigRequest
	EmailConfig      *EmailConfigRequest
	BrandingConfig   *BrandingConfigRequest
	PrivacyPolicyURL *string
	TermsURL         *string
	MaxSignups       *int
}

// CreateCampaignV2 creates a new campaign with normalized configs
func (p *CampaignProcessor) CreateCampaignV2(ctx context.Context, accountID uuid.UUID, req CreateCampaignRequestV2) (store.Campaign, error) {
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

	// Convert form fields
	formFields := make([]store.CreateFormFieldParams, len(req.FormConfig.Fields))
	for i, field := range req.FormConfig.Fields {
		formFields[i] = store.CreateFormFieldParams{
			Name:         field.Name,
			Type:         field.Type,
			Label:        field.Label,
			Placeholder:  field.Placeholder,
			Required:     field.Required,
			Options:      field.Options,
			Validation:   field.Validation,
			DisplayOrder: field.DisplayOrder,
		}
	}

	// Build store params with defaults
	params := store.CreateCampaignParams{
		AccountID:   accountID,
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		Type:        req.Type,
		FormConfig: store.CreateCampaignFormConfigParams{
			CaptchaEnabled: valueOrDefault(req.FormConfig.CaptchaEnabled, false),
			DoubleOptIn:    valueOrDefault(req.FormConfig.DoubleOptIn, true),
			CustomCSS:      req.FormConfig.CustomCSS,
			Fields:         formFields,
		},
		ReferralConfig: store.CreateCampaignReferralConfigParams{
			Enabled:             valueOrDefault(req.ReferralConfig.Enabled, true),
			PointsPerReferral:   valueOrDefault(req.ReferralConfig.PointsPerReferral, 1),
			VerifiedOnly:        valueOrDefault(req.ReferralConfig.VerifiedOnly, true),
			SharingChannels:     getDefaultSharingChannels(req.ReferralConfig.SharingChannels),
			CustomShareMessages: getDefaultJSONB(req.ReferralConfig.CustomShareMessages),
		},
		EmailConfig: store.CreateCampaignEmailConfigParams{
			FromName:             req.EmailConfig.FromName,
			FromEmail:            req.EmailConfig.FromEmail,
			ReplyTo:              req.EmailConfig.ReplyTo,
			VerificationRequired: valueOrDefault(req.EmailConfig.VerificationRequired, true),
		},
		BrandingConfig: store.CreateCampaignBrandingConfigParams{
			LogoURL:      req.BrandingConfig.LogoURL,
			PrimaryColor: valueOrDefault(req.BrandingConfig.PrimaryColor, "#2563EB"),
			FontFamily:   valueOrDefault(req.BrandingConfig.FontFamily, "Inter"),
			CustomDomain: req.BrandingConfig.CustomDomain,
		},
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

// UpdateCampaignV2 updates a campaign with normalized configs
func (p *CampaignProcessor) UpdateCampaignV2(ctx context.Context, accountID, campaignID uuid.UUID, req UpdateCampaignRequestV2) (store.Campaign, error) {
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

	// Convert form config if provided
	if req.FormConfig != nil {
		params.FormConfig = &store.UpdateCampaignFormConfigParams{
			CaptchaEnabled: req.FormConfig.CaptchaEnabled,
			DoubleOptIn:    req.FormConfig.DoubleOptIn,
			CustomCSS:      req.FormConfig.CustomCSS,
		}
	}

	// Convert referral config if provided
	if req.ReferralConfig != nil {
		sharingChannels := req.ReferralConfig.SharingChannels
		if len(sharingChannels) == 0 {
			sharingChannels = nil // Don't update if empty
		}
		params.ReferralConfig = &store.UpdateCampaignReferralConfigParams{
			Enabled:             req.ReferralConfig.Enabled,
			PointsPerReferral:   req.ReferralConfig.PointsPerReferral,
			VerifiedOnly:        req.ReferralConfig.VerifiedOnly,
			SharingChannels:     sharingChannels,
			CustomShareMessages: req.ReferralConfig.CustomShareMessages,
		}
	}

	// Convert email config if provided
	if req.EmailConfig != nil {
		params.EmailConfig = &store.UpdateCampaignEmailConfigParams{
			FromName:             req.EmailConfig.FromName,
			FromEmail:            req.EmailConfig.FromEmail,
			ReplyTo:              req.EmailConfig.ReplyTo,
			VerificationRequired: req.EmailConfig.VerificationRequired,
		}
	}

	// Convert branding config if provided
	if req.BrandingConfig != nil {
		params.BrandingConfig = &store.UpdateCampaignBrandingConfigParams{
			LogoURL:      req.BrandingConfig.LogoURL,
			PrimaryColor: req.BrandingConfig.PrimaryColor,
			FontFamily:   req.BrandingConfig.FontFamily,
			CustomDomain: req.BrandingConfig.CustomDomain,
		}
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

// Helper functions

func valueOrDefault[T any](value *T, defaultValue T) T {
	if value != nil {
		return *value
	}
	return defaultValue
}

func getDefaultSharingChannels(channels []string) []string {
	if len(channels) > 0 {
		return channels
	}
	return []string{"email", "twitter", "facebook", "linkedin", "whatsapp"}
}

func getDefaultJSONB(jsonb store.JSONB) store.JSONB {
	if jsonb != nil && len(jsonb) > 0 {
		return jsonb
	}
	return store.JSONB{}
}
