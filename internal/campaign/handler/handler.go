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
		apierrors.RespondWithError(c, h.logger, apierrors.Unauthorized("Account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		apierrors.RespondWithError(c, h.logger, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid account ID format"))
		return
	}

	var req CreateCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, h.logger, err)
		return
	}

	// Set default empty JSONB if not provided
	if req.FormConfig == nil {
		emptyJSON := store.JSONB{}
		req.FormConfig = &emptyJSON
	}
	if req.ReferralConfig == nil {
		emptyJSON := store.JSONB{}
		req.ReferralConfig = &emptyJSON
	}
	if req.EmailConfig == nil {
		emptyJSON := store.JSONB{}
		req.EmailConfig = &emptyJSON
	}
	if req.BrandingConfig == nil {
		emptyJSON := store.JSONB{}
		req.BrandingConfig = &emptyJSON
	}

	processorReq := processor.CreateCampaignRequest{
		Name:             req.Name,
		Slug:             req.Slug,
		Description:      req.Description,
		Type:             req.Type,
		FormConfig:       *req.FormConfig,
		ReferralConfig:   *req.ReferralConfig,
		EmailConfig:      *req.EmailConfig,
		BrandingConfig:   *req.BrandingConfig,
		PrivacyPolicyURL: req.PrivacyPolicyURL,
		TermsURL:         req.TermsURL,
		MaxSignups:       req.MaxSignups,
	}

	campaign, err := h.processor.CreateCampaign(ctx, accountID, processorReq)
	if err != nil {
		apierrors.RespondWithError(c, h.logger, err)
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
		apierrors.RespondWithError(c, h.logger, apierrors.Unauthorized("Account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		apierrors.RespondWithError(c, h.logger, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid account ID format"))
		return
	}

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
		apierrors.RespondWithError(c, h.logger, err)
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

// HandleGetCampaign retrieves a campaign by ID
func (h *Handler) HandleGetCampaign(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, h.logger, apierrors.Unauthorized("Account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		apierrors.RespondWithError(c, h.logger, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid account ID format"))
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		apierrors.RespondWithError(c, h.logger, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid campaign ID format"))
		return
	}

	campaign, err := h.processor.GetCampaign(ctx, accountID, campaignID)
	if err != nil {
		apierrors.RespondWithError(c, h.logger, err)
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
		apierrors.RespondWithError(c, h.logger, apierrors.Unauthorized("Account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		apierrors.RespondWithError(c, h.logger, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid account ID format"))
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		apierrors.RespondWithError(c, h.logger, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid campaign ID format"))
		return
	}

	var req UpdateCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, h.logger, err)
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

	if req.FormConfig != nil {
		processorReq.FormConfig = *req.FormConfig
	}
	if req.ReferralConfig != nil {
		processorReq.ReferralConfig = *req.ReferralConfig
	}
	if req.EmailConfig != nil {
		processorReq.EmailConfig = *req.EmailConfig
	}
	if req.BrandingConfig != nil {
		processorReq.BrandingConfig = *req.BrandingConfig
	}

	campaign, err := h.processor.UpdateCampaign(ctx, accountID, campaignID, processorReq)
	if err != nil {
		apierrors.RespondWithError(c, h.logger, err)
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
		apierrors.RespondWithError(c, h.logger, apierrors.Unauthorized("Account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		apierrors.RespondWithError(c, h.logger, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid account ID format"))
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		apierrors.RespondWithError(c, h.logger, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid campaign ID format"))
		return
	}

	err = h.processor.DeleteCampaign(ctx, accountID, campaignID)
	if err != nil {
		apierrors.RespondWithError(c, h.logger, err)
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
		apierrors.RespondWithError(c, h.logger, apierrors.Unauthorized("Account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		apierrors.RespondWithError(c, h.logger, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid account ID format"))
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		apierrors.RespondWithError(c, h.logger, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid campaign ID format"))
		return
	}

	var req UpdateCampaignStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, h.logger, err)
		return
	}

	campaign, err := h.processor.UpdateCampaignStatus(ctx, accountID, campaignID, req.Status)
	if err != nil {
		apierrors.RespondWithError(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, campaign)
}
