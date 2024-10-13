package handler

import (
	"base-server/internal/money/billing/processor"
	"base-server/internal/observability"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CreatePaymentIntentRequest struct {
	Items []processor.PaymentIntentItem `json:"items" binding:"required"`
}

type CreateSubscriptionRequest struct {
	PriceID string `json:"price_id" binding:"required"`
}

type UpdatePaymentMethodRequest struct {
	PaymentMethodID string `json:"payment_method_id" binding:"required"`
}

type CreateCheckoutSessionRequest struct {
	PriceID string `json:"price_id" binding:"required"`
}

// Create Stripe payment intent
func (h *Handler) HandleCreatePaymentIntent(c *gin.Context) {
	ctx := c.Request.Context()
	var req CreatePaymentIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{"user_id", parsedUserID})

	clientSecret, err := h.processor.CreateStripePaymentIntent(ctx, req.Items)
	if err != nil {
		h.logger.Error(ctx, "failed to create payment intent", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"client_secret": clientSecret})
	return
}

func (h *Handler) HandleCreateSubscriptionIntent(c *gin.Context) {
	ctx := c.Request.Context()
	var req CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{"user_id", parsedUserID})

	clientSecret, err := h.processor.CreateSubscriptionIntent(ctx, parsedUserID, req.PriceID)
	if err != nil {
		h.logger.Error(ctx, "failed to create subscription", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"client_secret": clientSecret})
	return
}

func (h *Handler) HandleCreateCheckoutSession(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{"user_id", parsedUserID})

	var req CreateCheckoutSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	session, err := h.processor.CreateCheckoutSession(ctx, parsedUserID, req.PriceID)
	if err != nil {
		h.logger.Error(ctx, "failed to create checkout session", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"client_secret": session.ClientSecret})
	return
}

func (h *Handler) GetCheckoutSession(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := c.Query("session_id")
	if sessionID == "" {
		h.logger.Error(ctx, "session_id is required", nil)
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	session, err := h.processor.GetCheckoutSession(ctx, sessionID)
	if err != nil {
		h.logger.Error(ctx, "failed to get checkout session", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
	return
}
