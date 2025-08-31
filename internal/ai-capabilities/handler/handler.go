package handler

import (
	"base-server/internal/ai-capabilities/processor"
	"base-server/internal/observability"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type Handler struct {
	aiCapabilities *processor.AIProcessor
	logger         *observability.Logger
}

func New(aiCapabilities *processor.AIProcessor, logger *observability.Logger) Handler {
	return Handler{
		aiCapabilities: aiCapabilities,
		logger:         logger,
	}
}

type SSEErrorPayload struct {
	Error string `json:"error"`
}

type CreateConversationRequest struct {
	Message        string    `json:"message"`
	ConversationID uuid.UUID `json:"conversation_id"`
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
