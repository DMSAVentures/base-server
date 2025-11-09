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

func (h *Handler) HandleUpdateSubscription(c *gin.Context) {
	ctx := c.Request.Context()

	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{"user_id", parsedUserID})

	var req CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, h.logger, err)
		return
	}

	err := h.processor.UpdateSubscription(ctx, parsedUserID, req.PriceID)
	if err != nil {
		apierrors.RespondWithError(c, h.logger, err)
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
		apierrors.RespondWithError(c, h.logger, err)
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
		apierrors.RespondWithError(c, h.logger, apierrors.BadRequest(apierrors.CodeInvalidInput, "failed to read request body"))
		return
	}

	// Retrieve the Stripe-Signature header
	signatureHeader := c.GetHeader("Stripe-Signature")
	if signatureHeader == "" {
		apierrors.RespondWithError(c, h.logger, apierrors.BadRequest(apierrors.CodeInvalidInput, "missing Stripe-Signature header"))
		return
	}
	event, err := webhook.ConstructEvent(payload, signatureHeader, h.processor.WebhookSecret)
	if err != nil {
		apierrors.RespondWithError(c, h.logger, apierrors.BadRequest(apierrors.CodeInvalidInput, "invalid webhook signature"))
		return
	}
	// Handle the event

	err = h.processor.HandleWebhook(ctx, event)
	if err != nil {
		apierrors.RespondWithError(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
	return
}
