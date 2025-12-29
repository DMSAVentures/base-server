package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListPrices(c *gin.Context) {
	ctx := c.Request.Context()

	prices, err := h.processor.ListPrices(ctx)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, prices)
	return
}
