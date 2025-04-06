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

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Get the response channel from AI capabilities
	responseChan := h.aiCapabilities.DoSomething(ctx)

	// Stream responses to client
	c.Stream(func(w io.Writer) bool {
        select {
        case response, ok := <-responseChan:
            if !ok {
                return false
            }
            
            if response.Error != nil {
                c.SSEvent("error", gin.H{
                    "error": response.Error.Error(),
                })
                return false
            }
            
            c.SSEvent("message", response.Content)
            return true
            
        case <-ctx.Done():
            return false
        }
    })
}
