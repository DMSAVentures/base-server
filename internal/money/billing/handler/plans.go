package handler

import (
	"base-server/internal/apierrors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListPrices(c *gin.Context) {
	ctx := c.Request.Context()

	prices, err := h.processor.ListPrices(ctx)
	if err != nil {
		apierrors.RespondWithError(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, prices)
	return
}
