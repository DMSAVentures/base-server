package openai

import (
	"base-server/internal/observability"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const openAIRealtimeURL = "wss://api.openai.com/v1/audio/transcriptions/stream"

// RealtimeTranscriptionConfig holds configuration for the session.
type RealtimeTranscriptionConfig struct {
	Model          string // e.g. "gpt-4o-transcribe", "whisper-1"
	Language       string // ISO-639-1 code, e.g. "en"
	Prompt         string
	NoiseReduction string // "near_field", "far_field", or ""
	VAD            bool   // Enable server VAD
}

// TranscriptionResult represents a partial or final transcription from OpenAI.
type TranscriptionResult struct {
	Type       string // "delta" or "completed"
	Delta      string // for delta events
	Transcript string // for completed events
	ItemID     string
}

type OpenAIRealtimeClient struct {
	apiKey string
	logger *observability.Logger
}

func NewOpenAIRealtimeClient(apiKey string, logger *observability.Logger) (*OpenAIRealtimeClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}
	return &OpenAIRealtimeClient{apiKey: apiKey, logger: logger}, nil
}

// StartRealtimeTranscription opens a websocket, creates a session, streams audio, and returns a channel of transcription results.
func (c *OpenAIRealtimeClient) StartRealtimeTranscription(ctx context.Context, audioStream <-chan []byte, cfg RealtimeTranscriptionConfig) (<-chan TranscriptionResult, error) {
	results := make(chan TranscriptionResult)
	go func() {
		defer close(results)
		dialer := websocket.Dialer{}
		headers := http.Header{}
		headers.Set("Authorization", "Bearer "+c.apiKey)

		conn, _, err := dialer.DialContext(ctx, openAIRealtimeURL, headers)
		if err != nil {
			if c.logger != nil {
				c.logger.Error(ctx, "Failed to connect to OpenAI realtime endpoint", err)
			}
			results <- TranscriptionResult{Type: "error", Delta: err.Error()}
			return
		}
		defer conn.Close()

		// 1. Send session creation message
		sessionReq := map[string]interface{}{
			"object":             "realtime.transcription_session",
			"input_audio_format": "pcm16",
			"input_audio_transcription": []map[string]string{
				{
					"model":    cfg.Model,
					"prompt":   cfg.Prompt,
					"language": cfg.Language,
				},
			},
		}
		if cfg.NoiseReduction != "" {
			sessionReq["input_audio_noise_reduction"] = map[string]string{"type": cfg.NoiseReduction}
		}
		if cfg.VAD {
			sessionReq["turn_detection"] = map[string]interface{}{
				"type":                "server_vad",
				"threshold":           0.5,
				"prefix_padding_ms":   300,
				"silence_duration_ms": 500,
			}
		} else {
			sessionReq["turn_detection"] = nil
		}
		if err := conn.WriteJSON(sessionReq); err != nil {
			if c.logger != nil {
				c.logger.Error(ctx, "Failed to send session creation message", err)
			}
			results <- TranscriptionResult{Type: "error", Delta: err.Error()}
			return
		}

		// 2. Start goroutine to read events
		go func() {
			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				var event map[string]interface{}
				if err := json.Unmarshal(msg, &event); err != nil {
					continue
				}
				typeStr, _ := event["type"].(string)
				itemID, _ := event["item_id"].(string)
				switch typeStr {
				case "conversation.item.input_audio_transcription.delta":
					delta, _ := event["delta"].(string)
					results <- TranscriptionResult{Type: "delta", Delta: delta, ItemID: itemID}
				case "conversation.item.input_audio_transcription.completed":
					transcript, _ := event["transcript"].(string)
					results <- TranscriptionResult{Type: "completed", Transcript: transcript, ItemID: itemID}
				}
			}
		}()

		// 3. Send audio chunks as input_audio_buffer.append events
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-audioStream:
				if !ok {
					return
				}
				appendEvent := map[string]interface{}{
					"type": "input_audio_buffer.append",
					"data": chunk,
				}
				if err := conn.WriteJSON(appendEvent); err != nil {
					if c.logger != nil {
						c.logger.Error(ctx, "Failed to send audio chunk", err)
					}
					return
				}
				// Optional: throttle to match real-time or API rate limits
				time.Sleep(40 * time.Millisecond)
			}
		}
	}()
	return results, nil
}

// SynthesizeSpeech uses OpenAI's TTS API to synthesize speech from text.
func (c *OpenAIRealtimeClient) SynthesizeSpeech(ctx context.Context, text string, voice string) ([]byte, error) {
	url := "https://api.openai.com/v1/audio/speech"
	jsonBody := map[string]interface{}{
		"model":           "tts-1", // or "tts-1-hd"
		"voice":           voice,   // e.g., "alloy", "echo", "fable", "onyx", "nova", "shimmer"
		"input":           text,
		"response_format": "mp3",
	}
	bodyBytes, err := json.Marshal(jsonBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal TTS request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create TTS request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI TTS error: %s", string(respBody))
	}

	return io.ReadAll(resp.Body)
}
