package handler

import (
	"base-server/internal/apierrors"
	"base-server/internal/observability"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) HandleUpdatePaymentMethod(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.MustGet("User-ID")
	parsedUserID := uuid.MustParse(userID.(string))
	ctx = observability.WithFields(ctx, observability.Field{Key: "user_id", Value: parsedUserID})

	clientSecret, err := h.processor.SetupPaymentMethodUpdateIntent(ctx, parsedUserID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"client_secret": clientSecret})
	return
}
