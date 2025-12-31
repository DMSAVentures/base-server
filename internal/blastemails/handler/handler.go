package handler

import (
	"errors"
	"net/http"

	"base-server/internal/apierrors"
	"base-server/internal/blastemails/processor"
	"base-server/internal/observability"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	processor processor.BlastEmailTemplateProcessor
	logger    *observability.Logger
}

func New(processor processor.BlastEmailTemplateProcessor, logger *observability.Logger) Handler {
	return Handler{
		processor: processor,
		logger:    logger,
	}
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, processor.ErrBlastEmailTemplateNotFound):
		apierrors.NotFound(c, "Blast email template not found")
	case errors.Is(err, processor.ErrUnauthorized):
		apierrors.Forbidden(c, "FORBIDDEN", "You do not have access to this template")
	case errors.Is(err, processor.ErrInvalidTemplateContent):
		apierrors.BadRequest(c, "INVALID_INPUT", "Invalid template content")
	case errors.Is(err, processor.ErrTestEmailFailed):
		apierrors.BadRequest(c, "EMAIL_SEND_FAILED", "Failed to send test email")
	case errors.Is(err, processor.ErrBlastEmailTemplatesNotAvailable):
		apierrors.Forbidden(c, "FEATURE_NOT_AVAILABLE", "Blast email templates are not available in your plan")
	default:
		apierrors.InternalError(c, err)
	}
}

// CreateBlastEmailTemplateRequest represents the HTTP request for creating a blast email template
type CreateBlastEmailTemplateRequest struct {
	Name       string      `json:"name" binding:"required,max=255"`
	Subject    string      `json:"subject" binding:"required,max=255"`
	HTMLBody   string      `json:"html_body" binding:"required"`
	BlocksJSON interface{} `json:"blocks_json"`
}

// HandleCreateBlastEmailTemplate handles POST /api/v1/blast-email-templates
func (h *Handler) HandleCreateBlastEmailTemplate(c *gin.Context) {
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

	var req CreateBlastEmailTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	processorReq := processor.CreateBlastEmailTemplateRequest{
		Name:       req.Name,
		Subject:    req.Subject,
		HTMLBody:   req.HTMLBody,
		BlocksJSON: req.BlocksJSON,
	}

	template, err := h.processor.CreateBlastEmailTemplate(ctx, accountID, processorReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, template)
}

// HandleListBlastEmailTemplates handles GET /api/v1/blast-email-templates
func (h *Handler) HandleListBlastEmailTemplates(c *gin.Context) {
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

	templates, err := h.processor.ListBlastEmailTemplates(ctx, accountID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, templates)
}

// HandleGetBlastEmailTemplate handles GET /api/v1/blast-email-templates/:id
func (h *Handler) HandleGetBlastEmailTemplate(c *gin.Context) {
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

	// Get template ID from path
	templateIDStr := c.Param("id")
	templateID, err := uuid.Parse(templateIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse template ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template id"})
		return
	}

	template, err := h.processor.GetBlastEmailTemplate(ctx, accountID, templateID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, template)
}

// UpdateBlastEmailTemplateRequest represents the HTTP request for updating a blast email template
type UpdateBlastEmailTemplateRequest struct {
	Name       *string     `json:"name,omitempty" binding:"omitempty,max=255"`
	Subject    *string     `json:"subject,omitempty" binding:"omitempty,max=255"`
	HTMLBody   *string     `json:"html_body,omitempty"`
	BlocksJSON interface{} `json:"blocks_json,omitempty"`
}

// HandleUpdateBlastEmailTemplate handles PUT /api/v1/blast-email-templates/:id
func (h *Handler) HandleUpdateBlastEmailTemplate(c *gin.Context) {
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

	// Get template ID from path
	templateIDStr := c.Param("id")
	templateID, err := uuid.Parse(templateIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse template ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template id"})
		return
	}

	var req UpdateBlastEmailTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	processorReq := processor.UpdateBlastEmailTemplateRequest{
		Name:       req.Name,
		Subject:    req.Subject,
		HTMLBody:   req.HTMLBody,
		BlocksJSON: req.BlocksJSON,
	}

	template, err := h.processor.UpdateBlastEmailTemplate(ctx, accountID, templateID, processorReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, template)
}

// HandleDeleteBlastEmailTemplate handles DELETE /api/v1/blast-email-templates/:id
func (h *Handler) HandleDeleteBlastEmailTemplate(c *gin.Context) {
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

	// Get template ID from path
	templateIDStr := c.Param("id")
	templateID, err := uuid.Parse(templateIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse template ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template id"})
		return
	}

	err = h.processor.DeleteBlastEmailTemplate(ctx, accountID, templateID)
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

// HandleSendTestEmail handles POST /api/v1/blast-email-templates/:id/send-test
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

	// Get template ID from path
	templateIDStr := c.Param("id")
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

	err = h.processor.SendTestEmail(ctx, accountID, templateID, processorReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Test email sent successfully",
		"sent_at": ctx.Value("timestamp"),
	})
}
