package handler

import (
	"base-server/internal/apierrors"
	"base-server/internal/emailblasts/processor"
	"base-server/internal/observability"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	processor processor.EmailBlastProcessor
	logger    *observability.Logger
}

func New(processor processor.EmailBlastProcessor, logger *observability.Logger) Handler {
	return Handler{
		processor: processor,
		logger:    logger,
	}
}

// CreateEmailBlastRequest represents the HTTP request for creating an email blast
type CreateEmailBlastRequest struct {
	Name                  string     `json:"name" binding:"required,max=255"`
	SegmentID             string     `json:"segment_id" binding:"required,uuid"`
	TemplateID            string     `json:"template_id" binding:"required,uuid"`
	Subject               string     `json:"subject" binding:"required,max=255"`
	ScheduledAt           *time.Time `json:"scheduled_at,omitempty"`
	BatchSize             *int       `json:"batch_size,omitempty"`
	SendThrottlePerSecond *int       `json:"send_throttle_per_second,omitempty"`
}

// HandleCreateEmailBlast handles POST /api/v1/campaigns/:campaign_id/blasts
func (h *Handler) HandleCreateEmailBlast(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get user ID from context (optional for created_by)
	var userID *uuid.UUID
	if userIDStr, exists := c.Get("User-ID"); exists {
		if id, err := uuid.Parse(userIDStr.(string)); err == nil {
			userID = &id
		}
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	var req CreateEmailBlastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	segmentID, _ := uuid.Parse(req.SegmentID)
	templateID, _ := uuid.Parse(req.TemplateID)

	batchSize := 100
	if req.BatchSize != nil {
		batchSize = *req.BatchSize
	}

	processorReq := processor.CreateEmailBlastRequest{
		Name:                  req.Name,
		SegmentID:             segmentID,
		TemplateID:            templateID,
		Subject:               req.Subject,
		ScheduledAt:           req.ScheduledAt,
		BatchSize:             batchSize,
		SendThrottlePerSecond: req.SendThrottlePerSecond,
	}

	blast, err := h.processor.CreateEmailBlast(ctx, accountID, campaignID, userID, processorReq)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusCreated, blast)
}

// HandleListEmailBlasts handles GET /api/v1/campaigns/:campaign_id/blasts
func (h *Handler) HandleListEmailBlasts(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
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

	page := parseIntQuery(c, "page", 1)
	limit := parseIntQuery(c, "limit", 25)

	processorReq := processor.ListEmailBlastsRequest{
		Page:  page,
		Limit: limit,
	}

	response, err := h.processor.ListEmailBlasts(ctx, accountID, campaignID, processorReq)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleGetEmailBlast handles GET /api/v1/campaigns/:campaign_id/blasts/:blast_id
func (h *Handler) HandleGetEmailBlast(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	blast, err := h.processor.GetEmailBlast(ctx, accountID, campaignID, blastID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, blast)
}

// UpdateEmailBlastRequest represents the HTTP request for updating an email blast
type UpdateEmailBlastRequest struct {
	Name      *string `json:"name,omitempty" binding:"omitempty,max=255"`
	Subject   *string `json:"subject,omitempty" binding:"omitempty,max=255"`
	BatchSize *int    `json:"batch_size,omitempty"`
}

// HandleUpdateEmailBlast handles PUT /api/v1/campaigns/:campaign_id/blasts/:blast_id
func (h *Handler) HandleUpdateEmailBlast(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	var req UpdateEmailBlastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	processorReq := processor.UpdateEmailBlastRequest{
		Name:      req.Name,
		Subject:   req.Subject,
		BatchSize: req.BatchSize,
	}

	blast, err := h.processor.UpdateEmailBlast(ctx, accountID, campaignID, blastID, processorReq)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, blast)
}

// HandleDeleteEmailBlast handles DELETE /api/v1/campaigns/:campaign_id/blasts/:blast_id
func (h *Handler) HandleDeleteEmailBlast(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	err = h.processor.DeleteEmailBlast(ctx, accountID, campaignID, blastID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ScheduleBlastRequest represents the HTTP request for scheduling a blast
type ScheduleBlastRequest struct {
	ScheduledAt time.Time `json:"scheduled_at" binding:"required"`
}

// HandleScheduleBlast handles POST /api/v1/campaigns/:campaign_id/blasts/:blast_id/schedule
func (h *Handler) HandleScheduleBlast(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	var req ScheduleBlastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	blast, err := h.processor.ScheduleBlast(ctx, accountID, campaignID, blastID, req.ScheduledAt)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, blast)
}

// HandleSendBlastNow handles POST /api/v1/campaigns/:campaign_id/blasts/:blast_id/send
func (h *Handler) HandleSendBlastNow(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	blast, err := h.processor.SendBlastNow(ctx, accountID, campaignID, blastID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, blast)
}

// HandlePauseBlast handles POST /api/v1/campaigns/:campaign_id/blasts/:blast_id/pause
func (h *Handler) HandlePauseBlast(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	blast, err := h.processor.PauseBlast(ctx, accountID, campaignID, blastID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, blast)
}

// HandleResumeBlast handles POST /api/v1/campaigns/:campaign_id/blasts/:blast_id/resume
func (h *Handler) HandleResumeBlast(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	blast, err := h.processor.ResumeBlast(ctx, accountID, campaignID, blastID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, blast)
}

// HandleCancelBlast handles POST /api/v1/campaigns/:campaign_id/blasts/:blast_id/cancel
func (h *Handler) HandleCancelBlast(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	blast, err := h.processor.CancelBlast(ctx, accountID, campaignID, blastID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, blast)
}

// HandleGetBlastAnalytics handles GET /api/v1/campaigns/:campaign_id/blasts/:blast_id/analytics
func (h *Handler) HandleGetBlastAnalytics(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	analytics, err := h.processor.GetBlastAnalytics(ctx, accountID, campaignID, blastID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, analytics)
}

// HandleListBlastRecipients handles GET /api/v1/campaigns/:campaign_id/blasts/:blast_id/recipients
func (h *Handler) HandleListBlastRecipients(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	page := parseIntQuery(c, "page", 1)
	limit := parseIntQuery(c, "limit", 25)

	processorReq := processor.ListBlastRecipientsRequest{
		Page:  page,
		Limit: limit,
	}

	response, err := h.processor.ListBlastRecipients(ctx, accountID, campaignID, blastID, processorReq)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// Helper function to parse integer query parameters
func parseIntQuery(c *gin.Context, key string, defaultValue int) int {
	valStr := c.Query(key)
	if valStr == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultValue
	}
	return val
}
