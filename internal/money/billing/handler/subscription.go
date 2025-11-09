package handler

import (
	"base-server/internal/apierrors"
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
		apierrors.RespondWithValidationError(c, err)
		return
	}

	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{"user_id", parsedUserID})

	clientSecret, err := h.processor.CreateStripePaymentIntent(ctx, req.Items)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"client_secret": clientSecret})
	return
}

func (h *Handler) HandleCreateSubscriptionIntent(c *gin.Context) {
	ctx := c.Request.Context()
	var req CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{"user_id", parsedUserID})

	clientSecret, err := h.processor.CreateSubscriptionIntent(ctx, parsedUserID, req.PriceID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"client_secret": clientSecret})
	return
}

func (h *Handler) HandleCancelSubscription(c *gin.Context) {
	ctx := c.Request.Context()

	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{"user_id", parsedUserID})

	err := h.processor.CancelSubscription(ctx, parsedUserID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "subscription cancelled"})
	return
}

func (h *Handler) HandleCreateCheckoutSession(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{"user_id", parsedUserID})

	var req CreateCheckoutSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	session, err := h.processor.CreateCheckoutSession(ctx, parsedUserID, req.PriceID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"client_secret": session.ClientSecret})
	return
}

func (h *Handler) GetCheckoutSession(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := c.Query("session_id")
	if sessionID == "" {
		apierrors.RespondWithError(c, apierrors.BadRequest(apierrors.CodeInvalidInput, "session_id is required"))
		return
	}

	session, err := h.processor.GetCheckoutSession(ctx, sessionID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, session)
	return
}

func (h *Handler) HandleGetSubscription(c *gin.Context) {
	ctx := c.Request.Context()

	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{"user_id", parsedUserID})

	sub, err := h.processor.GetActiveSubscription(ctx, parsedUserID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, sub)
	return
}

func (h *Handler) HandleCreateCustomerPortal(c *gin.Context) {
	ctx := c.Request.Context()

	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{"user_id", parsedUserID})

	sessionURL, err := h.processor.CreateCustomerPortal(ctx, parsedUserID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": sessionURL})
	return
}
