package handler

import (
	"base-server/internal/emailtemplates/processor"
	"base-server/internal/observability"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	processor processor.EmailTemplateProcessor
	logger    *observability.Logger
}

func New(processor processor.EmailTemplateProcessor, logger *observability.Logger) Handler {
	return Handler{
		processor: processor,
		logger:    logger,
	}
}

// CreateEmailTemplateRequest represents the HTTP request for creating an email template
type CreateEmailTemplateRequest struct {
	Name              string `json:"name" binding:"required,max=255"`
	Type              string `json:"type" binding:"required,oneof=verification welcome position_update reward_earned milestone custom"`
	Subject           string `json:"subject" binding:"required,max=255"`
	HTMLBody          string `json:"html_body" binding:"required"`
	TextBody          string `json:"text_body" binding:"required"`
	Enabled           *bool  `json:"enabled"`
	SendAutomatically *bool  `json:"send_automatically"`
}

// HandleCreateEmailTemplate handles POST /api/v1/campaigns/:campaign_id/email-templates
func (h *Handler) HandleCreateEmailTemplate(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		h.logger.Error(ctx, "account ID not found in context", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
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

	var req CreateEmailTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	processorReq := processor.CreateEmailTemplateRequest{
		Name:              req.Name,
		Type:              req.Type,
		Subject:           req.Subject,
		HTMLBody:          req.HTMLBody,
		TextBody:          req.TextBody,
		Enabled:           req.Enabled,
		SendAutomatically: req.SendAutomatically,
	}

	template, err := h.processor.CreateEmailTemplate(ctx, accountID, campaignID, processorReq)
	if err != nil {
		h.logger.Error(ctx, "failed to create email template", err)
		if errors.Is(err, processor.ErrCampaignNotFound) || errors.Is(err, processor.ErrUnauthorized) {
			c.JSON(http.StatusNotFound, gin.H{"error": "campaign not found"})
			return
		}
		if errors.Is(err, processor.ErrInvalidTemplateType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template type"})
			return
		}
		if errors.Is(err, processor.ErrInvalidTemplateContent) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template content"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, template)
}

// HandleListEmailTemplates handles GET /api/v1/campaigns/:campaign_id/email-templates
func (h *Handler) HandleListEmailTemplates(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		h.logger.Error(ctx, "account ID not found in context", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
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

	templates, err := h.processor.ListEmailTemplates(ctx, accountID, campaignID, templateType)
	if err != nil {
		h.logger.Error(ctx, "failed to list email templates", err)
		if errors.Is(err, processor.ErrCampaignNotFound) || errors.Is(err, processor.ErrUnauthorized) {
			c.JSON(http.StatusNotFound, gin.H{"error": "campaign not found"})
			return
		}
		if errors.Is(err, processor.ErrInvalidTemplateType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template type"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, templates)
}

// HandleGetEmailTemplate handles GET /api/v1/campaigns/:campaign_id/email-templates/:template_id
func (h *Handler) HandleGetEmailTemplate(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		h.logger.Error(ctx, "account ID not found in context", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
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

	template, err := h.processor.GetEmailTemplate(ctx, accountID, campaignID, templateID)
	if err != nil {
		h.logger.Error(ctx, "failed to get email template", err)
		if errors.Is(err, processor.ErrTemplateNotFound) || errors.Is(err, processor.ErrCampaignNotFound) || errors.Is(err, processor.ErrUnauthorized) {
			c.JSON(http.StatusNotFound, gin.H{"error": "email template not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, template)
}

// UpdateEmailTemplateRequest represents the HTTP request for updating an email template
type UpdateEmailTemplateRequest struct {
	Name              *string `json:"name,omitempty" binding:"omitempty,max=255"`
	Subject           *string `json:"subject,omitempty" binding:"omitempty,max=255"`
	HTMLBody          *string `json:"html_body,omitempty"`
	TextBody          *string `json:"text_body,omitempty"`
	Enabled           *bool   `json:"enabled,omitempty"`
	SendAutomatically *bool   `json:"send_automatically,omitempty"`
}

// HandleUpdateEmailTemplate handles PUT /api/v1/campaigns/:campaign_id/email-templates/:template_id
func (h *Handler) HandleUpdateEmailTemplate(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		h.logger.Error(ctx, "account ID not found in context", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
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

	var req UpdateEmailTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	processorReq := processor.UpdateEmailTemplateRequest{
		Name:              req.Name,
		Subject:           req.Subject,
		HTMLBody:          req.HTMLBody,
		TextBody:          req.TextBody,
		Enabled:           req.Enabled,
		SendAutomatically: req.SendAutomatically,
	}

	template, err := h.processor.UpdateEmailTemplate(ctx, accountID, campaignID, templateID, processorReq)
	if err != nil {
		h.logger.Error(ctx, "failed to update email template", err)
		if errors.Is(err, processor.ErrTemplateNotFound) || errors.Is(err, processor.ErrCampaignNotFound) || errors.Is(err, processor.ErrUnauthorized) {
			c.JSON(http.StatusNotFound, gin.H{"error": "email template not found"})
			return
		}
		if errors.Is(err, processor.ErrInvalidTemplateContent) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template content"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, template)
}

// HandleDeleteEmailTemplate handles DELETE /api/v1/campaigns/:campaign_id/email-templates/:template_id
func (h *Handler) HandleDeleteEmailTemplate(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		h.logger.Error(ctx, "account ID not found in context", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
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

	err = h.processor.DeleteEmailTemplate(ctx, accountID, campaignID, templateID)
	if err != nil {
		h.logger.Error(ctx, "failed to delete email template", err)
		if errors.Is(err, processor.ErrTemplateNotFound) || errors.Is(err, processor.ErrCampaignNotFound) || errors.Is(err, processor.ErrUnauthorized) {
			c.JSON(http.StatusNotFound, gin.H{"error": "email template not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		h.logger.Error(ctx, "account ID not found in context", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
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
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	processorReq := processor.SendTestEmailRequest{
		RecipientEmail: req.RecipientEmail,
		TestData:       req.TestData,
	}

	err = h.processor.SendTestEmail(ctx, accountID, campaignID, templateID, processorReq)
	if err != nil {
		h.logger.Error(ctx, "failed to send test email", err)
		if errors.Is(err, processor.ErrTemplateNotFound) || errors.Is(err, processor.ErrCampaignNotFound) || errors.Is(err, processor.ErrUnauthorized) {
			c.JSON(http.StatusNotFound, gin.H{"error": "email template not found"})
			return
		}
		if errors.Is(err, processor.ErrTestEmailFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send test email"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Test email sent successfully",
		"sent_at": ctx.Value("timestamp"),
	})
}
