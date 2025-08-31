package googleai

import (
	"base-server/internal/observability"
	"base-server/internal/voice/audio"
	"context"
	"fmt"

	"google.golang.org/genai"
)

// GoogleAILiveClient handles real-time audio streaming and transcription using Google AI
type GoogleAILiveClient struct {
	client *genai.Client
	logger *observability.Logger
}

// NewGoogleAILiveClient creates a new Google AI client for real-time audio streaming
func NewGoogleAILiveClient(apiKey string, logger *observability.Logger) (*GoogleAILiveClient, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Google AI client: %w", err)
	}

	return &GoogleAILiveClient{
		client: client,
		logger: logger,
	}, nil
}

// TranscriptionResult represents a transcription result from the Live API
type TranscriptionResult struct {
	Text    string
	IsFinal bool
	Error   error
}

// StartRealtimeTranscription starts a real-time transcription session with Google AI
func (g *GoogleAILiveClient) StartRealtimeTranscription(ctx context.Context, audioStream <-chan []byte) <-chan TranscriptionResult {
	results := make(chan TranscriptionResult, 100)

	go func() {
		defer close(results)

		// Connect to the Live API for audio transcription
		// Based on official documentation
		config := &genai.LiveConnectConfig{
			// For transcription, we want TEXT responses
			ResponseModalities: []genai.Modality{genai.Modality("AUDIO")},
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{
					{Text: "You are a helpful assistant that transcribes audio input into text and does nothing else."},
				},
			},
			// Enable input audio transcription to get transcripts
			OutputAudioTranscription: &genai.AudioTranscriptionConfig{},
		}

		// Use the standard Live API model
		modelName := "gemini-2.5-flash-preview-native-audio-dialog"
		session, err := g.client.Live.Connect(ctx, modelName, config)
		if err != nil {
			g.logger.Error(ctx, "Failed to connect to Google AI Live API", err)
			results <- TranscriptionResult{Error: fmt.Errorf("failed to connect: %w", err)}
			return
		}
		defer session.Close()

		g.logger.Info(ctx, "Connected to Google AI Live API for real-time transcription")

		// Start goroutine to receive server messages
		go func() {
			for {
				msg, err := session.Receive()
				if err != nil {
					g.logger.Error(ctx, "Error receiving message", err)
					results <- TranscriptionResult{Error: err}
					return
				}

				// Handle different message types
				if msg.ServerContent != nil {
					// Check for model turn content
					if msg.ServerContent.ModelTurn != nil {
						for _, part := range msg.ServerContent.ModelTurn.Parts {
							if part.Text != "" {
								g.logger.Info(ctx, fmt.Sprintf("Received model response: %s", part.Text))
								results <- TranscriptionResult{
									Text:    part.Text,
									IsFinal: msg.ServerContent.TurnComplete,
								}
							}
						}
					}

					// Check for input transcription
					if msg.ServerContent.OutputTranscription != nil && msg.ServerContent.OutputTranscription.Text != "" {
						g.logger.Info(ctx, fmt.Sprintf("Received input transcription: %s", msg.ServerContent.OutputTranscription.Text))
						results <- TranscriptionResult{
							Text:    msg.ServerContent.OutputTranscription.Text,
							IsFinal: msg.ServerContent.OutputTranscription.Finished,
						}
					}
				}

				// Check for turn completion
				if msg.ServerContent != nil && msg.ServerContent.TurnComplete {
					g.logger.Info(ctx, "Turn completed")
				}
			}
		}()

		// Don't send any initial text - the native audio model expects only audio
		// Send audio chunks to the Live API
		audioChunkCount := 0
		for {
			select {
			case <-ctx.Done():
				g.logger.Info(ctx, "Context cancelled, stopping transcription")
				return
			case audioChunk, ok := <-audioStream:
				if !ok {
					g.logger.Info(ctx, fmt.Sprintf("Audio stream closed after %d chunks", audioChunkCount))
					return
				}

				// Convert mulaw 8kHz to PCM 16kHz (matching Python reference)
				pcmAudio := audio.ConvertMuLawToPCM16kHz(audioChunk)
				audioChunkCount++

				// Log every 1000th chunk to avoid spam
				if audioChunkCount%1000 == 0 {
					g.logger.Info(ctx, fmt.Sprintf("Processed %d audio chunks, latest chunk: %d bytes mulaw -> %d bytes PCM 16kHz",
						audioChunkCount, len(audioChunk), len(pcmAudio)))
				}

				// Send audio chunk with proper format
				// Documentation shows: audio/pcm;rate=16000
				err = session.SendRealtimeInput(genai.LiveRealtimeInput{
					Audio: &genai.Blob{
						Data:     pcmAudio,
						MIMEType: "audio/pcm;rate=16000",
					},
				})
				if err != nil {
					g.logger.Error(ctx, "Failed to send audio chunk", err)
					results <- TranscriptionResult{Error: fmt.Errorf("failed to send audio: %w", err)}
					return
				}
			}
		}
	}()

	return results
}

// ConnectToLiveAPI establishes a connection to the Google AI Live API
func (g *GoogleAILiveClient) ConnectToLiveAPI(ctx context.Context) error {
	config := &genai.LiveConnectConfig{
		ResponseModalities:      []genai.Modality{genai.Modality("AUDIO"), genai.Modality("TEXT")},
		InputAudioTranscription: &genai.AudioTranscriptionConfig{},
	}

	session, err := g.client.Live.Connect(ctx, "models/gemini-2.5-flash-live-preview", config)
	if err != nil {
		return fmt.Errorf("failed to connect to Live API: %w", err)
	}
	defer session.Close()

	g.logger.Info(ctx, "Successfully connected to Google AI Live API")

	// Keep connection alive for testing
	<-ctx.Done()
	return nil
}
