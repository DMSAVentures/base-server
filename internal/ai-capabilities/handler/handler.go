package handler

import (
	"base-server/internal/ai-capabilities"
	"base-server/internal/observability"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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

type SSEErrorPayload struct {
	Error string `json:"error"`
}

func (h *Handler) HandleRequest(c *gin.Context) {
	ctx := c.Request.Context()
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

	responseChan := h.aiCapabilities.DoSomething(ctx)

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

// writeSSEEvent sends a full SSE event with event name and data block.
func (h *Handler) writeSSEEvent(ctx context.Context, w io.Writer, f http.Flusher, event, data string) error {
	d := strings.Trim(data, "\"")
	message := fmt.Sprintf("event: %s\ndata: %s\n", event, d)
	if _, err := io.WriteString(w, message); err != nil {
		return fmt.Errorf("error writing SSE event: %w", err)
	}
	f.Flush()
	return nil
}

// writeSSEMessage sends a generic SSE field, like retry, without an event name.
func writeSSEMessage(w io.Writer, f http.Flusher, field, value string) error {
	if _, err := io.WriteString(w, fmt.Sprintf("%s: %s\n\n", field, value)); err != nil {
		return fmt.Errorf("error writing SSE message: %w", err)
	}
	f.Flush()
	return nil
}

// tokenize splits content into individual words for simulation.
// You can improve this with a tokenizer that includes punctuation as separate tokens.
func tokenize(content string) []string {
	return strings.Fields(content)
}
