package handler

import (
	"base-server/internal/observability"
	"base-server/internal/referral/processor"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	processor processor.ReferralProcessor
	logger    *observability.Logger
	baseURL   string
}

func New(processor processor.ReferralProcessor, logger *observability.Logger, baseURL string) Handler {
	return Handler{
		processor: processor,
		logger:    logger,
		baseURL:   baseURL,
	}
}

// HandleListReferrals handles GET /api/v1/campaigns/:campaign_id/referrals
func (h *Handler) HandleListReferrals(c *gin.Context) {
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

	req := processor.ListReferralsRequest{
		Status: status,
		Page:   page,
		Limit:  limit,
	}

	response, err := h.processor.ListReferrals(ctx, accountID, campaignID, req)
	if err != nil {
		h.logger.Error(ctx, "failed to list referrals", err)
		if errors.Is(err, processor.ErrCampaignNotFound) || errors.Is(err, processor.ErrUnauthorized) {
			c.JSON(http.StatusNotFound, gin.H{"error": "campaign not found"})
			return
		}
		if errors.Is(err, processor.ErrInvalidStatus) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// TrackReferralRequest represents the HTTP request for tracking a referral
type TrackReferralRequest struct {
	ReferralCode string  `json:"referral_code" binding:"required"`
	Source       *string `json:"source,omitempty"`
}

// HandleTrackReferral handles POST /api/v1/campaigns/:campaign_id/referrals/track
func (h *Handler) HandleTrackReferral(c *gin.Context) {
	ctx := c.Request.Context()

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	var req TrackReferralRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Get IP address
	ipAddress := c.ClientIP()

	processorReq := processor.TrackReferralRequest{
		ReferralCode: req.ReferralCode,
		Source:       req.Source,
		IPAddress:    &ipAddress,
	}

	response, err := h.processor.TrackReferral(ctx, campaignID, processorReq)
	if err != nil {
		h.logger.Error(ctx, "failed to track referral", err)
		if errors.Is(err, processor.ErrInvalidReferral) || errors.Is(err, processor.ErrReferralCodeEmpty) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid referral code"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleGetUserReferrals handles GET /api/v1/campaigns/:campaign_id/users/:user_id/referrals
func (h *Handler) HandleGetUserReferrals(c *gin.Context) {
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

	// Get user ID from path
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse user ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
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

	req := processor.GetUserReferralsRequest{
		Page:  page,
		Limit: limit,
	}

	response, err := h.processor.GetUserReferrals(ctx, accountID, campaignID, userID, req)
	if err != nil {
		h.logger.Error(ctx, "failed to get user referrals", err)
		if errors.Is(err, processor.ErrUserNotFound) || errors.Is(err, processor.ErrCampaignNotFound) || errors.Is(err, processor.ErrUnauthorized) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleGetReferralLink handles GET /api/v1/campaigns/:campaign_id/users/:user_id/referral-link
func (h *Handler) HandleGetReferralLink(c *gin.Context) {
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

	// Get user ID from path
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse user ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	response, err := h.processor.GetReferralLink(ctx, accountID, campaignID, userID, h.baseURL)
	if err != nil {
		h.logger.Error(ctx, "failed to get referral link", err)
		if errors.Is(err, processor.ErrUserNotFound) || errors.Is(err, processor.ErrCampaignNotFound) || errors.Is(err, processor.ErrUnauthorized) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}
