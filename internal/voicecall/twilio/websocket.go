package twilio

import (
	"base-server/internal/observability"
	"base-server/internal/voice/audio"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
)

type MediaEvent struct {
	Event string `json:"event"`
	Start struct {
		StreamSid string `json:"streamSid"`
	} `json:"start,omitempty"`
	Media struct {
		Payload string `json:"payload"`
	} `json:"media,omitempty"`
	Stop struct {
		StreamSid string `json:"streamSid"`
	} `json:"stop,omitempty"`
}

type WebSocketHandler struct {
	conn       *websocket.Conn
	logger     *observability.Logger
	streamSid  string
	writeMutex sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewWebSocketHandler(conn *websocket.Conn, logger *observability.Logger) *WebSocketHandler {
	ctx, cancel := context.WithCancel(context.Background())
	return &WebSocketHandler{
		conn:   conn,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (h *WebSocketHandler) Start(ctx context.Context, audioIn chan<- []byte, audioOut <-chan []byte) error {
	// Merge contexts
	handlerCtx, cancel := context.WithCancel(ctx)
	h.ctx = handlerCtx
	oldCancel := h.cancel
	h.cancel = func() {
		cancel()
		oldCancel()
	}

	// Start goroutine to send audio to Twilio
	go h.sendAudioToTwilio(audioOut)

	// Start receiving audio from Twilio
	go h.receiveAudioFromTwilio(audioIn)

	return nil
}

func (h *WebSocketHandler) receiveAudioFromTwilio(audioIn chan<- []byte) {
	defer close(audioIn)

	for {
		select {
		case <-h.ctx.Done():
			h.logger.Info(h.ctx, "WebSocket receive stopped: context cancelled")
			return
		default:
			_, msg, err := h.conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					h.logger.Info(h.ctx, "WebSocket closed normally")
				} else {
					h.logger.Error(h.ctx, "WebSocket read error", err)
				}
				return
			}

			var event MediaEvent
			if err := json.Unmarshal(msg, &event); err != nil {
				h.logger.Error(h.ctx, "Failed to parse Twilio event", err)
				continue
			}

			switch event.Event {
			case "start":
				h.streamSid = event.Start.StreamSid
				h.logger.Info(h.ctx, fmt.Sprintf("Twilio stream started: %s", h.streamSid))

			case "media":
				audioBytes, err := audio.Base64ToBytes(event.Media.Payload)
				if err != nil {
					h.logger.Error(h.ctx, "Failed to decode audio", err)
					continue
				}

				// Send audio to pipeline (non-blocking)
				select {
				case audioIn <- audioBytes:
					// Successfully sent
				case <-h.ctx.Done():
					return
				default:
					h.logger.Warn(h.ctx, "Audio input buffer full, dropping chunk")
				}

			case "stop":
				h.logger.Info(h.ctx, fmt.Sprintf("Twilio stream stopped: %s", event.Stop.StreamSid))
				return

			default:
				h.logger.Debug(h.ctx, fmt.Sprintf("Unknown Twilio event: %s", event.Event))
			}
		}
	}
}

func (h *WebSocketHandler) sendAudioToTwilio(audioOut <-chan []byte) {
	for {
		select {
		case <-h.ctx.Done():
			h.logger.Info(h.ctx, "WebSocket send stopped: context cancelled")
			return

		case audioData, ok := <-audioOut:
			if !ok {
				h.logger.Info(h.ctx, "Audio output channel closed")
				return
			}

			// Convert to base64 for Twilio WebSocket
			audioBase64 := audio.BytesToBase64(audioData)

			// Create Twilio media message
			mediaMsg := map[string]interface{}{
				"event":     "media",
				"streamSid": h.streamSid,
				"media": map[string]string{
					"payload": audioBase64,
				},
			}

			msgBytes, err := json.Marshal(mediaMsg)
			if err != nil {
				h.logger.Error(h.ctx, "Failed to marshal media message", err)
				continue
			}

			// Send with mutex protection
			h.writeMutex.Lock()
			err = h.conn.WriteMessage(websocket.TextMessage, msgBytes)
			h.writeMutex.Unlock()

			if err != nil {
				h.logger.Error(h.ctx, "Failed to send audio to Twilio", err)
				return
			}

			h.logger.Debug(h.ctx, fmt.Sprintf("Sent %d bytes to Twilio", len(audioData)))
		}
	}
}

func (h *WebSocketHandler) Stop() {
	h.logger.Info(h.ctx, "Stopping Twilio WebSocket handler")
	h.cancel()

	// Send close message to Twilio
	h.writeMutex.Lock()
	h.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	h.writeMutex.Unlock()

	h.conn.Close()
}

func (h *WebSocketHandler) GetStreamSID() string {
	return h.streamSid
}
