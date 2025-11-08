package handler

import (
	"base-server/internal/observability"
	"base-server/internal/referral/processor"
	"base-server/internal/store"
	"net/http"
	"strconv"

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
	var status *string
	if statusParam := c.Query("status"); statusParam != "" {
		status = &statusParam
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	req := processor.ListReferralsRequest{
		Status: status,
		Page:   page,
		Limit:  limit,
	}

	response, err := h.processor.ListReferrals(ctx, accountID, campaignID, req)
	if err != nil {
		h.logger.Error(ctx, "failed to list referrals", err)
		h.handleProcessorError(c, err)
		return
	}

	// Ensure referrals is never null - return empty array instead
	if response.Referrals == nil {
		response.Referrals = []store.Referral{}
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
		h.handleProcessorError(c, err)
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
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	req := processor.GetUserReferralsRequest{
		Page:  page,
		Limit: limit,
	}

	response, err := h.processor.GetUserReferrals(ctx, accountID, campaignID, userID, req)
	if err != nil {
		h.logger.Error(ctx, "failed to get user referrals", err)
		h.handleProcessorError(c, err)
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
		h.handleProcessorError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// handleProcessorError maps processor errors to HTTP responses
func (h *Handler) handleProcessorError(c *gin.Context, err error) {
	switch err {
	case processor.ErrReferralNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": "referral not found"})
	case processor.ErrCampaignNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": "campaign not found"})
	case processor.ErrUserNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
	case processor.ErrUnauthorized:
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access to campaign"})
	case processor.ErrInvalidStatus:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid referral status"})
	case processor.ErrInvalidReferral:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid referral code"})
	case processor.ErrReferralCodeEmpty:
		c.JSON(http.StatusBadRequest, gin.H{"error": "referral code is required"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
