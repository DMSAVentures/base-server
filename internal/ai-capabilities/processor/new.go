package processor

import (
	"base-server/internal/clients/googleai"
	openAIHTTP "base-server/internal/clients/openai"
	"base-server/internal/observability"
	"base-server/internal/store"
	"base-server/internal/voice/audio"
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/genai"
)

type AIProcessor struct {
	logger         *observability.Logger
	geminiApiKey   string
	openAiApiKey   string
	store          store.Storer
	openaiRealtime *openAIHTTP.OpenAIWebsocketClient
	googleLive     *googleai.GoogleAILiveClient
}

func New(logger *observability.Logger, geminiApiKey string, openAiApiKey string, store store.Storer) *AIProcessor {
	var openaiRealtime *openAIHTTP.OpenAIWebsocketClient
	if openAiApiKey != "" {
		client, err := openAIHTTP.NewOpenAIRealtimeClient(openAiApiKey, logger)
		if err == nil {
			openaiRealtime = client
		}
	}

	// Initialize Google AI Live client
	var googleLive *googleai.GoogleAILiveClient
	if geminiApiKey != "" {
		client, err := googleai.NewGoogleAILiveClient(geminiApiKey, logger)
		if err == nil {
			googleLive = client
		} else {
			logger.Error(context.Background(), "Failed to create Google AI Live client", err)
		}
	}

	return &AIProcessor{
		logger:         logger,
		geminiApiKey:   geminiApiKey,
		openAiApiKey:   openAiApiKey,
		store:          store,
		openaiRealtime: openaiRealtime,
		googleLive:     googleLive,
	}
}

type StreamResponse struct {
	Content   string
	Error     error
	Completed bool
}

type ModelResponse struct {
	TotalTokens int32
	Message     string
}

type TextAIChatProcessor interface {
	CreateConversation(ctx context.Context, userID uuid.UUID, msg string) (<-chan StreamResponse, error)
	ContinueConversation(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID,
		msg string) (<-chan StreamResponse,
		error)
	ChatTextAI(ctx context.Context, conversationID uuid.UUID,
		messages []store.Message) (<-chan StreamResponse,
		<-chan ModelResponse)
}

func (a *AIProcessor) GenerateTitle(ctx context.Context, userMsg string, assistantMsg string) (string, error) {
	prompt := fmt.Sprintf(`
Given the following conversation, generate a short descriptive title (max 6 words). Avoid quotes.

User: %s
Assistant: %s

Title:`, userMsg, assistantMsg)

	c, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: a.geminiApiKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Use genai.Text helper to create content
	contents := genai.Text(prompt)
	resp, err := c.Models.GenerateContent(ctx, "gemini-2.5-pro-preview-03-25", contents, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate title: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no title returned from Gemini")
	}

	// Extract text from the part
	if resp.Candidates[0].Content.Parts[0].Text != "" {
		return strings.TrimSpace(resp.Candidates[0].Content.Parts[0].Text), nil
	}
	return "", fmt.Errorf("unexpected response format")
}

func (a *AIProcessor) TranscribeAudio(ctx context.Context) (chan []byte, chan struct{}) {
	// Channel for streaming audio to the AI processor
	audioChan := make(chan []byte, 4096)
	// Channel for signaling when to stop transcription
	stopChan := make(chan struct{})

	// Use Google AI Live for transcription instead of OpenAI
	if a.googleLive != nil {
		go func() {
			results := a.googleLive.StartRealtimeTranscription(ctx, audioChan)

			for res := range results {
				if res.Error != nil {
					a.logger.Error(ctx, "❌ Real-time transcription error:", res.Error)
					return
				}

				if res.Text != "" {
					if res.IsFinal {
						a.logger.Info(ctx, fmt.Sprintf("📝 Final transcript: %s", res.Text))
					} else {
						a.logger.Info(ctx, fmt.Sprintf("📝 Partial transcript: %s", res.Text))
					}
				}
			}
			close(stopChan)
		}()
	} else if a.openaiRealtime != nil {
		// Fallback to OpenAI if Google AI is not available
		go func() {
			results := a.openaiRealtime.StartRealtimeTranscription(ctx, audioChan)

			for res := range results {
				if res.Err != nil {
					a.logger.Error(ctx, "❌ Real-time transcription error:", res.Err)
					return
				}

				a.logger.Info(ctx, fmt.Sprintf("📝 Whisper transcript: %s", res.Result.Transcript))
			}
			close(stopChan)
		}()
	} else {
		a.logger.Error(ctx, "No transcription service available", nil)
		close(stopChan)
	}

	return audioChan, stopChan
}

func (a *AIProcessor) StartVoiceAgent(ctx context.Context) (chan []byte, chan []byte) {
	// Channel for streaming audio from Twilio to AI
	audioInChan := make(chan []byte, 4096)
	// Channel for streaming audio from AI to Twilio
	audioOutChan := make(chan []byte, 4096)

	if a.googleLive != nil {
		go func() {
			results := a.googleLive.StartVoiceAgent(ctx, audioInChan)

			for res := range results {
				if res.Error != nil {
					// Only log unexpected errors, not connection closed errors
					errStr := res.Error.Error()
					if !strings.Contains(errStr, "use of closed network connection") &&
						!strings.Contains(errStr, "closed") {
						a.logger.Error(ctx, "Voice agent error", res.Error)
					}
					continue
				}

				if res.AudioData != nil {
					// Convert 24kHz PCM from Gemini to 8kHz mulaw for Twilio
					mulawAudio := audio.ConvertPCM24kHzToMuLaw8kHz(res.AudioData)
					select {
					case audioOutChan <- mulawAudio:
						a.logger.Debug(ctx, fmt.Sprintf("Sent %d bytes of mulaw audio", len(mulawAudio)))
					default:
						a.logger.Warn(ctx, "Audio output channel full, dropping audio chunk")
					}
				}

				if res.Text != "" {
					a.logger.Info(ctx, fmt.Sprintf("Agent response: %s", res.Text))
				}
			}
			close(audioOutChan)
		}()
	} else {
		a.logger.Error(ctx, "Google AI Live client not available for voice agent", nil)
		close(audioOutChan)
	}

	return audioInChan, audioOutChan
}

func (a *AIProcessor) ConnectToWS(ctx context.Context) {
	// Use Google AI Live API for WebSocket connection
	if a.googleLive != nil {
		err := a.googleLive.ConnectToLiveAPI(ctx)
		if err != nil {
			a.logger.Error(ctx, "Failed to connect to Google AI Live API", err)
		}
	} else if a.openaiRealtime != nil {
		// Fallback to OpenAI if Google AI is not available
		_ = a.openaiRealtime.StartRealtimeTranscription(ctx, nil)
	}
}
