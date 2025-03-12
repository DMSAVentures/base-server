package handler

import (
	"base-server/internal/observability"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) HandleUpdatePaymentMethod(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{"user_id", parsedUserID})

	clientSecret, err := h.processor.SetupPaymentMethodUpdateIntent(ctx, parsedUserID)
	if err != nil {
		h.logger.Error(ctx, "failed to update payment method", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"client_secret": clientSecret})
	return
}
