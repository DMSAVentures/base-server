package handler

import (
	"base-server/internal/observability"
	"base-server/internal/voicecall/processor"
	"net/http"

	"github.com/gorilla/websocket"
)

type Handler struct {
	voiceProcessor *processor.VoiceCallProcessor
	logger         *observability.Logger
}

func New(voiceProcessor *processor.VoiceCallProcessor, logger *observability.Logger) Handler {
	return Handler{
		voiceProcessor: voiceProcessor,
		logger:         logger,
	}
}

// upgrader is a shared WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Add proper origin validation for production
		return true
	},
}
