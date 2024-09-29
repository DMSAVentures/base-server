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
