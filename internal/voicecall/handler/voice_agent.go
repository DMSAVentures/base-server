package handler

import (
	"base-server/internal/apierrors"
	"base-server/internal/voice/pipeline"
	"base-server/internal/voicecall/twilio"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/twilio/twilio-go/twiml"
)

func (h *Handler) HandleVoiceAgent(c *gin.Context) {
	ctx := c.Request.Context()

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error(ctx, "WebSocket upgrade failed", err)
		return
	}
	defer conn.Close()

	h.logger.Info(ctx, "Twilio WebSocket connection established for voice agent")

	// Create Twilio WebSocket handler
	twilioHandler := twilio.NewWebSocketHandler(conn, h.logger)
	defer twilioHandler.Stop()

	// Channels for audio flow
	twilioIn := make(chan []byte, 4096)  // Audio from Twilio
	twilioOut := make(chan []byte, 4096) // Audio to Twilio

	// Create pipeline with fixed source (Twilio)
	pipelineConfig := pipeline.DefaultConfig()
	audioPipeline, err := pipeline.NewAudioPipeline(twilioIn, twilioOut, h.logger, pipelineConfig)
	if err != nil {
		h.logger.Error(ctx, "Failed to create pipeline", err)
		return
	}
	defer audioPipeline.Stop()

	// Get AI provider channels (sink)
	aiIn, aiOut := h.voiceProcessor.StartTwilioVoiceAgent(ctx)
	defer close(aiIn)

	// Connect AI provider as sink
	err = audioPipeline.ConnectSink(aiIn, aiOut)
	if err != nil {
		h.logger.Error(ctx, "Failed to connect AI sink", err)
		return
	}

	// Start pipeline
	err = audioPipeline.Start(ctx)
	if err != nil {
		h.logger.Error(ctx, "Failed to start pipeline", err)
		return
	}

	// Start Twilio WebSocket handler
	err = twilioHandler.Start(ctx, twilioIn, twilioOut)
	if err != nil {
		h.logger.Error(ctx, "Failed to start Twilio handler", err)
		return
	}

	// Wait for context cancellation
	<-ctx.Done()
	h.logger.Info(ctx, "Voice agent session ended")
}

func (h *Handler) HandleAnswerVoiceAgent(c *gin.Context) {
	// The WebSocket server URL for voice agent
	wsURL := "wss://cd5f83cca865.ngrok-free.app/api/phone/voice-agent"

	h.logger.Info(c.Request.Context(), fmt.Sprintf("Voice Agent TwiML WebSocket URL: %s", wsURL))

	// Initial greeting while connecting
	say := &twiml.VoiceSay{
		Message: "Hello! Connecting you to our assistant. One moment please.",
	}

	// Stream configuration for bi-directional audio
	stream := twiml.VoiceStream{
		Name: "voice-agent-stream",
		Url:  wsURL,
	}

	connect := twiml.VoiceConnect{
		InnerElements: []twiml.Element{stream},
	}

	twimlResult, err := twiml.Voice([]twiml.Element{say, connect})
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	} else {
		h.logger.Info(c.Request.Context(), fmt.Sprintf("Voice Agent TwiML Response: %s", twimlResult))
		c.Header("Content-Type", "text/xml")
		c.String(200, twimlResult)
	}
}
