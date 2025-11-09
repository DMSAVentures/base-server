package handler

import (
	"base-server/internal/analytics/processor"
	"base-server/internal/apierrors"
	"base-server/internal/observability"
	"net/http"
	"time"

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

// HandleGetAnalyticsOverview retrieves high-level analytics for a campaign
func (h *Handler) HandleGetAnalyticsOverview(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, h.logger, apierrors.Unauthorized("account ID not found in context"))
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
		apierrors.RespondWithError(c, h.logger, err)
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
		apierrors.RespondWithError(c, h.logger, apierrors.Unauthorized("account ID not found in context"))
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
		apierrors.RespondWithError(c, h.logger, err)
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
		apierrors.RespondWithError(c, h.logger, apierrors.Unauthorized("account ID not found in context"))
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
		apierrors.RespondWithError(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, referrals)
}

// HandleGetTimeSeriesAnalytics retrieves time-series analytics data
func (h *Handler) HandleGetTimeSeriesAnalytics(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, h.logger, apierrors.Unauthorized("account ID not found in context"))
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

	// Parse granularity parameter (default: day)
	granularity := c.DefaultQuery("granularity", "day")

	// Get time series analytics
	timeSeries, err := h.processor.GetTimeSeriesAnalytics(ctx, accountID, campaignID, dateFrom, dateTo, granularity)
	if err != nil {
		apierrors.RespondWithError(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, timeSeries)
}

// HandleGetSourceAnalytics retrieves traffic source breakdown
func (h *Handler) HandleGetSourceAnalytics(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.RespondWithError(c, h.logger, apierrors.Unauthorized("account ID not found in context"))
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
		apierrors.RespondWithError(c, h.logger, err)
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
		apierrors.RespondWithError(c, h.logger, apierrors.Unauthorized("account ID not found in context"))
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
		apierrors.RespondWithError(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, funnel)
}
