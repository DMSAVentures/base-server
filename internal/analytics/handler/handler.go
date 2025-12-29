package handler

import (
	"errors"
	"net/http"
	"time"

	"base-server/internal/analytics/processor"
	"base-server/internal/apierrors"
	"base-server/internal/observability"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	processor processor.AnalyticsProcessor
	logger    *observability.Logger
}

func New(processor processor.AnalyticsProcessor, logger *observability.Logger) Handler {
	return Handler{
		processor: processor,
		logger:    logger,
	}
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, processor.ErrCampaignNotFound):
		apierrors.NotFound(c, "Campaign not found")
	case errors.Is(err, processor.ErrUnauthorized):
		apierrors.Forbidden(c, "FORBIDDEN", "You do not have access to this campaign")
	case errors.Is(err, processor.ErrInvalidDateRange):
		apierrors.BadRequest(c, "INVALID_DATE_RANGE", "Invalid date range")
	case errors.Is(err, processor.ErrInvalidGranularity):
		apierrors.BadRequest(c, "INVALID_GRANULARITY", "Invalid granularity")
	default:
		apierrors.InternalError(c, err)
	}
}

// HandleGetAnalyticsOverview retrieves high-level analytics for a campaign
func (h *Handler) HandleGetAnalyticsOverview(c *gin.Context) {
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

	// Get analytics overview
	overview, err := h.processor.GetAnalyticsOverview(ctx, accountID, campaignID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, overview)
}

// HandleGetConversionAnalytics retrieves conversion funnel analytics
func (h *Handler) HandleGetConversionAnalytics(c *gin.Context) {
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

	// Parse date range parameters
	var dateFrom, dateTo *time.Time
	if dateFromStr := c.Query("date_from"); dateFromStr != "" {
		parsed, err := time.Parse(time.RFC3339, dateFromStr)
		if err != nil {
			h.logger.Error(ctx, "failed to parse date_from", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date_from format, use RFC3339"})
			return
		}
		dateFrom = &parsed
	}

	if dateToStr := c.Query("date_to"); dateToStr != "" {
		parsed, err := time.Parse(time.RFC3339, dateToStr)
		if err != nil {
			h.logger.Error(ctx, "failed to parse date_to", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date_to format, use RFC3339"})
			return
		}
		dateTo = &parsed
	}

	// Get conversion analytics
	conversions, err := h.processor.GetConversionAnalytics(ctx, accountID, campaignID, dateFrom, dateTo)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, conversions)
}

// HandleGetReferralAnalytics retrieves referral performance analytics
func (h *Handler) HandleGetReferralAnalytics(c *gin.Context) {
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

	// Parse date range parameters
	var dateFrom, dateTo *time.Time
	if dateFromStr := c.Query("date_from"); dateFromStr != "" {
		parsed, err := time.Parse(time.RFC3339, dateFromStr)
		if err != nil {
			h.logger.Error(ctx, "failed to parse date_from", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date_from format, use RFC3339"})
			return
		}
		dateFrom = &parsed
	}

	if dateToStr := c.Query("date_to"); dateToStr != "" {
		parsed, err := time.Parse(time.RFC3339, dateToStr)
		if err != nil {
			h.logger.Error(ctx, "failed to parse date_to", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date_to format, use RFC3339"})
			return
		}
		dateTo = &parsed
	}

	// Get referral analytics
	referrals, err := h.processor.GetReferralAnalytics(ctx, accountID, campaignID, dateFrom, dateTo)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, referrals)
}

// HandleGetSignupsOverTime retrieves signups over time for charts
func (h *Handler) HandleGetSignupsOverTime(c *gin.Context) {
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

	// Parse date range parameters
	var dateFrom, dateTo *time.Time
	if dateFromStr := c.Query("from"); dateFromStr != "" {
		parsed, err := time.Parse(time.RFC3339, dateFromStr)
		if err != nil {
			h.logger.Error(ctx, "failed to parse from", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from format, use RFC3339"})
			return
		}
		dateFrom = &parsed
	}

	if dateToStr := c.Query("to"); dateToStr != "" {
		parsed, err := time.Parse(time.RFC3339, dateToStr)
		if err != nil {
			h.logger.Error(ctx, "failed to parse to", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to format, use RFC3339"})
			return
		}
		dateTo = &parsed
	}

	// Parse period parameter (default: day)
	period := c.DefaultQuery("period", "day")

	// Get signups over time
	response, err := h.processor.GetSignupsOverTime(ctx, accountID, campaignID, dateFrom, dateTo, period)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleGetSignupsBySource retrieves signups by source for stacked bar charts
func (h *Handler) HandleGetSignupsBySource(c *gin.Context) {
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

	// Parse date range parameters
	var dateFrom, dateTo *time.Time
	if dateFromStr := c.Query("from"); dateFromStr != "" {
		parsed, err := time.Parse(time.RFC3339, dateFromStr)
		if err != nil {
			h.logger.Error(ctx, "failed to parse from", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from format, use RFC3339"})
			return
		}
		dateFrom = &parsed
	}

	if dateToStr := c.Query("to"); dateToStr != "" {
		parsed, err := time.Parse(time.RFC3339, dateToStr)
		if err != nil {
			h.logger.Error(ctx, "failed to parse to", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to format, use RFC3339"})
			return
		}
		dateTo = &parsed
	}

	// Parse period parameter (default: day)
	period := c.DefaultQuery("period", "day")

	// Get signups by source
	response, err := h.processor.GetSignupsBySource(ctx, accountID, campaignID, dateFrom, dateTo, period)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleGetSourceAnalytics retrieves traffic source breakdown
func (h *Handler) HandleGetSourceAnalytics(c *gin.Context) {
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

	// Parse date range parameters
	var dateFrom, dateTo *time.Time
	if dateFromStr := c.Query("date_from"); dateFromStr != "" {
		parsed, err := time.Parse(time.RFC3339, dateFromStr)
		if err != nil {
			h.logger.Error(ctx, "failed to parse date_from", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date_from format, use RFC3339"})
			return
		}
		dateFrom = &parsed
	}

	if dateToStr := c.Query("date_to"); dateToStr != "" {
		parsed, err := time.Parse(time.RFC3339, dateToStr)
		if err != nil {
			h.logger.Error(ctx, "failed to parse date_to", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date_to format, use RFC3339"})
			return
		}
		dateTo = &parsed
	}

	// Get source analytics
	sources, err := h.processor.GetSourceAnalytics(ctx, accountID, campaignID, dateFrom, dateTo)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, sources)
}

// HandleGetFunnelAnalytics retrieves conversion funnel visualization data
func (h *Handler) HandleGetFunnelAnalytics(c *gin.Context) {
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

	// Parse date range parameters
	var dateFrom, dateTo *time.Time
	if dateFromStr := c.Query("date_from"); dateFromStr != "" {
		parsed, err := time.Parse(time.RFC3339, dateFromStr)
		if err != nil {
			h.logger.Error(ctx, "failed to parse date_from", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date_from format, use RFC3339"})
			return
		}
		dateFrom = &parsed
	}

	if dateToStr := c.Query("date_to"); dateToStr != "" {
		parsed, err := time.Parse(time.RFC3339, dateToStr)
		if err != nil {
			h.logger.Error(ctx, "failed to parse date_to", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date_to format, use RFC3339"})
			return
		}
		dateTo = &parsed
	}

	// Get funnel analytics
	funnel, err := h.processor.GetFunnelAnalytics(ctx, accountID, campaignID, dateFrom, dateTo)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, funnel)
}
