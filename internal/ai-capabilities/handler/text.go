package handler

import (
	"base-server/internal/ai-capabilities/processor"
	"base-server/internal/observability"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) HandleConversation(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := c.Get("User-ID")
	if !ok {
		h.logger.Error(ctx, "failed to get userID from context", nil)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id"})
		return
	}

	parsedUserID, err := uuid.Parse(userID.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse userID id", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id"})
		return
	}
	ctx = observability.WithFields(ctx, observability.Field{Key: "user_id", Value: parsedUserID.String()})

	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(ctx, "Failed to bind JSON request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.Message == "" {
		h.logger.Error(ctx, "Message is required", nil)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message is required"})
		return
	}

	w := c.Writer
	flusher, ok := w.(http.Flusher)

	if !ok {
		h.logger.Error(ctx, "Streaming unsupported: http.ResponseWriter does not implement http.Flusher", nil)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Streaming unsupported"})
		return
	}

	// Set required SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	h.logger.Info(ctx, "SSE connection starting")

	// Send an initial retry instruction
	if err := writeSSEMessage(w, flusher, "retry", "3000"); err != nil {
		h.logger.Warn(ctx, "Failed to send initial SSE message, client likely disconnected early")
		return
	}

	var responseChan <-chan processor.StreamResponse
	if req.ConversationID != uuid.Nil {
		ctx = observability.WithFields(ctx, observability.Field{Key: "conversation_id", Value: req.ConversationID.String()})
		responseChan, err = h.aiCapabilities.ContinueConversation(ctx, parsedUserID, req.ConversationID, req.Message)
		if err != nil {
			h.logger.Error(ctx, "Failed to continue conversation", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to continue conversation"})
			return
		}
	} else {
		responseChan, err = h.aiCapabilities.CreateConversation(ctx, parsedUserID, req.Message)
		if err != nil {
			h.logger.Error(ctx, "Failed to create conversation", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create conversation"})
			return
		}
	}

	for {
		select {
		case <-ctx.Done():
			h.logger.Info(ctx, "SSE connection closed by client or context cancelled")
			return

		case response, chanOk := <-responseChan:
			if !chanOk {
				_ = h.writeSSEEvent(ctx, w, flusher, "done", `[DONE]`)
				return
			}

			if response.Error != nil {
				h.logger.Error(ctx, "Error received from SSE data source", response.Error)
				errorPayload := SSEErrorPayload{Error: response.Error.Error()}
				errorData, marshalErr := json.Marshal(errorPayload)
				if marshalErr != nil {
					h.logger.Error(ctx, "Failed to marshal SSE error payload", marshalErr)
					fallback := `{"error":"internal error"}`
					_ = h.writeSSEEvent(ctx, w, flusher, "error", fallback)
					return
				}
				_ = h.writeSSEEvent(ctx, w, flusher, "error", string(errorData))
				continue
			}

			if writeErr := h.writeSSEEvent(ctx, w, flusher, "message", response.Content); writeErr != nil {
				h.logger.Warn(ctx, "Failed to write SSE token")
				return
			}

			if response.Completed {
				// Send final done event so client can close stream
				if writeErr := h.writeSSEEvent(ctx, w, flusher, "done", `[DONE]`); writeErr != nil {
					h.logger.Warn(ctx, "Failed to write SSE done event")
					return
				}
			}
		}
	}
}
