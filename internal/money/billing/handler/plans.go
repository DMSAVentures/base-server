package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListPrices(c *gin.Context) {
	ctx := c.Request.Context()

	prices, err := h.processor.ListPrices(ctx)
	if err != nil {
		h.logger.Error(ctx, "failed to list prices", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, prices)
	return
}
