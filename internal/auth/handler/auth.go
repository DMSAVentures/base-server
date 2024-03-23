package handler

import (
	"base-server/internal/auth/processor"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	authProcessor processor.AuthProcessor
}

func New(authProcessor processor.AuthProcessor) Handler {
	return Handler{authProcessor: authProcessor}
}

func (h *Handler) HandleLogin(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"message": "FOUND IT",
	})
	return
}
