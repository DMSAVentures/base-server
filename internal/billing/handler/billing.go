package handler

import (
	"base-server/internal/billing/processor"
	"base-server/internal/observability"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79/webhook"
)

type Handler struct {
	processor processor.BillingProcessor
	logger    *observability.Logger
}

type CreatePaymentIntentRequest struct {
	Items []processor.PaymentIntentItem `json:"items" binding:"required"`
}

type CreateSubscriptionRequest struct {
	PriceID string `json:"price_id" binding:"required"`
}

func New(processor processor.BillingProcessor, logger *observability.Logger) Handler {
	return Handler{processor: processor, logger: logger}
}

// Create Stripe payment intent
func (h *Handler) HandleCreatePaymentIntent(c *gin.Context) {
	ctx := c.Request.Context()
	var req CreatePaymentIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, exists := c.Get("User-ID")
	if !exists {
		h.logger.Error(ctx, "User-ID does not exist on context", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	parsedUserID, err := uuid.Parse(userID.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse userID", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	clientSecret, err := h.processor.CreateSubscription(ctx, parsedUserID, req.PriceID)
	if err != nil {
		h.logger.Error(ctx, "failed to create subscription", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"client_secret": clientSecret})
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
