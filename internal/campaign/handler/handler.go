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

// CreateCampaignRequest represents the HTTP request for creating a campaign
type CreateCampaignRequest struct {
	Name             string       `json:"name" binding:"required,min=1,max=255"`
	Slug             string       `json:"slug" binding:"required,min=1,max=255"`
	Description      *string      `json:"description,omitempty"`
	Type             string       `json:"type" binding:"required,oneof=waitlist referral contest"`
	FormConfig       *store.JSONB `json:"form_config,omitempty"`
	ReferralConfig   *store.JSONB `json:"referral_config,omitempty"`
	EmailConfig      *store.JSONB `json:"email_config,omitempty"`
	BrandingConfig   *store.JSONB `json:"branding_config,omitempty"`
	PrivacyPolicyURL *string      `json:"privacy_policy_url,omitempty"`
	TermsURL         *string      `json:"terms_url,omitempty"`
	MaxSignups       *int         `json:"max_signups,omitempty"`
}

// UpdateCampaignRequest represents the HTTP request for updating a campaign
type UpdateCampaignRequest struct {
	Name             *string      `json:"name,omitempty"`
	Description      *string      `json:"description,omitempty"`
	LaunchDate       *string      `json:"launch_date,omitempty"`
	EndDate          *string      `json:"end_date,omitempty"`
	FormConfig       *store.JSONB `json:"form_config,omitempty"`
	ReferralConfig   *store.JSONB `json:"referral_config,omitempty"`
	EmailConfig      *store.JSONB `json:"email_config,omitempty"`
	BrandingConfig   *store.JSONB `json:"branding_config,omitempty"`
	PrivacyPolicyURL *string      `json:"privacy_policy_url,omitempty"`
	TermsURL         *string      `json:"terms_url,omitempty"`
	MaxSignups       *int         `json:"max_signups,omitempty"`
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

	// Convert JSONB configs to typed configs for v2 processor
	formConfig := convertJSONBToFormConfig(req.FormConfig)
	referralConfig := convertJSONBToReferralConfig(req.ReferralConfig)
	emailConfig := convertJSONBToEmailConfig(req.EmailConfig)
	brandingConfig := convertJSONBToBrandingConfig(req.BrandingConfig)

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

	processorReq := processor.UpdateCampaignRequestV2{
		Name:             req.Name,
		Description:      req.Description,
		LaunchDate:       req.LaunchDate,
		EndDate:          req.EndDate,
		PrivacyPolicyURL: req.PrivacyPolicyURL,
		TermsURL:         req.TermsURL,
		MaxSignups:       req.MaxSignups,
	}

	// Convert JSONB configs to typed configs if provided
	if req.FormConfig != nil {
		converted := convertJSONBToFormConfig(req.FormConfig)
		processorReq.FormConfig = &converted
	}
	if req.ReferralConfig != nil {
		converted := convertJSONBToReferralConfig(req.ReferralConfig)
		processorReq.ReferralConfig = &converted
	}
	if req.EmailConfig != nil {
		converted := convertJSONBToEmailConfig(req.EmailConfig)
		processorReq.EmailConfig = &converted
	}
	if req.BrandingConfig != nil {
		converted := convertJSONBToBrandingConfig(req.BrandingConfig)
		processorReq.BrandingConfig = &converted
	}

	campaign, err := h.processor.UpdateCampaignV2(ctx, accountID, campaignID, processorReq)
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

// Conversion helpers from JSONB to typed configs

func convertJSONBToFormConfig(jsonb *store.JSONB) processor.FormConfigRequest {
	if jsonb == nil {
		return processor.FormConfigRequest{}
	}

	config := processor.FormConfigRequest{}
	j := *jsonb

	// Convert captcha_enabled
	if val, ok := j["captcha_enabled"].(bool); ok {
		config.CaptchaEnabled = &val
	}

	// Convert double_opt_in
	if val, ok := j["double_opt_in"].(bool); ok {
		config.DoubleOptIn = &val
	}

	// Convert custom_css
	if val, ok := j["custom_css"].(string); ok {
		config.CustomCSS = &val
	}

	// Convert fields array
	if fieldsInterface, ok := j["fields"].([]interface{}); ok {
		fields := make([]processor.FormFieldRequest, 0, len(fieldsInterface))
		for i, fieldInterface := range fieldsInterface {
			if fieldMap, ok := fieldInterface.(map[string]interface{}); ok {
				field := processor.FormFieldRequest{
					DisplayOrder: i,
				}
				if name, ok := fieldMap["name"].(string); ok {
					field.Name = name
				}
				if fieldType, ok := fieldMap["type"].(string); ok {
					field.Type = fieldType
				}
				if label, ok := fieldMap["label"].(string); ok {
					field.Label = label
				}
				if placeholder, ok := fieldMap["placeholder"].(string); ok {
					field.Placeholder = &placeholder
				}
				if required, ok := fieldMap["required"].(bool); ok {
					field.Required = required
				}
				if options, ok := fieldMap["options"].([]interface{}); ok {
					strOptions := make([]string, 0, len(options))
					for _, opt := range options {
						if strOpt, ok := opt.(string); ok {
							strOptions = append(strOptions, strOpt)
						}
					}
					field.Options = strOptions
				}
				if validation, ok := fieldMap["validation"].(map[string]interface{}); ok {
					field.Validation = validation
				}
				fields = append(fields, field)
			}
		}
		config.Fields = fields
	}

	return config
}

func convertJSONBToReferralConfig(jsonb *store.JSONB) processor.ReferralConfigRequest {
	if jsonb == nil {
		return processor.ReferralConfigRequest{}
	}

	config := processor.ReferralConfigRequest{}
	j := *jsonb

	if val, ok := j["enabled"].(bool); ok {
		config.Enabled = &val
	}

	if val, ok := j["points_per_referral"].(float64); ok {
		intVal := int(val)
		config.PointsPerReferral = &intVal
	} else if val, ok := j["points_per_referral"].(int); ok {
		config.PointsPerReferral = &val
	}

	if val, ok := j["verified_only"].(bool); ok {
		config.VerifiedOnly = &val
	}

	if channels, ok := j["sharing_channels"].([]interface{}); ok {
		strChannels := make([]string, 0, len(channels))
		for _, ch := range channels {
			if strCh, ok := ch.(string); ok {
				strChannels = append(strChannels, strCh)
			}
		}
		config.SharingChannels = strChannels
	}

	if messages, ok := j["custom_share_messages"].(map[string]interface{}); ok {
		config.CustomShareMessages = messages
	}

	return config
}

func convertJSONBToEmailConfig(jsonb *store.JSONB) processor.EmailConfigRequest {
	if jsonb == nil {
		return processor.EmailConfigRequest{}
	}

	config := processor.EmailConfigRequest{}
	j := *jsonb

	if val, ok := j["from_name"].(string); ok {
		config.FromName = &val
	}

	if val, ok := j["from_email"].(string); ok {
		config.FromEmail = &val
	}

	if val, ok := j["reply_to"].(string); ok {
		config.ReplyTo = &val
	}

	if val, ok := j["verification_required"].(bool); ok {
		config.VerificationRequired = &val
	}

	return config
}

func convertJSONBToBrandingConfig(jsonb *store.JSONB) processor.BrandingConfigRequest {
	if jsonb == nil {
		return processor.BrandingConfigRequest{}
	}

	config := processor.BrandingConfigRequest{}
	j := *jsonb

	if val, ok := j["logo_url"].(string); ok {
		config.LogoURL = &val
	}

	if val, ok := j["primary_color"].(string); ok {
		config.PrimaryColor = &val
	}

	if val, ok := j["font_family"].(string); ok {
		config.FontFamily = &val
	}

	if val, ok := j["custom_domain"].(string); ok {
		config.CustomDomain = &val
	}

	return config
}
