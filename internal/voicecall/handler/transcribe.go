package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/twilio/twilio-go/twiml"
)

func (h *Handler) HandleAnswerPhone(c *gin.Context) {
	// Get the host from the request to build the WebSocket URL
	host := c.Request.Host

	// Build WebSocket URL - use wss:// for HTTPS, ws:// for HTTP
	protocol := "ws"

	// The WebSocket server is on port 8081, but when using ngrok or a proxy,
	// the external URL might be different
	wsURL := " https://cd5f83cca865.ngrok-free.app/api/phone/media-stream" // Default for local testing
	if wsURL == "" {
		// Use the same host but with WebSocket protocol
		// Assuming the WebSocket server is exposed on the same domain
		wsURL = fmt.Sprintf("%s://%s/api/phone/media-stream", protocol, host)
	}

	h.logger.Info(c.Request.Context(), fmt.Sprintf("TwiML WebSocket URL: %s", wsURL))

	say := &twiml.VoiceSay{
		Message: "Hello! Starting transcription. Please speak.",
	}
	stream := twiml.VoiceStream{
		Name: "media-stream",
		Url:  "wss://cd5f83cca865.ngrok-free.app/api/phone/media-stream",
	}
	connect := twiml.VoiceConnect{
		InnerElements:      []twiml.Element{stream},
		OptionalAttributes: nil,
	}

	twimlResult, err := twiml.Voice([]twiml.Element{say, connect})
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	} else {
		h.logger.Info(c.Request.Context(), fmt.Sprintf("TwiML Response: %s", twimlResult))
		c.Header("Content-Type", "text/xml")
		c.String(http.StatusOK, twimlResult)
	}
}

// TwilioMediaEvent moved to internal/voice/twilio package

func (h *Handler) HandleVoice(c *gin.Context) {
	ctx := c.Request.Context()

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err, c.Request.Header, c.Request)
		return
	}
	defer conn.Close()
	h.logger.Info(ctx, "Twilio WebSocket connection established")

	audioChan, stopChan := h.voiceProcessor.StartTwilioTranscribeAgent(ctx)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Connection closed:", err)
			break
		}

		var event struct {
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
		if err := json.Unmarshal(msg, &event); err != nil {
			log.Println("❌ JSON parse error:", err)
			continue
		}

		switch event.Event {
		case "start":
			log.Printf("🚀 Stream started: SID = %s", event.Start.StreamSid)
		case "media":
			audioBytes, err := base64.StdEncoding.DecodeString(event.Media.Payload)
			if err != nil {
				log.Println("❌ Failed to decode audio:", err)
				continue
			}
			//log.Printf("🎧 Received %d audio bytes", len(audioBytes))
			select {
			case audioChan <- audioBytes:
				h.logger.Debug(ctx, fmt.Sprintf("Sent %d bytes to audioChan (len now %d)", len(audioBytes),
					len(audioChan)))
			default:
				h.logger.Info(ctx, "⚠️ Audio channel full, dropping chunk")
			}
		case "stop":
			log.Printf("🛑 Stream stopped: SID = %s", event.Stop.StreamSid)
			close(audioChan)
			<-stopChan // Wait for transcription goroutine to finish
			return
		default:
			log.Printf("⚠️ Unknown event type: %s", event.Event)
		}
	}
}
