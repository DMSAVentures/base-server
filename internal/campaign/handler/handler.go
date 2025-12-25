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
	CaptchaEnabled  bool        `json:"captcha_enabled"`
	CaptchaProvider *string     `json:"captcha_provider,omitempty"`
	CaptchaSiteKey  *string     `json:"captcha_site_key,omitempty"`
	DoubleOptIn     bool        `json:"double_opt_in"`
	Design          store.JSONB `json:"design"`
	SuccessTitle    *string     `json:"success_title,omitempty"`
	SuccessMessage  *string     `json:"success_message,omitempty"`
}

// ReferralSettingsRequest represents referral settings in HTTP request
type ReferralSettingsRequest struct {
	Enabled                 bool     `json:"enabled"`
	PointsPerReferral       int      `json:"points_per_referral"`
	VerifiedOnly            bool     `json:"verified_only"`
	PositionsToJump         int      `json:"positions_to_jump"`
	ReferrerPositionsToJump int      `json:"referrer_positions_to_jump"`
	SharingChannels         []string `json:"sharing_channels"`
}

// FormFieldRequest represents a form field in HTTP request
type FormFieldRequest struct {
	Name              string   `json:"name" binding:"required"`
	FieldType         string   `json:"field_type" binding:"required"`
	Label             string   `json:"label" binding:"required"`
	Placeholder       *string  `json:"placeholder,omitempty"`
	Required          bool     `json:"required"`
	ValidationPattern *string  `json:"validation_pattern,omitempty"`
	Options           []string `json:"options,omitempty"`
	DisplayOrder      int      `json:"display_order"`
}

// ShareMessageRequest represents a share message in HTTP request
type ShareMessageRequest struct {
	Channel string `json:"channel" binding:"required"`
	Message string `json:"message" binding:"required"`
}

// TrackingIntegrationRequest represents a tracking integration in HTTP request
type TrackingIntegrationRequest struct {
	IntegrationType string  `json:"integration_type" binding:"required"`
	Enabled         bool    `json:"enabled"`
	TrackingID      string  `json:"tracking_id" binding:"required"`
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
	FormFields           []FormFieldRequest           `json:"form_fields,omitempty"`
	ShareMessages        []ShareMessageRequest        `json:"share_messages,omitempty"`
	TrackingIntegrations []TrackingIntegrationRequest `json:"tracking_integrations,omitempty"`
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
	FormFields           []FormFieldRequest           `json:"form_fields,omitempty"`
	ShareMessages        []ShareMessageRequest        `json:"share_messages,omitempty"`
	TrackingIntegrations []TrackingIntegrationRequest `json:"tracking_integrations,omitempty"`
}

// UpdateCampaignStatusRequest represents the HTTP request for updating campaign status
type UpdateCampaignStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=draft active paused completed"`
}

// HandleCreateCampaign creates a new campaign
func (h *Handler) HandleCreateCampaign(c *gin.Context) {
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

	var req CreateCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	// Add campaign identifiers to observability context for comprehensive logging
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_slug", Value: req.Slug},
		observability.Field{Key: "campaign_type", Value: req.Type},
	)

	processorReq := processor.CreateCampaignRequest{
		Name:             req.Name,
		Slug:             req.Slug,
		Description:      req.Description,
		Type:             req.Type,
		PrivacyPolicyURL: req.PrivacyPolicyURL,
		TermsURL:         req.TermsURL,
		MaxSignups:       req.MaxSignups,
	}

	// Convert settings
	if req.EmailSettings != nil {
		processorReq.EmailSettings = &processor.EmailSettingsInput{
			FromName:             req.EmailSettings.FromName,
			FromEmail:            req.EmailSettings.FromEmail,
			ReplyTo:              req.EmailSettings.ReplyTo,
			VerificationRequired: req.EmailSettings.VerificationRequired,
			SendWelcomeEmail:     req.EmailSettings.SendWelcomeEmail,
		}
	}

	if req.BrandingSettings != nil {
		processorReq.BrandingSettings = &processor.BrandingSettingsInput{
			LogoURL:      req.BrandingSettings.LogoURL,
			PrimaryColor: req.BrandingSettings.PrimaryColor,
			FontFamily:   req.BrandingSettings.FontFamily,
			CustomDomain: req.BrandingSettings.CustomDomain,
		}
	}

	if req.FormSettings != nil {
		var captchaProvider *store.CaptchaProvider
		if req.FormSettings.CaptchaProvider != nil {
			cp := store.CaptchaProvider(*req.FormSettings.CaptchaProvider)
			captchaProvider = &cp
		}
		processorReq.FormSettings = &processor.FormSettingsInput{
			CaptchaEnabled:  req.FormSettings.CaptchaEnabled,
			CaptchaProvider: captchaProvider,
			CaptchaSiteKey:  req.FormSettings.CaptchaSiteKey,
			DoubleOptIn:     req.FormSettings.DoubleOptIn,
			Design:          req.FormSettings.Design,
			SuccessTitle:    req.FormSettings.SuccessTitle,
			SuccessMessage:  req.FormSettings.SuccessMessage,
		}
	}

	if req.ReferralSettings != nil {
		sharingChannels := make([]store.SharingChannel, len(req.ReferralSettings.SharingChannels))
		for i, ch := range req.ReferralSettings.SharingChannels {
			sharingChannels[i] = store.SharingChannel(ch)
		}
		processorReq.ReferralSettings = &processor.ReferralSettingsInput{
			Enabled:                 req.ReferralSettings.Enabled,
			PointsPerReferral:       req.ReferralSettings.PointsPerReferral,
			VerifiedOnly:            req.ReferralSettings.VerifiedOnly,
			PositionsToJump:         req.ReferralSettings.PositionsToJump,
			ReferrerPositionsToJump: req.ReferralSettings.ReferrerPositionsToJump,
			SharingChannels:         sharingChannels,
		}
	}

	// Convert form fields
	for _, f := range req.FormFields {
		processorReq.FormFields = append(processorReq.FormFields, processor.FormFieldInput{
			Name:              f.Name,
			FieldType:         store.FormFieldType(f.FieldType),
			Label:             f.Label,
			Placeholder:       f.Placeholder,
			Required:          f.Required,
			ValidationPattern: f.ValidationPattern,
			Options:           f.Options,
			DisplayOrder:      f.DisplayOrder,
		})
	}

	// Convert share messages
	for _, m := range req.ShareMessages {
		processorReq.ShareMessages = append(processorReq.ShareMessages, processor.ShareMessageInput{
			Channel: store.SharingChannel(m.Channel),
			Message: m.Message,
		})
	}

	// Convert tracking integrations
	for _, t := range req.TrackingIntegrations {
		processorReq.TrackingIntegrations = append(processorReq.TrackingIntegrations, processor.TrackingIntegrationInput{
			IntegrationType: store.TrackingIntegrationType(t.IntegrationType),
			Enabled:         t.Enabled,
			TrackingID:      t.TrackingID,
			TrackingLabel:   t.TrackingLabel,
		})
	}

	campaign, err := h.processor.CreateCampaign(ctx, accountID, processorReq)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusCreated, campaign)
}

// HandleListCampaigns lists all campaigns for the account
func (h *Handler) HandleListCampaigns(c *gin.Context) {
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

	// Parse query parameters
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
		apierrors.RespondWithError(c, err)
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

	campaign, err := h.processor.GetCampaign(ctx, accountID, campaignID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, campaign)
}

// HandleGetPublicCampaign retrieves a campaign by ID without authentication (for public form rendering)
func (h *Handler) HandleGetPublicCampaign(c *gin.Context) {
	ctx := c.Request.Context()

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		apierrors.RespondWithError(c, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid campaign ID format"))
		return
	}

	// Add campaign_id to observability context for comprehensive logging
	ctx = observability.WithFields(ctx, observability.Field{Key: "campaign_id", Value: campaignID.String()})

	campaign, err := h.processor.GetPublicCampaign(ctx, campaignID)
	if err != nil {
		// Processor already logged detailed error with full context
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, campaign)
}

// HandleUpdateCampaign updates a campaign
func (h *Handler) HandleUpdateCampaign(c *gin.Context) {
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

	var req UpdateCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	processorReq := processor.UpdateCampaignRequest{
		Name:             req.Name,
		Description:      req.Description,
		LaunchDate:       req.LaunchDate,
		EndDate:          req.EndDate,
		PrivacyPolicyURL: req.PrivacyPolicyURL,
		TermsURL:         req.TermsURL,
		MaxSignups:       req.MaxSignups,
	}

	// Convert settings
	if req.EmailSettings != nil {
		processorReq.EmailSettings = &processor.EmailSettingsInput{
			FromName:             req.EmailSettings.FromName,
			FromEmail:            req.EmailSettings.FromEmail,
			ReplyTo:              req.EmailSettings.ReplyTo,
			VerificationRequired: req.EmailSettings.VerificationRequired,
			SendWelcomeEmail:     req.EmailSettings.SendWelcomeEmail,
		}
	}

	if req.BrandingSettings != nil {
		processorReq.BrandingSettings = &processor.BrandingSettingsInput{
			LogoURL:      req.BrandingSettings.LogoURL,
			PrimaryColor: req.BrandingSettings.PrimaryColor,
			FontFamily:   req.BrandingSettings.FontFamily,
			CustomDomain: req.BrandingSettings.CustomDomain,
		}
	}

	if req.FormSettings != nil {
		var captchaProvider *store.CaptchaProvider
		if req.FormSettings.CaptchaProvider != nil {
			cp := store.CaptchaProvider(*req.FormSettings.CaptchaProvider)
			captchaProvider = &cp
		}
		processorReq.FormSettings = &processor.FormSettingsInput{
			CaptchaEnabled:  req.FormSettings.CaptchaEnabled,
			CaptchaProvider: captchaProvider,
			CaptchaSiteKey:  req.FormSettings.CaptchaSiteKey,
			DoubleOptIn:     req.FormSettings.DoubleOptIn,
			Design:          req.FormSettings.Design,
			SuccessTitle:    req.FormSettings.SuccessTitle,
			SuccessMessage:  req.FormSettings.SuccessMessage,
		}
	}

	if req.ReferralSettings != nil {
		sharingChannels := make([]store.SharingChannel, len(req.ReferralSettings.SharingChannels))
		for i, ch := range req.ReferralSettings.SharingChannels {
			sharingChannels[i] = store.SharingChannel(ch)
		}
		processorReq.ReferralSettings = &processor.ReferralSettingsInput{
			Enabled:                 req.ReferralSettings.Enabled,
			PointsPerReferral:       req.ReferralSettings.PointsPerReferral,
			VerifiedOnly:            req.ReferralSettings.VerifiedOnly,
			PositionsToJump:         req.ReferralSettings.PositionsToJump,
			ReferrerPositionsToJump: req.ReferralSettings.ReferrerPositionsToJump,
			SharingChannels:         sharingChannels,
		}
	}

	// Convert form fields
	for _, f := range req.FormFields {
		processorReq.FormFields = append(processorReq.FormFields, processor.FormFieldInput{
			Name:              f.Name,
			FieldType:         store.FormFieldType(f.FieldType),
			Label:             f.Label,
			Placeholder:       f.Placeholder,
			Required:          f.Required,
			ValidationPattern: f.ValidationPattern,
			Options:           f.Options,
			DisplayOrder:      f.DisplayOrder,
		})
	}

	// Convert share messages
	for _, m := range req.ShareMessages {
		processorReq.ShareMessages = append(processorReq.ShareMessages, processor.ShareMessageInput{
			Channel: store.SharingChannel(m.Channel),
			Message: m.Message,
		})
	}

	// Convert tracking integrations
	for _, t := range req.TrackingIntegrations {
		processorReq.TrackingIntegrations = append(processorReq.TrackingIntegrations, processor.TrackingIntegrationInput{
			IntegrationType: store.TrackingIntegrationType(t.IntegrationType),
			Enabled:         t.Enabled,
			TrackingID:      t.TrackingID,
			TrackingLabel:   t.TrackingLabel,
		})
	}

	campaign, err := h.processor.UpdateCampaign(ctx, accountID, campaignID, processorReq)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, campaign)
}

// HandleDeleteCampaign deletes a campaign
func (h *Handler) HandleDeleteCampaign(c *gin.Context) {
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

	err = h.processor.DeleteCampaign(ctx, accountID, campaignID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// HandleUpdateCampaignStatus updates a campaign's status
func (h *Handler) HandleUpdateCampaignStatus(c *gin.Context) {
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

	var req UpdateCampaignStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	// Add new status to observability context for comprehensive logging
	ctx = observability.WithFields(ctx, observability.Field{Key: "new_status", Value: req.Status})

	campaign, err := h.processor.UpdateCampaignStatus(ctx, accountID, campaignID, req.Status)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, campaign)
}
