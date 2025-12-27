package handler

import (
	"base-server/internal/apierrors"
	"base-server/internal/observability"
	"base-server/internal/segments/processor"
	"base-server/internal/store"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	processor processor.SegmentProcessor
	logger    *observability.Logger
}

func New(processor processor.SegmentProcessor, logger *observability.Logger) Handler {
	return Handler{
		processor: processor,
		logger:    logger,
	}
}

// FilterCriteriaRequest represents the filter criteria in HTTP requests
type FilterCriteriaRequest struct {
	Statuses      []string          `json:"statuses,omitempty"`
	Sources       []string          `json:"sources,omitempty"`
	EmailVerified *bool             `json:"email_verified,omitempty"`
	HasReferrals  *bool             `json:"has_referrals,omitempty"`
	MinReferrals  *int              `json:"min_referrals,omitempty"`
	MinPosition   *int              `json:"min_position,omitempty"`
	MaxPosition   *int              `json:"max_position,omitempty"`
	DateFrom      *string           `json:"date_from,omitempty"`
	DateTo        *string           `json:"date_to,omitempty"`
	CustomFields  map[string]string `json:"custom_fields,omitempty"`
}

// CreateSegmentRequest represents the HTTP request for creating a segment
type CreateSegmentRequest struct {
	Name           string                `json:"name" binding:"required,max=255"`
	Description    *string               `json:"description,omitempty"`
	FilterCriteria FilterCriteriaRequest `json:"filter_criteria" binding:"required"`
}

// HandleCreateSegment handles POST /api/v1/campaigns/:campaign_id/segments
func (h *Handler) HandleCreateSegment(c *gin.Context) {
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

	var req CreateSegmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	// Convert filter criteria
	filterCriteria, err := convertFilterCriteria(req.FilterCriteria)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filter criteria: " + err.Error()})
		return
	}

	processorReq := processor.CreateSegmentRequest{
		Name:           req.Name,
		Description:    req.Description,
		FilterCriteria: filterCriteria,
	}

	segment, err := h.processor.CreateSegment(ctx, accountID, campaignID, processorReq)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusCreated, segment)
}

// HandleListSegments handles GET /api/v1/campaigns/:campaign_id/segments
func (h *Handler) HandleListSegments(c *gin.Context) {
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

	segments, err := h.processor.ListSegments(ctx, accountID, campaignID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"segments":    segments,
		"total":       len(segments),
		"page":        1,
		"limit":       100,
		"total_pages": 1,
	})
}

// HandleGetSegment handles GET /api/v1/campaigns/:campaign_id/segments/:segment_id
func (h *Handler) HandleGetSegment(c *gin.Context) {
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

	// Get segment ID from path
	segmentIDStr := c.Param("segment_id")
	segmentID, err := uuid.Parse(segmentIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse segment ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment id"})
		return
	}

	segment, err := h.processor.GetSegment(ctx, accountID, campaignID, segmentID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, segment)
}

// UpdateSegmentRequest represents the HTTP request for updating a segment
type UpdateSegmentRequest struct {
	Name           *string                `json:"name,omitempty" binding:"omitempty,max=255"`
	Description    *string                `json:"description,omitempty"`
	FilterCriteria *FilterCriteriaRequest `json:"filter_criteria,omitempty"`
	Status         *string                `json:"status,omitempty" binding:"omitempty,oneof=active archived"`
}

// HandleUpdateSegment handles PUT /api/v1/campaigns/:campaign_id/segments/:segment_id
func (h *Handler) HandleUpdateSegment(c *gin.Context) {
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

	// Get segment ID from path
	segmentIDStr := c.Param("segment_id")
	segmentID, err := uuid.Parse(segmentIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse segment ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment id"})
		return
	}

	var req UpdateSegmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	processorReq := processor.UpdateSegmentRequest{
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
	}

	// Convert filter criteria if provided
	if req.FilterCriteria != nil {
		filterCriteria, err := convertFilterCriteria(*req.FilterCriteria)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filter criteria: " + err.Error()})
			return
		}
		processorReq.FilterCriteria = &filterCriteria
	}

	segment, err := h.processor.UpdateSegment(ctx, accountID, campaignID, segmentID, processorReq)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, segment)
}

// HandleDeleteSegment handles DELETE /api/v1/campaigns/:campaign_id/segments/:segment_id
func (h *Handler) HandleDeleteSegment(c *gin.Context) {
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

	// Get segment ID from path
	segmentIDStr := c.Param("segment_id")
	segmentID, err := uuid.Parse(segmentIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse segment ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment id"})
		return
	}

	err = h.processor.DeleteSegment(ctx, accountID, campaignID, segmentID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// PreviewSegmentRequest represents the HTTP request for previewing a segment
type PreviewSegmentRequest struct {
	FilterCriteria FilterCriteriaRequest `json:"filter_criteria" binding:"required"`
	SampleLimit    *int                  `json:"sample_limit,omitempty"`
}

// HandlePreviewSegment handles POST /api/v1/campaigns/:campaign_id/segments/preview
func (h *Handler) HandlePreviewSegment(c *gin.Context) {
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

	var req PreviewSegmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	// Convert filter criteria
	filterCriteria, err := convertFilterCriteria(req.FilterCriteria)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filter criteria: " + err.Error()})
		return
	}

	sampleLimit := 10
	if req.SampleLimit != nil {
		sampleLimit = *req.SampleLimit
	}

	processorReq := processor.PreviewSegmentRequest{
		FilterCriteria: filterCriteria,
		SampleLimit:    sampleLimit,
	}

	preview, err := h.processor.PreviewSegment(ctx, accountID, campaignID, processorReq)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, preview)
}

// HandleRefreshSegmentCount handles POST /api/v1/campaigns/:campaign_id/segments/:segment_id/refresh
func (h *Handler) HandleRefreshSegmentCount(c *gin.Context) {
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

	// Get segment ID from path
	segmentIDStr := c.Param("segment_id")
	segmentID, err := uuid.Parse(segmentIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse segment ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment id"})
		return
	}

	count, err := h.processor.RefreshSegmentCount(ctx, accountID, campaignID, segmentID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"segment_id":   segmentID,
		"user_count":   count,
		"refreshed_at": time.Now().UTC(),
	})
}

// Helper function to convert HTTP request filter criteria to store model
func convertFilterCriteria(req FilterCriteriaRequest) (store.SegmentFilterCriteria, error) {
	criteria := store.SegmentFilterCriteria{
		Statuses:      req.Statuses,
		Sources:       req.Sources,
		EmailVerified: req.EmailVerified,
		HasReferrals:  req.HasReferrals,
		MinReferrals:  req.MinReferrals,
		MinPosition:   req.MinPosition,
		MaxPosition:   req.MaxPosition,
		CustomFields:  req.CustomFields,
	}

	// Parse date strings
	if req.DateFrom != nil {
		t, err := parseDateTime(*req.DateFrom)
		if err != nil {
			return store.SegmentFilterCriteria{}, err
		}
		criteria.DateFrom = &t
	}

	if req.DateTo != nil {
		t, err := parseDateTime(*req.DateTo)
		if err != nil {
			return store.SegmentFilterCriteria{}, err
		}
		criteria.DateTo = &t
	}

	return criteria, nil
}

func parseDateTime(dateStr string) (time.Time, error) {
	// Try RFC3339 first
	t, err := time.Parse(time.RFC3339, dateStr)
	if err == nil {
		return t, nil
	}

	// Try date only format
	t, err = time.Parse("2006-01-02", dateStr)
	if err == nil {
		return t, nil
	}

	return time.Time{}, err
}

// ParseIntQuery parses an integer query parameter with a default value
func ParseIntQuery(c *gin.Context, key string, defaultValue int) int {
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
