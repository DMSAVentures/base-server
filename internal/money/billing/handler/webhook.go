package handler

import (
	"base-server/internal/apierrors"
	"base-server/internal/observability"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79/webhook"
)

func (h *Handler) HandleGetPaymentMethod(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{Key: "user_id", Value: parsedUserID})

	paymentMethod, err := h.processor.GetPaymentMethodForUser(ctx, parsedUserID)
	if err != nil {
		h.handleError(c, err)
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
		apierrors.BadRequest(c, "INVALID_INPUT", "failed to read request body")
		return
	}

	// Retrieve the Stripe-Signature header
	signatureHeader := c.GetHeader("Stripe-Signature")
	if signatureHeader == "" {
		apierrors.BadRequest(c, "INVALID_INPUT", "missing Stripe-Signature header")
		return
	}
	event, err := webhook.ConstructEvent(payload, signatureHeader, h.processor.WebhookSecret)
	if err != nil {
		apierrors.BadRequest(c, "INVALID_INPUT", "invalid webhook signature")
		return
	}
	// Handle the event

	err = h.processor.HandleWebhook(ctx, event)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
	return
}
