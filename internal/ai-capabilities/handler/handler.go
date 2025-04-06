package handler

import (
	"base-server/internal/ai-capabilities"
	"base-server/internal/observability"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	aiCapabilities *ai_capabilities.GeminiAI
	logger         *observability.Logger
}

func New(aiCapabilities *ai_capabilities.GeminiAI, logger *observability.Logger) Handler {
	return Handler{
		aiCapabilities: aiCapabilities,
		logger:         logger,
	}
}

func (h *Handler) HandleRequest(c *gin.Context) {
	// Implement the logic to handle the request using aiCapabilities
	ctx := c.Request.Context()

	ctx = observability.WithFields(ctx, observability.Field{Key: "request", Value: "example request"})

	h.logger.Info(ctx, "Handling request")

	// Example of calling a method on aiCapabilities
	h.aiCapabilities.DoSomething(ctx)

	// Return a response (this is just an example)
	c.JSON(200, gin.H{"message": "Request handled successfully"})
}
