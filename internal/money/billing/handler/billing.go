package handler

import (
	"errors"

	"base-server/internal/apierrors"
	"base-server/internal/money/billing/processor"
	"base-server/internal/observability"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	processor processor.BillingProcessor
	logger    *observability.Logger
}

func New(processor processor.BillingProcessor, logger *observability.Logger) Handler {
	return Handler{processor: processor, logger: logger}
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, processor.ErrNoActiveSubscription):
		apierrors.NotFound(c, "No active subscription found")
	default:
		apierrors.InternalError(c, err)
	}
}
