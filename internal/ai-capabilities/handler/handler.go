package handler

import (
	"base-server/internal/ai-capabilities/processor"
	"base-server/internal/apierrors"
	"base-server/internal/observability"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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

// handleError maps processor errors to appropriate API error responses
func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, processor.ErrConversationNotFound):
		apierrors.NotFound(c, "Conversation not found")
	case errors.Is(err, processor.ErrUnauthorized):
		apierrors.Forbidden(c, "FORBIDDEN", "You do not have access to this conversation")
	case errors.Is(err, processor.ErrAIServiceError):
		apierrors.ServiceUnavailable(c, "AI_SERVICE_ERROR", "AI service is temporarily unavailable", err)
	default:
		apierrors.InternalError(c, err)
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
