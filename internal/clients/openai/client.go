package openai

import (
	"base-server/internal/observability"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

const baseRestURL = "https://api.openai.com/v1"
const openAIRealtimeURL = "wss://api.openai.com/v1/realtime"

// RealtimeTranscriptionConfig holds configuration for the session.
type RealtimeTranscriptionConfig struct {
	Model          string // e.g. "gpt-4o-transcribe", "whisper-1"
	Language       string // ISO-639-1 code, e.g. "en"
	Prompt         string
	NoiseReduction string // "near_field", "far_field", or ""
	VAD            bool   // Enable server VAD
}

// TranscriptionResult represents a partial or final transcription from OpenAI.
type TranscriptionCompletedResult struct {
	EventID      string `json:"event_id"`
	Type         string `json:"type"`
	ContentIndex int    `json:"content_index"`
	Transcript   string `json:"transcript"`
	ItemID       string `json:"item_id"`
}

type TranscriptionDeltaResult struct {
	EventID      string `json:"event_id"`
	Type         string `json:"type"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
	ItemID       string `json:"item_id"`
}

type TranscriptionModelConfig struct {
	Model    string `json:"model"`
	Language string `json:"language"`
	Prompt   string `json:"prompt"`
}

type TurnDetectionConfig struct {
	Type              string  `json:"type"`
	Threshold         float64 `json:"threshold"`
	PrefixPaddingMS   int     `json:"prefix_padding_ms"`
	SilenceDurationMS int     `json:"silence_duration_ms"`
}
type TranscriptionSession struct {
	ID                      string                   `json:"id"`
	Object                  string                   `json:"object"`
	Modalities              []string                 `json:"modalities"`
	InputAudioFormat        string                   `json:"input_audio_format"`
	TurnDetection           TurnDetectionConfig      `json:"turn_detection"`
	InputAudioTranscription TranscriptionModelConfig `json:"input_audio_transcription"`
	ClientSecret            string                   `json:"client_secret"`
}

type TranscriptionSessionRequest struct {
	Modalities               []string                 `json:"modalities"`
	InputAudioFormat         string                   `json:"input_audio_format"`
	InputAudioNoiseReduction map[string]string        `json:"input_audio_noise_reduction"`
	InputAudioTranscription  TranscriptionModelConfig `json:"input_audio_transcription"`
	TurnDetection            TurnDetectionConfig      `json:"turn_detection"`
}

type Session struct {
	Type    string                      `json:"type"`
	Session TranscriptionSessionRequest `json:"session"`
}

type RealtimeTranscriptionSessionConfig struct {
	InputAudioFormat         string                   `json:"input_audio_format"`
	InputAudioNoiseReduction map[string]string        `json:"input_audio_noise_reduction"`
	InputAudioTranscription  TranscriptionModelConfig `json:"input_audio_transcription"`
	Instructions             string                   `json:"instructions"`
	MaxResponseOutputTokens  int                      `json:"max_response_output_tokens"`
	Modalities               []string                 `json:"modalities"`
	OutputAudioFormat        string                   `json:"output_audio_format"`
	Temperature              int                      `json:"temperature"`
	ToolChoice               string                   `json:"tool_choice"`
	Tools                    []string                 `json:"tools"`
	TurnDetection            TurnDetectionConfig      `json:"turn_detection"`
}

type TranscriptionChannelResult struct {
	Result TranscriptionCompletedResult
	Err    error
}

type OpenAIWebsocketClient struct {
	apiKey string
	logger *observability.Logger
	client http.Client
}

func NewOpenAIRealtimeClient(apiKey string, logger *observability.Logger) (*OpenAIWebsocketClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	return &OpenAIWebsocketClient{apiKey: apiKey, logger: logger, client: client}, nil
}

// StartRealtimeTranscription opens a websocket, creates a session, streams audio, and returns a channel of transcription results.
func (c *OpenAIWebsocketClient) StartRealtimeTranscription(ctx context.Context, audioStream <-chan []byte) <-chan TranscriptionChannelResult {
	results := make(chan TranscriptionChannelResult, 100)
	go func() {
		defer close(results)
		dialer := websocket.Dialer{}
		headers := http.Header{}
		headers.Set("Authorization", "Bearer "+c.apiKey)
		headers.Set("OpenAI-Beta", "realtime=v1")

		openAIURl, err := url.Parse(openAIRealtimeURL)
		if err != nil {
			if c.logger != nil {
				c.logger.Error(ctx, "Failed to parse OpenAI realtime URL", err)
			}
			results <- TranscriptionChannelResult{Err: err}
			return
		}

		query := openAIURl.Query()
		query.Set("intent", "transcription")
		openAIURl.RawQuery = query.Encode()
		openURL := openAIURl.String()
		c.logger.Info(ctx, "Connecting to OpenAI Realtime URL: "+openURL)
		conn, resp, err := dialer.DialContext(ctx, openURL, headers)
		if err != nil {
			if c.logger != nil {
				c.logger.Error(ctx, "Failed to connect to OpenAI realtime endpoint", err)
			}
			results <- TranscriptionChannelResult{Err: err}
			return
		}
		b, _ := io.ReadAll(resp.Body)
		c.logger.Info(ctx, fmt.Sprintf("Connected to OpenAI Realtime endpoint: %s", string(b)))

		defer conn.Close()

		sess := TranscriptionSessionRequest{
			InputAudioFormat:         "pcm16",
			InputAudioNoiseReduction: map[string]string{"type": "near_field"},
			InputAudioTranscription: TranscriptionModelConfig{
				Language: "en",
				Model:    "gpt-4o-transcribe",
				Prompt:   "Please transcribe this audio.",
			},
			TurnDetection: TurnDetectionConfig{
				Type: "server_vad",
			},
		}

		sessUpdate := Session{
			Type:    "session.update",
			Session: sess,
		}

		if err := conn.WriteJSON(sessUpdate); err != nil {
			if c.logger != nil {
				c.logger.Error(ctx, "Failed to send session creation message", err)
			}
			results <- TranscriptionChannelResult{Err: err}
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
				typeStr := event["type"].(string)
				switch typeStr {
				case "conversation.item.input_audio_transcription.delta":
					var res TranscriptionDeltaResult
					if err := json.Unmarshal(msg, &res); err != nil {
						c.logger.Error(ctx, "Failed to unmarshal delta event", err)
						continue
					}
					c.logger.Info(ctx, fmt.Sprintf("Received delta event %s", res.Delta))
				case "conversation.item.input_audio_transcription.completed":
					var res TranscriptionCompletedResult
					if err := json.Unmarshal(msg, &res); err != nil {
						c.logger.Error(ctx, "Failed to unmarshal completed event", err)
						continue
					}
					results <- TranscriptionChannelResult{Result: res}
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
					c.logger.Info(ctx, "Audio stream closed")
					return
				}
				appendEvent := map[string]interface{}{
					"type":  "input_audio_buffer.append",
					"audio": base64.StdEncoding.EncodeToString(chunk),
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
	return results
}

// SynthesizeSpeech uses OpenAI's TTS API to synthesize speech from text.
func (c *OpenAIWebsocketClient) SynthesizeSpeech(ctx context.Context, text string, voice string) ([]byte, error) {
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

func (c *OpenAIWebsocketClient) CreateTranscriptionSession(ctx context.Context) (TranscriptionSession, error) {
	req := &http.Request{
		Method: http.MethodPost,
		URL:    &url.URL{Host: baseRestURL, Path: "/v1/audio/transcriptions"},
		Header: http.Header{
			"Authorization": []string{"Bearer " + c.apiKey},
			"Content-Type":  []string{"application/json"},
		},
	}
	body := TranscriptionSessionRequest{
		InputAudioFormat:         "g711_ulaw",
		InputAudioNoiseReduction: map[string]string{"type": "near_field"},
		InputAudioTranscription: TranscriptionModelConfig{
			Language: "en",
			Model:    "gpt-4o-transcribe",
			Prompt:   "Please transcribe this audio.",
		},
		TurnDetection: TurnDetectionConfig{
			Type: "server_vad",
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		c.logger.Error(ctx, "Failed to marshal request body", err)
		return TranscriptionSession{}, fmt.Errorf("failed to marshal request body: %w", err)
	}
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	req.ContentLength = int64(len(bodyBytes))
	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error(ctx, "Failed to create session", err)
		return TranscriptionSession{}, fmt.Errorf("failed to create session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		c.logger.Error(ctx, "Failed to create session", fmt.Errorf("status code: %d, body: %s", resp.StatusCode, string(bodyBytes)))
		return TranscriptionSession{}, fmt.Errorf("failed to create session: status code %d", resp.StatusCode)
	}

	var session TranscriptionSession
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		c.logger.Error(ctx, "Failed to decode session response", err)
		return TranscriptionSession{}, fmt.Errorf("failed to decode session response: %w", err)
	}

	return session, nil
}
