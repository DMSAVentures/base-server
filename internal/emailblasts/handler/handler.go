package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"base-server/internal/apierrors"
	"base-server/internal/emailblasts/processor"
	"base-server/internal/observability"

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

// handleError maps processor errors to appropriate API error responses
func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, processor.ErrBlastNotFound):
		apierrors.NotFound(c, "Email blast not found")
	case errors.Is(err, processor.ErrSegmentNotFound):
		apierrors.NotFound(c, "Segment not found")
	case errors.Is(err, processor.ErrTemplateNotFound):
		apierrors.NotFound(c, "Email template not found")
	case errors.Is(err, processor.ErrCampaignNotFound):
		apierrors.NotFound(c, "Campaign not found")
	case errors.Is(err, processor.ErrUnauthorized):
		apierrors.Forbidden(c, "FORBIDDEN", "You do not have access to this email blast")
	case errors.Is(err, processor.ErrBlastCannotModify):
		apierrors.BadRequest(c, "BLAST_CANNOT_MODIFY", "Email blast cannot be modified in its current status")
	case errors.Is(err, processor.ErrBlastCannotDelete):
		apierrors.BadRequest(c, "BLAST_CANNOT_MODIFY", "Email blast cannot be deleted in its current status")
	case errors.Is(err, processor.ErrBlastCannotStart):
		apierrors.BadRequest(c, "BLAST_CANNOT_MODIFY", "Email blast cannot be started in its current status")
	case errors.Is(err, processor.ErrBlastCannotPause):
		apierrors.BadRequest(c, "BLAST_CANNOT_MODIFY", "Email blast cannot be paused in its current status")
	case errors.Is(err, processor.ErrBlastCannotResume):
		apierrors.BadRequest(c, "BLAST_CANNOT_MODIFY", "Email blast cannot be resumed in its current status")
	case errors.Is(err, processor.ErrBlastCannotCancel):
		apierrors.BadRequest(c, "BLAST_CANNOT_MODIFY", "Email blast cannot be cancelled in its current status")
	case errors.Is(err, processor.ErrInvalidScheduleTime):
		apierrors.BadRequest(c, "INVALID_INPUT", "Scheduled time must be in the future")
	case errors.Is(err, processor.ErrNoRecipients):
		apierrors.BadRequest(c, "INVALID_INPUT", "Segment has no matching users to send to")
	case errors.Is(err, processor.ErrEmailBlastsNotAvailable):
		apierrors.Forbidden(c, "FEATURE_NOT_AVAILABLE", "Email blasts are not available in your plan. Please upgrade to Team plan.")
	default:
		apierrors.InternalError(c, err)
	}
}

// CreateEmailBlastRequest represents the HTTP request for creating an email blast
type CreateEmailBlastRequest struct {
	Name                  string     `json:"name" binding:"required,max=255"`
	SegmentIDs            []string   `json:"segment_ids" binding:"required,min=1,dive,uuid"`
	BlastTemplateID       string     `json:"blast_template_id" binding:"required,uuid"`
	Subject               string     `json:"subject" binding:"required,max=255"`
	ScheduledAt           *time.Time `json:"scheduled_at,omitempty"`
	BatchSize             *int       `json:"batch_size,omitempty"`
	SendThrottlePerSecond *int       `json:"send_throttle_per_second,omitempty"`
}

// HandleCreateEmailBlast handles POST /api/v1/blasts
func (h *Handler) HandleCreateEmailBlast(c *gin.Context) {
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

	// Get user ID from context (optional for created_by)
	var userID *uuid.UUID
	if userIDStr, exists := c.Get("User-ID"); exists {
		if id, err := uuid.Parse(userIDStr.(string)); err == nil {
			userID = &id
		}
	}

	var req CreateEmailBlastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	// Parse segment IDs
	segmentIDs := make([]uuid.UUID, len(req.SegmentIDs))
	for i, s := range req.SegmentIDs {
		segmentIDs[i], _ = uuid.Parse(s)
	}

	blastTemplateID, _ := uuid.Parse(req.BlastTemplateID)

	batchSize := 100
	if req.BatchSize != nil {
		batchSize = *req.BatchSize
	}

	processorReq := processor.CreateEmailBlastRequest{
		Name:                  req.Name,
		SegmentIDs:            segmentIDs,
		BlastTemplateID:       blastTemplateID,
		Subject:               req.Subject,
		ScheduledAt:           req.ScheduledAt,
		BatchSize:             batchSize,
		SendThrottlePerSecond: req.SendThrottlePerSecond,
	}

	blast, err := h.processor.CreateEmailBlast(ctx, accountID, userID, processorReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, blast)
}

// HandleListEmailBlasts handles GET /api/v1/blasts
func (h *Handler) HandleListEmailBlasts(c *gin.Context) {
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

	page := parseIntQuery(c, "page", 1)
	limit := parseIntQuery(c, "limit", 25)

	processorReq := processor.ListEmailBlastsRequest{
		Page:  page,
		Limit: limit,
	}

	response, err := h.processor.ListEmailBlasts(ctx, accountID, processorReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleGetEmailBlast handles GET /api/v1/blasts/:blast_id
func (h *Handler) HandleGetEmailBlast(c *gin.Context) {
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	blast, err := h.processor.GetEmailBlast(ctx, accountID, blastID)
	if err != nil {
		h.handleError(c, err)
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

// HandleUpdateEmailBlast handles PUT /api/v1/blasts/:blast_id
func (h *Handler) HandleUpdateEmailBlast(c *gin.Context) {
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
		apierrors.ValidationError(c, err)
		return
	}

	processorReq := processor.UpdateEmailBlastRequest{
		Name:      req.Name,
		Subject:   req.Subject,
		BatchSize: req.BatchSize,
	}

	blast, err := h.processor.UpdateEmailBlast(ctx, accountID, blastID, processorReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, blast)
}

// HandleDeleteEmailBlast handles DELETE /api/v1/blasts/:blast_id
func (h *Handler) HandleDeleteEmailBlast(c *gin.Context) {
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	err = h.processor.DeleteEmailBlast(ctx, accountID, blastID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ScheduleBlastRequest represents the HTTP request for scheduling a blast
type ScheduleBlastRequest struct {
	ScheduledAt time.Time `json:"scheduled_at" binding:"required"`
}

// HandleScheduleBlast handles POST /api/v1/blasts/:blast_id/schedule
func (h *Handler) HandleScheduleBlast(c *gin.Context) {
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
		apierrors.ValidationError(c, err)
		return
	}

	blast, err := h.processor.ScheduleBlast(ctx, accountID, blastID, req.ScheduledAt)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, blast)
}

// HandleSendBlastNow handles POST /api/v1/blasts/:blast_id/send
func (h *Handler) HandleSendBlastNow(c *gin.Context) {
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	blast, err := h.processor.SendBlastNow(ctx, accountID, blastID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, blast)
}

// HandlePauseBlast handles POST /api/v1/blasts/:blast_id/pause
func (h *Handler) HandlePauseBlast(c *gin.Context) {
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	blast, err := h.processor.PauseBlast(ctx, accountID, blastID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, blast)
}

// HandleResumeBlast handles POST /api/v1/blasts/:blast_id/resume
func (h *Handler) HandleResumeBlast(c *gin.Context) {
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	blast, err := h.processor.ResumeBlast(ctx, accountID, blastID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, blast)
}

// HandleCancelBlast handles POST /api/v1/blasts/:blast_id/cancel
func (h *Handler) HandleCancelBlast(c *gin.Context) {
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	blast, err := h.processor.CancelBlast(ctx, accountID, blastID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, blast)
}

// HandleGetBlastAnalytics handles GET /api/v1/blasts/:blast_id/analytics
func (h *Handler) HandleGetBlastAnalytics(c *gin.Context) {
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

	// Get blast ID from path
	blastIDStr := c.Param("blast_id")
	blastID, err := uuid.Parse(blastIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse blast ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blast id"})
		return
	}

	analytics, err := h.processor.GetBlastAnalytics(ctx, accountID, blastID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, analytics)
}

// HandleListBlastRecipients handles GET /api/v1/blasts/:blast_id/recipients
func (h *Handler) HandleListBlastRecipients(c *gin.Context) {
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

	response, err := h.processor.ListBlastRecipients(ctx, accountID, blastID, processorReq)
	if err != nil {
		h.handleError(c, err)
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
