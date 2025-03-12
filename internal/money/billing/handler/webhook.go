package handler

import (
	"base-server/internal/observability"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79/webhook"
)

func (h *Handler) HandleUpdateSubscription(c *gin.Context) {
	ctx := c.Request.Context()

	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{"user_id", parsedUserID})

	var req CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	err := h.processor.UpdateSubscription(ctx, parsedUserID, req.PriceID)
	if err != nil {
		h.logger.Error(ctx, "failed to update subscription", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
	return
}

func (h *Handler) HandleUpdatePaymentMethod(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{"user_id", parsedUserID})

	var paymentMethodReq UpdatePaymentMethodRequest
	if err := c.ShouldBindJSON(&paymentMethodReq); err != nil {
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	err := h.processor.UpdatePaymentMethodForUser(ctx, paymentMethodReq.PaymentMethodID, parsedUserID)
	if err != nil {
		h.logger.Error(ctx, "failed to update payment method", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
	return

}

func (h *Handler) HandleGetPaymentMethod(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{"user_id", parsedUserID})

	paymentMethod, err := h.processor.GetPaymentMethodForUser(ctx, parsedUserID)
	if err != nil {
		h.logger.Error(ctx, "failed to get payment method", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, paymentMethod)
	return
}

func (h *Handler) HandleWebhook(c *gin.Context) {
	ctx := c.Request.Context()

	// Read the request body
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error(ctx, "failed to read request body", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Retrieve the Stripe-Signature header
	signatureHeader := c.GetHeader("Stripe-Signature")
	if signatureHeader == "" {
		h.logger.Error(ctx, "missing Stripe-Signature header", nil)
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing Stripe-Signature header"})
		return
	}
	event, err := webhook.ConstructEvent(payload, signatureHeader, h.processor.WebhookSecret)
	if err != nil {
		h.logger.Error(ctx, "failed to construct event", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
	}
	// Handle the event

	err = h.processor.HandleWebhook(ctx, event)
	if err != nil {
		h.logger.Error(ctx, "failed to handle webhook", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
	return
}
