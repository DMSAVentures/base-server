package googleai

import (
	"base-server/internal/voice/audio"
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

// VoiceAgentResult represents a voice agent response
type VoiceAgentResult struct {
	AudioData []byte // PCM audio data to send back
	Text      string // Text transcription of the response
	Error     error
}

// StartVoiceAgent starts a two-way voice conversation with Google AI
func (g *GoogleAILiveClient) StartVoiceAgent(ctx context.Context, audioStream <-chan []byte) <-chan VoiceAgentResult {
	results := make(chan VoiceAgentResult, 100)

	// Create a cancellable context for coordinated shutdown
	voiceCtx, cancel := context.WithCancel(ctx)

	go func() {
		defer close(results)
		defer cancel() // Cancel context when main goroutine exits

		// Connect to the Live API for voice conversation
		// Use AUDIO response modality for voice responses
		config := &genai.LiveConnectConfig{
			ResponseModalities: []genai.Modality{genai.Modality("AUDIO")},
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{
					{Text: "You are a helpful and friendly voice assistant on a phone call. Keep your responses concise and conversational. Be warm and engaging. Respond naturally to what you hear."},
				},
			},
			// Enable transcription of both input and output
			InputAudioTranscription:  &genai.AudioTranscriptionConfig{},
			OutputAudioTranscription: &genai.AudioTranscriptionConfig{},
			// Configure voice
			SpeechConfig: &genai.SpeechConfig{
				VoiceConfig: &genai.VoiceConfig{
					PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{
						VoiceName: "Aoede", // Use a friendly voice
					},
				},
			},
			// Enable automatic voice activity detection
			RealtimeInputConfig: &genai.RealtimeInputConfig{
				AutomaticActivityDetection: &genai.AutomaticActivityDetection{
					Disabled: false, // Enable VAD
				},
			},
		}

		// Use native audio dialog model for natural conversation
		modelName := "gemini-2.5-flash-preview-native-audio-dialog"
		session, err := g.client.Live.Connect(voiceCtx, modelName, config)
		if err != nil {
			g.logger.Error(ctx, "Failed to connect to Google AI Live API", err)
			results <- VoiceAgentResult{Error: fmt.Errorf("failed to connect: %w", err)}
			return
		}
		// Defer session close but also close it explicitly when we're done
		defer session.Close()

		g.logger.Info(ctx, "Connected to Google AI Live API for voice agent")

		// Goroutine to receive responses from the model
		go func() {
			for {
				select {
				case <-voiceCtx.Done():
					// Context cancelled, stop receiving
					return
				default:
					msg, err := session.Receive()
					if err != nil {
						// Check if context was cancelled or if it's a connection closed error
						if voiceCtx.Err() != nil {
							g.logger.Info(ctx, "Receive goroutine exiting due to context cancellation")
							return // Clean shutdown
						}
						// Check if it's a connection closed error (expected when we close the session)
						if strings.Contains(err.Error(), "use of closed network connection") || 
						   strings.Contains(err.Error(), "closed") {
							g.logger.Info(ctx, "Google AI session closed, stopping receive goroutine")
							return // Expected closure
						}
						// This is an unexpected error
						g.logger.Error(ctx, "Unexpected error receiving message", err)
						// Try to send error, but don't panic if channel is closed
						select {
						case results <- VoiceAgentResult{Error: err}:
						case <-voiceCtx.Done():
						}
						return
					}

				// Log all messages to understand what we're getting
				if msg.ServerContent != nil {
					if msg.ServerContent.Interrupted {
						g.logger.Info(ctx, "Model was interrupted")
					}
					if msg.ServerContent.InputTranscription != nil && msg.ServerContent.InputTranscription.Text != "" {
						g.logger.Info(ctx, fmt.Sprintf("📢 User is saying: %s", msg.ServerContent.InputTranscription.Text))
					}
				}

				// Handle audio responses from model turn
				if msg.ServerContent != nil && msg.ServerContent.ModelTurn != nil {
					for _, part := range msg.ServerContent.ModelTurn.Parts {
						// Check if part has inline data (audio)
						if part.InlineData != nil && part.InlineData.Data != nil {
							g.logger.Info(ctx, fmt.Sprintf("Received %d bytes of audio from model", len(part.InlineData.Data)))
							// Try to send audio, but don't panic if channel is closed
							select {
							case results <- VoiceAgentResult{
								AudioData: part.InlineData.Data, // This is 24kHz PCM audio
							}:
							case <-voiceCtx.Done():
								return
							}
						}
					}
				}

				// Handle transcriptions of the model's audio output
				if msg.ServerContent != nil && msg.ServerContent.OutputTranscription != nil {
					transcript := msg.ServerContent.OutputTranscription.Text
					if transcript != "" {
						g.logger.Info(ctx, fmt.Sprintf("Model said: %s", transcript))
						// Try to send transcript, but don't panic if channel is closed
						select {
						case results <- VoiceAgentResult{
							Text: transcript,
						}:
						case <-voiceCtx.Done():
							return
						}
					}
				}

				// Log input transcriptions for debugging
				if msg.ServerContent != nil && msg.ServerContent.InputTranscription != nil {
					if msg.ServerContent.InputTranscription.Text != "" {
						g.logger.Info(ctx, fmt.Sprintf("User said: %s", msg.ServerContent.InputTranscription.Text))
					}
				}

				// Handle turn completion
				if msg.ServerContent != nil && msg.ServerContent.TurnComplete {
					g.logger.Info(ctx, "Model turn completed")
				}
				}
			}
		}()

		// No text input goroutine - pure audio conversation to avoid concurrent writes

		// Main goroutine to send audio chunks
		audioChunkCount := 0
		for {
			select {
			case <-voiceCtx.Done():
				g.logger.Info(ctx, "Context cancelled, stopping voice agent")
				return
			case audioChunk, ok := <-audioStream:
				if !ok {
					g.logger.Info(ctx, fmt.Sprintf("Audio stream closed after %d chunks", audioChunkCount))
					// Gracefully close the session before returning
					session.Close()
					return
				}

				// Convert mulaw 8kHz to PCM 16kHz
				pcmAudio := audio.ConvertMuLawToPCM16kHz(audioChunk)
				audioChunkCount++

				// Log progress
				if audioChunkCount%1000 == 0 {
					g.logger.Info(ctx, fmt.Sprintf("Processed %d audio chunks", audioChunkCount))
				}

				// Send audio to the model
				err = session.SendRealtimeInput(genai.LiveRealtimeInput{
					Audio: &genai.Blob{
						Data:     pcmAudio,
						MIMEType: "audio/pcm;rate=16000",
					},
				})
				if err != nil {
					g.logger.Error(ctx, "Failed to send audio chunk", err)
					// Try to send error, but don't panic if channel is closed
					select {
					case results <- VoiceAgentResult{Error: fmt.Errorf("failed to send audio: %w", err)}:
					case <-voiceCtx.Done():
					}
					return
				}
				
				// Log every 100 chunks to see if we're actually sending
				if audioChunkCount%100 == 0 {
					g.logger.Debug(ctx, fmt.Sprintf("Sent chunk %d to Google AI, size: %d bytes", audioChunkCount, len(pcmAudio)))
				}
			}
		}
	}()

	return results
}
