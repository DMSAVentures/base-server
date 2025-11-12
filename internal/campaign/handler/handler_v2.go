package handler

import (
	"base-server/internal/apierrors"
	"base-server/internal/campaign/processor"
	"base-server/internal/observability"
	"base-server/internal/store"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreateCampaignRequestV2 represents the HTTP request for creating a campaign (normalized configs)
type CreateCampaignRequestV2 struct {
	Name             string                           `json:"name" binding:"required,min=1,max=255"`
	Slug             string                           `json:"slug" binding:"required,min=1,max=255"`
	Description      *string                          `json:"description,omitempty"`
	Type             string                           `json:"type" binding:"required,oneof=waitlist referral contest"`
	FormConfig       *FormConfigRequest               `json:"form_config,omitempty"`
	ReferralConfig   *ReferralConfigRequest           `json:"referral_config,omitempty"`
	EmailConfig      *EmailConfigRequest              `json:"email_config,omitempty"`
	BrandingConfig   *BrandingConfigRequest           `json:"branding_config,omitempty"`
	PrivacyPolicyURL *string                          `json:"privacy_policy_url,omitempty"`
	TermsURL         *string                          `json:"terms_url,omitempty"`
	MaxSignups       *int                             `json:"max_signups,omitempty"`
}

// FormConfigRequest represents form configuration in HTTP requests
type FormConfigRequest struct {
	CaptchaEnabled *bool              `json:"captcha_enabled,omitempty"`
	DoubleOptIn    *bool              `json:"double_opt_in,omitempty"`
	CustomCSS      *string            `json:"custom_css,omitempty"`
	Fields         []FormFieldRequest `json:"fields,omitempty"`
}

// FormFieldRequest represents a form field in HTTP requests
type FormFieldRequest struct {
	Name         string       `json:"name" binding:"required"`
	Type         string       `json:"type" binding:"required,oneof=email text select checkbox textarea number"`
	Label        string       `json:"label" binding:"required"`
	Placeholder  *string      `json:"placeholder,omitempty"`
	Required     bool         `json:"required"`
	Options      []string     `json:"options,omitempty"`
	Validation   store.JSONB  `json:"validation,omitempty"`
	DisplayOrder int          `json:"display_order"`
}

// ReferralConfigRequest represents referral configuration in HTTP requests
type ReferralConfigRequest struct {
	Enabled             *bool        `json:"enabled,omitempty"`
	PointsPerReferral   *int         `json:"points_per_referral,omitempty"`
	VerifiedOnly        *bool        `json:"verified_only,omitempty"`
	SharingChannels     []string     `json:"sharing_channels,omitempty"`
	CustomShareMessages store.JSONB  `json:"custom_share_messages,omitempty"`
}

// EmailConfigRequest represents email configuration in HTTP requests
type EmailConfigRequest struct {
	FromName             *string `json:"from_name,omitempty"`
	FromEmail            *string `json:"from_email,omitempty"`
	ReplyTo              *string `json:"reply_to,omitempty"`
	VerificationRequired *bool   `json:"verification_required,omitempty"`
}

// BrandingConfigRequest represents branding configuration in HTTP requests
type BrandingConfigRequest struct {
	LogoURL      *string `json:"logo_url,omitempty"`
	PrimaryColor *string `json:"primary_color,omitempty"`
	FontFamily   *string `json:"font_family,omitempty"`
	CustomDomain *string `json:"custom_domain,omitempty"`
}

// UpdateCampaignRequestV2 represents the HTTP request for updating a campaign (normalized configs)
type UpdateCampaignRequestV2 struct {
	Name             *string                `json:"name,omitempty"`
	Description      *string                `json:"description,omitempty"`
	LaunchDate       *string                `json:"launch_date,omitempty"`
	EndDate          *string                `json:"end_date,omitempty"`
	FormConfig       *FormConfigRequest     `json:"form_config,omitempty"`
	ReferralConfig   *ReferralConfigRequest `json:"referral_config,omitempty"`
	EmailConfig      *EmailConfigRequest    `json:"email_config,omitempty"`
	BrandingConfig   *BrandingConfigRequest `json:"branding_config,omitempty"`
	PrivacyPolicyURL *string                `json:"privacy_policy_url,omitempty"`
	TermsURL         *string                `json:"terms_url,omitempty"`
	MaxSignups       *int                   `json:"max_signups,omitempty"`
}

// HandleCreateCampaignV2 creates a new campaign with normalized configs
func (h *Handler) HandleCreateCampaignV2(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, apierrors.Unauthorized("Account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		apierrors.RespondWithError(c, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid account ID format"))
		return
	}

	// Add account_id to observability context for comprehensive logging
	ctx = observability.WithFields(ctx, observability.Field{Key: "account_id", Value: accountID.String()})

	var req CreateCampaignRequestV2
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	// Add campaign identifiers to observability context for comprehensive logging
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_slug", Value: req.Slug},
		observability.Field{Key: "campaign_type", Value: req.Type},
	)

	// Set defaults if not provided
	formConfig := processor.FormConfigRequest{}
	if req.FormConfig != nil {
		formConfig = convertToProcessorFormConfig(*req.FormConfig)
	}

	referralConfig := processor.ReferralConfigRequest{}
	if req.ReferralConfig != nil {
		referralConfig = convertToProcessorReferralConfig(*req.ReferralConfig)
	}

	emailConfig := processor.EmailConfigRequest{}
	if req.EmailConfig != nil {
		emailConfig = convertToProcessorEmailConfig(*req.EmailConfig)
	}

	brandingConfig := processor.BrandingConfigRequest{}
	if req.BrandingConfig != nil {
		brandingConfig = convertToProcessorBrandingConfig(*req.BrandingConfig)
	}

	processorReq := processor.CreateCampaignRequestV2{
		Name:             req.Name,
		Slug:             req.Slug,
		Description:      req.Description,
		Type:             req.Type,
		FormConfig:       formConfig,
		ReferralConfig:   referralConfig,
		EmailConfig:      emailConfig,
		BrandingConfig:   brandingConfig,
		PrivacyPolicyURL: req.PrivacyPolicyURL,
		TermsURL:         req.TermsURL,
		MaxSignups:       req.MaxSignups,
	}

	campaign, err := h.processor.CreateCampaignV2(ctx, accountID, processorReq)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusCreated, campaign)
}

// HandleUpdateCampaignV2 updates a campaign with normalized configs
func (h *Handler) HandleUpdateCampaignV2(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, apierrors.Unauthorized("Account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		apierrors.RespondWithError(c, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid account ID format"))
		return
	}

	// Add account_id to observability context for comprehensive logging
	ctx = observability.WithFields(ctx, observability.Field{Key: "account_id", Value: accountID.String()})

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		apierrors.RespondWithError(c, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid campaign ID format"))
		return
	}

	// Add campaign_id to observability context for comprehensive logging
	ctx = observability.WithFields(ctx, observability.Field{Key: "campaign_id", Value: campaignID.String()})

	var req UpdateCampaignRequestV2
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	processorReq := processor.UpdateCampaignRequestV2{
		Name:             req.Name,
		Description:      req.Description,
		LaunchDate:       req.LaunchDate,
		EndDate:          req.EndDate,
		PrivacyPolicyURL: req.PrivacyPolicyURL,
		TermsURL:         req.TermsURL,
		MaxSignups:       req.MaxSignups,
	}

	// Convert configs if provided
	if req.FormConfig != nil {
		converted := convertToProcessorFormConfig(*req.FormConfig)
		processorReq.FormConfig = &converted
	}

	if req.ReferralConfig != nil {
		converted := convertToProcessorReferralConfig(*req.ReferralConfig)
		processorReq.ReferralConfig = &converted
	}

	if req.EmailConfig != nil {
		converted := convertToProcessorEmailConfig(*req.EmailConfig)
		processorReq.EmailConfig = &converted
	}

	if req.BrandingConfig != nil {
		converted := convertToProcessorBrandingConfig(*req.BrandingConfig)
		processorReq.BrandingConfig = &converted
	}

	campaign, err := h.processor.UpdateCampaignV2(ctx, accountID, campaignID, processorReq)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, campaign)
}

// Conversion helper functions

func convertToProcessorFormConfig(req FormConfigRequest) processor.FormConfigRequest {
	fields := make([]processor.FormFieldRequest, len(req.Fields))
	for i, field := range req.Fields {
		fields[i] = processor.FormFieldRequest{
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

	return processor.FormConfigRequest{
		CaptchaEnabled: req.CaptchaEnabled,
		DoubleOptIn:    req.DoubleOptIn,
		CustomCSS:      req.CustomCSS,
		Fields:         fields,
	}
}

func convertToProcessorReferralConfig(req ReferralConfigRequest) processor.ReferralConfigRequest {
	return processor.ReferralConfigRequest{
		Enabled:             req.Enabled,
		PointsPerReferral:   req.PointsPerReferral,
		VerifiedOnly:        req.VerifiedOnly,
		SharingChannels:     req.SharingChannels,
		CustomShareMessages: req.CustomShareMessages,
	}
}

func convertToProcessorEmailConfig(req EmailConfigRequest) processor.EmailConfigRequest {
	return processor.EmailConfigRequest{
		FromName:             req.FromName,
		FromEmail:            req.FromEmail,
		ReplyTo:              req.ReplyTo,
		VerificationRequired: req.VerificationRequired,
	}
}

func convertToProcessorBrandingConfig(req BrandingConfigRequest) processor.BrandingConfigRequest {
	return processor.BrandingConfigRequest{
		LogoURL:      req.LogoURL,
		PrimaryColor: req.PrimaryColor,
		FontFamily:   req.FontFamily,
		CustomDomain: req.CustomDomain,
	}
}
