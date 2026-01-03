package handler

import (
	"base-server/internal/apierrors"
	"base-server/internal/observability"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CreateCheckoutSessionRequest struct {
	PriceID string `json:"price_id" binding:"required"`
}

func (h *Handler) HandleCancelSubscription(c *gin.Context) {
	ctx := c.Request.Context()

	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{Key: "user_id", Value: parsedUserID})

	err := h.processor.CancelSubscription(ctx, parsedUserID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "subscription cancelled"})
	return
}

func (h *Handler) HandleCreateCheckoutSession(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{Key: "user_id", Value: parsedUserID})

	var req CreateCheckoutSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	session, err := h.processor.CreateCheckoutSession(ctx, parsedUserID, req.PriceID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"client_secret": session.ClientSecret})
	return
}

func (h *Handler) GetCheckoutSession(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := c.Query("session_id")
	if sessionID == "" {
		apierrors.BadRequest(c, "INVALID_INPUT", "session_id is required")
		return
	}

	session, err := h.processor.GetCheckoutSession(ctx, sessionID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, session)
	return
}

func (h *Handler) HandleGetSubscription(c *gin.Context) {
	ctx := c.Request.Context()

	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{Key: "user_id", Value: parsedUserID})

	sub, err := h.processor.GetActiveSubscription(ctx, parsedUserID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, sub)
	return
}

func (h *Handler) HandleCreateCustomerPortal(c *gin.Context) {
	ctx := c.Request.Context()

	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{Key: "user_id", Value: parsedUserID})

	sessionURL, err := h.processor.CreateCustomerPortal(ctx, parsedUserID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": sessionURL})
}
