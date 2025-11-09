package handler

import (
	"base-server/internal/ai-capabilities/processor"
	"base-server/internal/apierrors"
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
		apierrors.RespondWithError(c, apierrors.Unauthorized("User ID not found in context"))
		return
	}

	parsedUserID, err := uuid.Parse(userID.(string))
	if err != nil {
		apierrors.RespondWithError(c, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid user ID format"))
		return
	}
	ctx = observability.WithFields(ctx, observability.Field{Key: "user_id", Value: parsedUserID.String()})

	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	if req.Message == "" {
		apierrors.RespondWithError(c, apierrors.BadRequest(apierrors.CodeInvalidInput, "Message is required"))
		return
	}

	w := c.Writer
	flusher, ok := w.(http.Flusher)

	if !ok {
		apierrors.RespondWithError(c, apierrors.InternalError(nil))
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
			apierrors.RespondWithError(c, err)
			return
		}
	} else {
		responseChan, err = h.aiCapabilities.CreateConversation(ctx, parsedUserID, req.Message)
		if err != nil {
			apierrors.RespondWithError(c, err)
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
