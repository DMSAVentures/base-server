package handler

import (
	"errors"
	"fmt"
	"net/http"

	"base-server/internal/apierrors"
	"base-server/internal/campaign/processor"
	"base-server/internal/observability"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	processor processor.CampaignProcessor
	logger    *observability.Logger
}

func New(processor processor.CampaignProcessor, logger *observability.Logger) Handler {
	return Handler{
		processor: processor,
		logger:    logger,
	}
}

// EmailSettingsRequest represents email settings in HTTP request
type EmailSettingsRequest struct {
	FromName             *string `json:"from_name,omitempty"`
	FromEmail            *string `json:"from_email,omitempty"`
	ReplyTo              *string `json:"reply_to,omitempty"`
	VerificationRequired bool    `json:"verification_required"`
	SendWelcomeEmail     bool    `json:"send_welcome_email"`
}

// BrandingSettingsRequest represents branding settings in HTTP request
type BrandingSettingsRequest struct {
	LogoURL      *string `json:"logo_url,omitempty"`
	PrimaryColor *string `json:"primary_color,omitempty"`
	FontFamily   *string `json:"font_family,omitempty"`
	CustomDomain *string `json:"custom_domain,omitempty"`
}

// FormSettingsRequest represents form settings in HTTP request
type FormSettingsRequest struct {
	CaptchaEnabled  bool           `json:"captcha_enabled"`
	CaptchaProvider *string        `json:"captcha_provider,omitempty" binding:"omitempty,oneof=turnstile recaptcha hcaptcha"`
	CaptchaSiteKey  *string        `json:"captcha_site_key,omitempty"`
	DoubleOptIn     bool           `json:"double_opt_in"`
	Design          map[string]any `json:"design"`
	SuccessTitle    *string        `json:"success_title,omitempty"`
	SuccessMessage  *string        `json:"success_message,omitempty"`
}

// ReferralSettingsRequest represents referral settings in HTTP request
type ReferralSettingsRequest struct {
	Enabled                 bool     `json:"enabled"`
	PointsPerReferral       int      `json:"points_per_referral" binding:"gte=0"`
	VerifiedOnly            bool     `json:"verified_only"`
	PositionsToJump         int      `json:"positions_to_jump" binding:"gte=0"`
	ReferrerPositionsToJump int      `json:"referrer_positions_to_jump" binding:"gte=0"`
	SharingChannels         []string `json:"sharing_channels" binding:"dive,oneof=email twitter facebook linkedin whatsapp"`
}

// FormFieldRequest represents a form field in HTTP request
type FormFieldRequest struct {
	Name              string   `json:"name" binding:"required,min=1"`
	FieldType         string   `json:"field_type" binding:"required,oneof=email text textarea select checkbox radio phone url date number"`
	Label             string   `json:"label" binding:"required,min=1"`
	Placeholder       *string  `json:"placeholder,omitempty"`
	Required          bool     `json:"required"`
	ValidationPattern *string  `json:"validation_pattern,omitempty"`
	Options           []string `json:"options,omitempty"`
	DisplayOrder      int      `json:"display_order"`
}

// ShareMessageRequest represents a share message in HTTP request
type ShareMessageRequest struct {
	Channel string `json:"channel" binding:"required,oneof=email twitter facebook linkedin whatsapp"`
	Message string `json:"message" binding:"required,min=1"`
}

// TrackingIntegrationRequest represents a tracking integration in HTTP request
type TrackingIntegrationRequest struct {
	IntegrationType string  `json:"integration_type" binding:"required,oneof=google_analytics meta_pixel google_ads tiktok_pixel linkedin_insight"`
	Enabled         bool    `json:"enabled"`
	TrackingID      string  `json:"tracking_id" binding:"required,min=1"`
	TrackingLabel   *string `json:"tracking_label,omitempty"`
}

// CreateCampaignRequest represents the HTTP request for creating a campaign
type CreateCampaignRequest struct {
	Name             string  `json:"name" binding:"required,min=1,max=255"`
	Slug             string  `json:"slug" binding:"required,min=1,max=255"`
	Description      *string `json:"description,omitempty"`
	Type             string  `json:"type" binding:"required,oneof=waitlist referral contest"`
	PrivacyPolicyURL *string `json:"privacy_policy_url,omitempty"`
	TermsURL         *string `json:"terms_url,omitempty"`
	MaxSignups       *int    `json:"max_signups,omitempty"`

	// Settings
	EmailSettings        *EmailSettingsRequest        `json:"email_settings,omitempty"`
	BrandingSettings     *BrandingSettingsRequest     `json:"branding_settings,omitempty"`
	FormSettings         *FormSettingsRequest         `json:"form_settings,omitempty"`
	ReferralSettings     *ReferralSettingsRequest     `json:"referral_settings,omitempty"`
	FormFields           []FormFieldRequest           `json:"form_fields,omitempty" binding:"dive"`
	ShareMessages        []ShareMessageRequest        `json:"share_messages,omitempty" binding:"dive"`
	TrackingIntegrations []TrackingIntegrationRequest `json:"tracking_integrations,omitempty" binding:"dive"`
}

// UpdateCampaignRequest represents the HTTP request for updating a campaign
type UpdateCampaignRequest struct {
	Name             *string `json:"name,omitempty"`
	Description      *string `json:"description,omitempty"`
	LaunchDate       *string `json:"launch_date,omitempty"`
	EndDate          *string `json:"end_date,omitempty"`
	PrivacyPolicyURL *string `json:"privacy_policy_url,omitempty"`
	TermsURL         *string `json:"terms_url,omitempty"`
	MaxSignups       *int    `json:"max_signups,omitempty"`

	// Settings
	EmailSettings        *EmailSettingsRequest        `json:"email_settings,omitempty"`
	BrandingSettings     *BrandingSettingsRequest     `json:"branding_settings,omitempty"`
	FormSettings         *FormSettingsRequest         `json:"form_settings,omitempty"`
	ReferralSettings     *ReferralSettingsRequest     `json:"referral_settings,omitempty"`
	FormFields           []FormFieldRequest           `json:"form_fields,omitempty" binding:"dive"`
	ShareMessages        []ShareMessageRequest        `json:"share_messages,omitempty" binding:"dive"`
	TrackingIntegrations []TrackingIntegrationRequest `json:"tracking_integrations,omitempty" binding:"dive"`
}

// UpdateCampaignStatusRequest represents the HTTP request for updating campaign status
type UpdateCampaignStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=draft active paused completed"`
}

// HandleCreateCampaign creates a new campaign
func (h *Handler) HandleCreateCampaign(c *gin.Context) {
	ctx := c.Request.Context()

	accountID, ok := h.getAccountID(c)
	if !ok {
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "account_id", Value: accountID.String()})

	var req CreateCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_slug", Value: req.Slug},
		observability.Field{Key: "campaign_type", Value: req.Type},
	)

	params := processor.CreateCampaignParams{
		Name:             req.Name,
		Slug:             req.Slug,
		Description:      req.Description,
		Type:             req.Type,
		PrivacyPolicyURL: req.PrivacyPolicyURL,
		TermsURL:         req.TermsURL,
		MaxSignups:       req.MaxSignups,
		Settings:         convertSettingsRequest(req.EmailSettings, req.BrandingSettings, req.FormSettings, req.ReferralSettings, req.FormFields, req.ShareMessages, req.TrackingIntegrations),
	}

	campaign, err := h.processor.CreateCampaign(ctx, accountID, params)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, campaign)
}

// HandleListCampaigns lists all campaigns for the account
func (h *Handler) HandleListCampaigns(c *gin.Context) {
	ctx := c.Request.Context()

	accountID, ok := h.getAccountID(c)
	if !ok {
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "account_id", Value: accountID.String()})

	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if _, err := fmt.Sscanf(pageStr, "%d", &page); err != nil || page < 1 {
			page = 1
		}
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil || limit < 1 || limit > 100 {
			limit = 20
		}
	}

	var status *string
	if statusStr := c.Query("status"); statusStr != "" {
		status = &statusStr
	}

	var campaignType *string
	if typeStr := c.Query("type"); typeStr != "" {
		campaignType = &typeStr
	}

	result, err := h.processor.ListCampaigns(ctx, accountID, status, campaignType, page, limit)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"campaigns": result.Campaigns,
		"pagination": gin.H{
			"total_count": result.TotalCount,
			"page":        result.Page,
			"page_size":   result.Limit,
			"total_pages": result.TotalPages,
		},
	})
}

// HandleGetCampaign retrieves a campaign by ID (authenticated)
func (h *Handler) HandleGetCampaign(c *gin.Context) {
	ctx := c.Request.Context()

	accountID, ok := h.getAccountID(c)
	if !ok {
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "account_id", Value: accountID.String()})

	campaignID, ok := h.getCampaignID(c)
	if !ok {
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "campaign_id", Value: campaignID.String()})

	campaign, err := h.processor.GetCampaign(ctx, accountID, campaignID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, campaign)
}

// HandleGetPublicCampaign retrieves a campaign by ID without authentication (for public form rendering)
func (h *Handler) HandleGetPublicCampaign(c *gin.Context) {
	ctx := c.Request.Context()

	campaignID, ok := h.getCampaignID(c)
	if !ok {
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "campaign_id", Value: campaignID.String()})

	campaign, err := h.processor.GetPublicCampaign(ctx, campaignID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, campaign)
}

// HandleUpdateCampaign updates a campaign
func (h *Handler) HandleUpdateCampaign(c *gin.Context) {
	ctx := c.Request.Context()

	accountID, ok := h.getAccountID(c)
	if !ok {
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "account_id", Value: accountID.String()})

	campaignID, ok := h.getCampaignID(c)
	if !ok {
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "campaign_id", Value: campaignID.String()})

	var req UpdateCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	params := processor.UpdateCampaignParams{
		Name:             req.Name,
		Description:      req.Description,
		LaunchDate:       req.LaunchDate,
		EndDate:          req.EndDate,
		PrivacyPolicyURL: req.PrivacyPolicyURL,
		TermsURL:         req.TermsURL,
		MaxSignups:       req.MaxSignups,
		Settings:         convertSettingsRequest(req.EmailSettings, req.BrandingSettings, req.FormSettings, req.ReferralSettings, req.FormFields, req.ShareMessages, req.TrackingIntegrations),
	}

	campaign, err := h.processor.UpdateCampaign(ctx, accountID, campaignID, params)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, campaign)
}

// HandleDeleteCampaign deletes a campaign
func (h *Handler) HandleDeleteCampaign(c *gin.Context) {
	ctx := c.Request.Context()

	accountID, ok := h.getAccountID(c)
	if !ok {
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "account_id", Value: accountID.String()})

	campaignID, ok := h.getCampaignID(c)
	if !ok {
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "campaign_id", Value: campaignID.String()})

	err := h.processor.DeleteCampaign(ctx, accountID, campaignID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// HandleUpdateCampaignStatus updates a campaign's status
func (h *Handler) HandleUpdateCampaignStatus(c *gin.Context) {
	ctx := c.Request.Context()

	accountID, ok := h.getAccountID(c)
	if !ok {
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "account_id", Value: accountID.String()})

	campaignID, ok := h.getCampaignID(c)
	if !ok {
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "campaign_id", Value: campaignID.String()})

	var req UpdateCampaignStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "new_status", Value: req.Status})

	campaign, err := h.processor.UpdateCampaignStatus(ctx, accountID, campaignID, req.Status)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, campaign)
}

func (h *Handler) getAccountID(c *gin.Context) (uuid.UUID, bool) {
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "Account ID not found in context")
		return uuid.UUID{}, false
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		apierrors.BadRequest(c, "INVALID_INPUT", "Invalid account ID format")
		return uuid.UUID{}, false
	}
	return accountID, true
}

func (h *Handler) getCampaignID(c *gin.Context) (uuid.UUID, bool) {
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		apierrors.BadRequest(c, "INVALID_INPUT", "Invalid campaign ID format")
		return uuid.UUID{}, false
	}
	return campaignID, true
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, processor.ErrCampaignNotFound):
		apierrors.NotFound(c, "Campaign not found")
	case errors.Is(err, processor.ErrSlugAlreadyExists):
		apierrors.Conflict(c, "SLUG_EXISTS", "Campaign slug already exists")
	case errors.Is(err, processor.ErrInvalidCampaignStatus):
		apierrors.BadRequest(c, "INVALID_STATUS", "Invalid campaign status")
	case errors.Is(err, processor.ErrInvalidCampaignType):
		apierrors.BadRequest(c, "INVALID_TYPE", "Invalid campaign type")
	case errors.Is(err, processor.ErrUnauthorized):
		apierrors.Forbidden(c, "FORBIDDEN", "You do not have access to this campaign")
	case errors.Is(err, processor.ErrCampaignLimitReached):
		apierrors.Forbidden(c, "CAMPAIGN_LIMIT_REACHED", "You have reached your campaign limit. Please upgrade your plan to create more campaigns.")
	default:
		apierrors.InternalError(c, err)
	}
}

// convertSettingsRequest converts HTTP request settings to processor params
func convertSettingsRequest(
	emailSettings *EmailSettingsRequest,
	brandingSettings *BrandingSettingsRequest,
	formSettings *FormSettingsRequest,
	referralSettings *ReferralSettingsRequest,
	formFields []FormFieldRequest,
	shareMessages []ShareMessageRequest,
	trackingIntegrations []TrackingIntegrationRequest,
) processor.CampaignSettingsParams {
	settings := processor.CampaignSettingsParams{}

	if emailSettings != nil {
		settings.EmailSettings = &processor.EmailSettingsParams{
			FromName:             emailSettings.FromName,
			FromEmail:            emailSettings.FromEmail,
			ReplyTo:              emailSettings.ReplyTo,
			VerificationRequired: emailSettings.VerificationRequired,
			SendWelcomeEmail:     emailSettings.SendWelcomeEmail,
		}
	}

	if brandingSettings != nil {
		settings.BrandingSettings = &processor.BrandingSettingsParams{
			LogoURL:      brandingSettings.LogoURL,
			PrimaryColor: brandingSettings.PrimaryColor,
			FontFamily:   brandingSettings.FontFamily,
			CustomDomain: brandingSettings.CustomDomain,
		}
	}

	if formSettings != nil {
		settings.FormSettings = &processor.FormSettingsParams{
			CaptchaEnabled:  formSettings.CaptchaEnabled,
			CaptchaProvider: formSettings.CaptchaProvider,
			CaptchaSiteKey:  formSettings.CaptchaSiteKey,
			DoubleOptIn:     formSettings.DoubleOptIn,
			Design:          formSettings.Design,
			SuccessTitle:    formSettings.SuccessTitle,
			SuccessMessage:  formSettings.SuccessMessage,
		}
	}

	if referralSettings != nil {
		settings.ReferralSettings = &processor.ReferralSettingsParams{
			Enabled:                 referralSettings.Enabled,
			PointsPerReferral:       referralSettings.PointsPerReferral,
			VerifiedOnly:            referralSettings.VerifiedOnly,
			PositionsToJump:         referralSettings.PositionsToJump,
			ReferrerPositionsToJump: referralSettings.ReferrerPositionsToJump,
			SharingChannels:         referralSettings.SharingChannels,
		}
	}

	for _, f := range formFields {
		settings.FormFields = append(settings.FormFields, processor.FormFieldParams{
			Name:              f.Name,
			FieldType:         f.FieldType,
			Label:             f.Label,
			Placeholder:       f.Placeholder,
			Required:          f.Required,
			ValidationPattern: f.ValidationPattern,
			Options:           f.Options,
			DisplayOrder:      f.DisplayOrder,
		})
	}

	for _, m := range shareMessages {
		settings.ShareMessages = append(settings.ShareMessages, processor.ShareMessageParams{
			Channel: m.Channel,
			Message: m.Message,
		})
	}

	for _, t := range trackingIntegrations {
		settings.TrackingIntegrations = append(settings.TrackingIntegrations, processor.TrackingIntegrationParams{
			IntegrationType: t.IntegrationType,
			Enabled:         t.Enabled,
			TrackingID:      t.TrackingID,
			TrackingLabel:   t.TrackingLabel,
		})
	}

	return settings
}
