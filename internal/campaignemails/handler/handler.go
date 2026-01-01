package handler

import (
	"errors"
	"net/http"

	"base-server/internal/apierrors"
	"base-server/internal/campaignemails/processor"
	"base-server/internal/observability"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	processor processor.CampaignEmailTemplateProcessor
	logger    *observability.Logger
}

func New(processor processor.CampaignEmailTemplateProcessor, logger *observability.Logger) Handler {
	return Handler{
		processor: processor,
		logger:    logger,
	}
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, processor.ErrCampaignEmailTemplateNotFound):
		apierrors.NotFound(c, "Campaign email template not found")
	case errors.Is(err, processor.ErrCampaignNotFound):
		apierrors.NotFound(c, "Campaign not found")
	case errors.Is(err, processor.ErrUnauthorized):
		apierrors.Forbidden(c, "FORBIDDEN", "You do not have access to this template")
	case errors.Is(err, processor.ErrInvalidTemplateType):
		apierrors.BadRequest(c, "INVALID_TYPE", "Invalid template type")
	case errors.Is(err, processor.ErrInvalidTemplateContent):
		apierrors.BadRequest(c, "INVALID_INPUT", "Invalid template content")
	case errors.Is(err, processor.ErrTestEmailFailed):
		apierrors.BadRequest(c, "EMAIL_SEND_FAILED", "Failed to send test email")
	case errors.Is(err, processor.ErrVisualEmailBuilderNotAvailable):
		apierrors.Forbidden(c, "FEATURE_NOT_AVAILABLE", "Visual email builder is not available in your plan. Please upgrade to Pro or Team plan.")
	default:
		apierrors.InternalError(c, err)
	}
}

// CreateCampaignEmailTemplateRequest represents the HTTP request for creating a campaign email template
type CreateCampaignEmailTemplateRequest struct {
	Name              string      `json:"name" binding:"required,max=255"`
	Type              string      `json:"type" binding:"required,oneof=verification welcome position_update reward_earned milestone custom"`
	Subject           string      `json:"subject" binding:"required,max=255"`
	HTMLBody          string      `json:"html_body" binding:"required"`
	BlocksJSON        interface{} `json:"blocks_json"`
	Enabled           *bool       `json:"enabled"`
	SendAutomatically *bool       `json:"send_automatically"`
}

// HandleCreateCampaignEmailTemplate handles POST /api/v1/campaigns/:campaign_id/email-templates
func (h *Handler) HandleCreateCampaignEmailTemplate(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	var req CreateCampaignEmailTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	processorReq := processor.CreateCampaignEmailTemplateRequest{
		Name:              req.Name,
		Type:              req.Type,
		Subject:           req.Subject,
		HTMLBody:          req.HTMLBody,
		BlocksJSON:        req.BlocksJSON,
		Enabled:           req.Enabled,
		SendAutomatically: req.SendAutomatically,
	}

	template, err := h.processor.CreateCampaignEmailTemplate(ctx, accountID, campaignID, processorReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, template)
}

// HandleListCampaignEmailTemplates handles GET /api/v1/campaigns/:campaign_id/email-templates
func (h *Handler) HandleListCampaignEmailTemplates(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get optional type filter
	var templateType *string
	if typeStr := c.Query("type"); typeStr != "" {
		templateType = &typeStr
	}

	templates, err := h.processor.ListCampaignEmailTemplates(ctx, accountID, campaignID, templateType)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, templates)
}

// HandleListAllCampaignEmailTemplates handles GET /api/v1/campaign-email-templates
func (h *Handler) HandleListAllCampaignEmailTemplates(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	templates, err := h.processor.ListAllCampaignEmailTemplates(ctx, accountID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, templates)
}

// HandleGetCampaignEmailTemplate handles GET /api/v1/campaigns/:campaign_id/email-templates/:template_id
func (h *Handler) HandleGetCampaignEmailTemplate(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get template ID from path
	templateIDStr := c.Param("template_id")
	templateID, err := uuid.Parse(templateIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse template ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template id"})
		return
	}

	template, err := h.processor.GetCampaignEmailTemplate(ctx, accountID, campaignID, templateID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, template)
}

// UpdateCampaignEmailTemplateRequest represents the HTTP request for updating a campaign email template
type UpdateCampaignEmailTemplateRequest struct {
	Name              *string     `json:"name,omitempty" binding:"omitempty,max=255"`
	Subject           *string     `json:"subject,omitempty" binding:"omitempty,max=255"`
	HTMLBody          *string     `json:"html_body,omitempty"`
	BlocksJSON        interface{} `json:"blocks_json,omitempty"`
	Enabled           *bool       `json:"enabled,omitempty"`
	SendAutomatically *bool       `json:"send_automatically,omitempty"`
}

// HandleUpdateCampaignEmailTemplate handles PUT /api/v1/campaigns/:campaign_id/email-templates/:template_id
func (h *Handler) HandleUpdateCampaignEmailTemplate(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get template ID from path
	templateIDStr := c.Param("template_id")
	templateID, err := uuid.Parse(templateIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse template ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template id"})
		return
	}

	var req UpdateCampaignEmailTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	processorReq := processor.UpdateCampaignEmailTemplateRequest{
		Name:              req.Name,
		Subject:           req.Subject,
		HTMLBody:          req.HTMLBody,
		BlocksJSON:        req.BlocksJSON,
		Enabled:           req.Enabled,
		SendAutomatically: req.SendAutomatically,
	}

	template, err := h.processor.UpdateCampaignEmailTemplate(ctx, accountID, campaignID, templateID, processorReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, template)
}

// HandleDeleteCampaignEmailTemplate handles DELETE /api/v1/campaigns/:campaign_id/email-templates/:template_id
func (h *Handler) HandleDeleteCampaignEmailTemplate(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get template ID from path
	templateIDStr := c.Param("template_id")
	templateID, err := uuid.Parse(templateIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse template ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template id"})
		return
	}

	err = h.processor.DeleteCampaignEmailTemplate(ctx, accountID, campaignID, templateID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// SendTestEmailRequest represents the HTTP request for sending a test email
type SendTestEmailRequest struct {
	RecipientEmail string                 `json:"recipient_email" binding:"required,email"`
	TestData       map[string]interface{} `json:"test_data,omitempty"`
}

// HandleSendTestEmail handles POST /api/v1/campaigns/:campaign_id/email-templates/:template_id/send-test
func (h *Handler) HandleSendTestEmail(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get template ID from path
	templateIDStr := c.Param("template_id")
	templateID, err := uuid.Parse(templateIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse template ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template id"})
		return
	}

	var req SendTestEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	processorReq := processor.SendTestEmailRequest{
		RecipientEmail: req.RecipientEmail,
		TestData:       req.TestData,
	}

	err = h.processor.SendTestEmail(ctx, accountID, campaignID, templateID, processorReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Test email sent successfully",
		"sent_at": ctx.Value("timestamp"),
	})
}
