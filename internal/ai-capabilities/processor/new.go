package processor

import (
	openai2 "base-server/internal/clients/openai"
	"base-server/internal/observability"
	"base-server/internal/store"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
	openaiOption "github.com/openai/openai-go/option"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type AIProcessor struct {
	logger         *observability.Logger
	geminiApiKey   string
	openAiApiKey   string
	store          store.Store
	openaiRealtime *openai2.OpenAIRealtimeClient
}

func New(logger *observability.Logger, geminiApiKey string, openAiApiKey string, store store.Store) *AIProcessor {
	var openaiRealtime *openai2.OpenAIRealtimeClient
	if openAiApiKey != "" {
		client, err := openai2.NewOpenAIRealtimeClient(openAiApiKey, logger)
		if err == nil {
			openaiRealtime = client
		} else if logger != nil {
			logger.Error(context.Background(), "Failed to initialize OpenAIRealtimeClient", err)
		}
	}
	return &AIProcessor{
		logger:         logger,
		geminiApiKey:   geminiApiKey,
		openAiApiKey:   openAiApiKey,
		store:          store,
		openaiRealtime: openaiRealtime,
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

func (a *AIProcessor) ChatTextAI(ctx context.Context, conversationID uuid.UUID,
	messages []store.Message) (<-chan StreamResponse,
	<-chan ModelResponse) {
	streamingResponseChan := make(chan StreamResponse)
	fullResponseChan := make(chan ModelResponse, 1)

	var history []*genai.Content
	var prompt genai.Part

	if len(messages) > 1 {
		for _, m := range messages[:len(messages)-1] {
			role := "user"
			if m.Role == "assistant" {
				role = "model" // Gemini SDK expects "model"
			}

			history = append(history, &genai.Content{
				Role:  role,
				Parts: []genai.Part{genai.Text(m.Content)},
			})
		}

		// Last message is the new user prompt
		prompt = genai.Text(messages[len(messages)-1].Content)

	} else if len(messages) == 1 {
		prompt = genai.Text(messages[0].Content)
	}

	go func() {
		defer close(streamingResponseChan)
		defer close(fullResponseChan)

		a.logger.Info(ctx, "Starting AI stream")
		c, err := genai.NewClient(ctx, option.WithAPIKey(a.geminiApiKey))
		if err != nil {
			a.logger.Error(ctx, "Failed to create client", err)
			streamingResponseChan <- StreamResponse{Error: fmt.Errorf("failed to create AI client: %w", err)}
			return
		}
		defer c.Close()

		model := c.GenerativeModel("gemini-2.5-pro-preview-03-25")
		chat := model.StartChat()
		chat.History = history
		iter := chat.SendMessageStream(ctx, prompt)

		var totalTokens int32 = 0
		var fullAssistantMessage strings.Builder

		streamingResponseChan <- StreamResponse{Content: "[Conversation_ID]: " + conversationID.String()}

		for {
			select {
			case <-ctx.Done():
				a.logger.Info(ctx, "Context cancelled, stopping stream")
				fullResponseChan <- ModelResponse{TotalTokens: totalTokens, Message: fullAssistantMessage.String()}
				return
			default:
				resp, err := iter.Next()
				if errors.Is(err, iterator.Done) {
					a.logger.Info(ctx, "Stream completed")
					fullResponseChan <- ModelResponse{TotalTokens: totalTokens, Message: fullAssistantMessage.String()}
					streamingResponseChan <- StreamResponse{Completed: true}
					return
				}
				if err != nil {
					a.logger.Error(ctx, "Failed to get next response", err)
					streamingResponseChan <- StreamResponse{Error: fmt.Errorf("failed to get AI response: %w", err)}
					return
				}

				for _, part := range resp.Candidates[0].Content.Parts {
					bs, err := json.Marshal(part)
					if err != nil {
						a.logger.Error(ctx, "Failed to marshal response part", err)
						streamingResponseChan <- StreamResponse{Error: fmt.Errorf("failed to marshal response: %w", err)}
						continue
					}

					stringPart := string(bs)
					streamingResponseChan <- StreamResponse{Content: stringPart}
					fullAssistantMessage.WriteString(stringPart)
				}

				if resp.UsageMetadata != nil {
					totalTokens = resp.UsageMetadata.CandidatesTokenCount + resp.UsageMetadata.PromptTokenCount
				}
			}
		}
	}()

	return streamingResponseChan, fullResponseChan
}

func (a *AIProcessor) GenerateTitle(ctx context.Context, userMsg string, assistantMsg string) (string, error) {
	prompt := fmt.Sprintf(`
Given the following conversation, generate a short descriptive title (max 6 words). Avoid quotes.

User: %s
Assistant: %s

Title:`, userMsg, assistantMsg)

	c, err := genai.NewClient(ctx, option.WithAPIKey(a.geminiApiKey))
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}
	defer c.Close()

	model := c.GenerativeModel("gemini-2.5-pro-preview-03-25")
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate title: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no title returned from Gemini")
	}

	// Safely cast the part to Text and return
	part, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return "", fmt.Errorf("unexpected response format")
	}

	return strings.TrimSpace(string(part)), nil
}

func (a *AIProcessor) CreateConversation(ctx context.Context, userID uuid.UUID, msg string) (<-chan StreamResponse,
	error) {
	a.logger.Info(ctx, "Creating conversation for user")
	conversation, err := a.store.CreateConversation(ctx, userID)
	if err != nil {
		a.logger.Error(ctx, "Failed to create conversation", err)
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "conversation_id", Value: conversation.ID.String()})
	a.logger.Info(ctx, "Conversation created successfully")

	_, err = a.store.CreateMessage(ctx, conversation.ID, "user", msg)
	if err != nil {
		a.logger.Error(ctx, "Failed to create message", err)
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	respChannel, modelResponseChannel := a.ChatTextAI(ctx, conversation.ID, []store.Message{
		{
			Role:    "user",
			Content: msg,
		},
	})

	go func() {
		for resp := range modelResponseChannel {
			_, err := a.store.CreateMessage(ctx, conversation.ID, "assistant", resp.Message)
			if err != nil {
				a.logger.Error(ctx, "Failed to save assistant message", err)
				return
			}

			title, err := a.GenerateTitle(ctx, msg, resp.Message)
			if err == nil {
				err = a.store.UpdateConversationTitleByConversationID(ctx, conversation.ID, title)
				if err != nil {
					a.logger.Error(ctx, "Failed to update conversation title", err)
				}
			} else {
				a.logger.Error(ctx, "Failed to generate title", err)
			}

			usageLog := store.UsageLog{
				UserID:         userID,
				ConversationID: conversation.ID,
				TokensUsed:     resp.TotalTokens,
				CostInCents:    0,
				Model:          "gemini-2.5-pro-preview-03-25",
			}
			_, err = a.store.InsertUsageLog(ctx, usageLog)
			if err != nil {
				a.logger.Error(ctx, "Failed to insert usage log", err)
			}
		}
	}()

	return respChannel, nil
}

func (a *AIProcessor) ContinueConversation(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID,
	msg string) (<-chan StreamResponse,
	error) {
	a.logger.Info(ctx, "Continuing conversation for user")
	msgs, err := a.store.GetAllMessagesByConversationID(ctx, conversationID)
	if err != nil {
		a.logger.Error(ctx, "Failed to get messages", err)
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	if len(msgs) == 0 {
		a.logger.Error(ctx, "No messages found for conversation", nil)
		return nil, fmt.Errorf("no messages found for conversation")
	}

	msgs = append(msgs, store.Message{
		Role:    "user",
		Content: msg,
	})

	respChannel, modelResponseChannel := a.ChatTextAI(ctx, conversationID, msgs)

	go func() {
		for resp := range modelResponseChannel {
			_, err = a.store.CreateMessage(ctx, conversationID, "user", msg)
			if err != nil {
				a.logger.Error(ctx, "Failed to create message", err)
				//return nil, fmt.Errorf("failed to create message: %w", err)
			}

			_, err := a.store.CreateMessage(ctx, conversationID, "assistant", resp.Message)
			if err != nil {
				a.logger.Error(ctx, "Failed to save assistant message", err)
				return
			}

			usageLog := store.UsageLog{
				UserID:         userID,
				ConversationID: conversationID,
				TokensUsed:     resp.TotalTokens,
				CostInCents:    0,
				Model:          "gemini-2.5-pro-preview-03-25",
			}
			_, err = a.store.InsertUsageLog(ctx, usageLog)
			if err != nil {
				a.logger.Error(ctx, "Failed to insert usage log", err)
			}
		}
	}()

	return respChannel, nil

}

func (a *AIProcessor) GenerateImageAI(ctx context.Context, conversationID uuid.UUID, prompt string) (<-chan StreamResponse,
	<-chan ModelResponse) {
	streamingResponseChan := make(chan StreamResponse)
	fullResponseChan := make(chan ModelResponse, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				a.logger.Error(ctx, "Recovered from panic in GenerateImageAI goroutine", fmt.Errorf("reason: %+v", r))
				streamingResponseChan <- StreamResponse{Error: fmt.Errorf("panic: %+v", r)}
				// Optionally also send to fullResponseChan to unblock receiver
				//fullResponseChan <- ModelResponse{Message: fmt.Errorf("panic: %+v", r)}
			}
			close(streamingResponseChan)
			close(fullResponseChan)
		}()

		a.logger.Info(ctx, "Starting AI stream")
		options := []openaiOption.RequestOption{
			openaiOption.WithAPIKey(a.openAiApiKey),
		}
		client := openai.NewClient(options...)

		image, err := client.Images.Generate(ctx, openai.ImageGenerateParams{
			Prompt:         prompt,
			Size:           openai.ImageGenerateParamsSize256x256,
			Model:          openai.ImageModelDallE2,
			ResponseFormat: openai.ImageGenerateParamsResponseFormatB64JSON,
		})
		if err != nil {
			a.logger.Error(ctx, "Failed to generate image", err)
			streamingResponseChan <- StreamResponse{Error: fmt.Errorf("failed to generate image: %w", err)}
			return
		}

		//var totalTokens int32 = 0
		//var fullAssistantMessage strings.Builder

		streamingResponseChan <- StreamResponse{Content: "[Conversation_ID]: " + conversationID.String()}

		streamingResponseChan <- StreamResponse{Content: "[Image_URL]: " + image.Data[0].B64JSON}
		a.logger.Info(ctx, "Stream completed")
		// NOTE: OpenAI Go SDK v1 does not return usage in stream chunks
		fullResponseChan <- ModelResponse{TotalTokens: 0, Message: image.Data[0].B64JSON}
		streamingResponseChan <- StreamResponse{Completed: true}
	}()

	return streamingResponseChan, fullResponseChan
}

func (a *AIProcessor) CreateImageGenerationConversation(ctx context.Context, userID uuid.UUID,
	msg string) (<-chan StreamResponse,
	error) {
	a.logger.Info(ctx, "Creating conversation for user")
	conversation, err := a.store.CreateConversation(ctx, userID)
	if err != nil {
		a.logger.Error(ctx, "Failed to create conversation", err)
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "conversation_id", Value: conversation.ID.String()})
	a.logger.Info(ctx, "Conversation created successfully")

	_, err = a.store.CreateMessage(ctx, conversation.ID, "user", msg)
	if err != nil {
		a.logger.Error(ctx, "Failed to create message", err)
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	respChannel, modelResponseChannel := a.GenerateImageAI(ctx, conversation.ID, msg)

	go func() {
		for resp := range modelResponseChannel {
			_, err := a.store.CreateMessage(ctx, conversation.ID, "assistant", resp.Message)
			if err != nil {
				a.logger.Error(ctx, "Failed to save assistant message", err)
				return
			}

			title, err := a.GenerateTitle(ctx, msg, resp.Message)
			if err == nil {
				err = a.store.UpdateConversationTitleByConversationID(ctx, conversation.ID, title)
				if err != nil {
					a.logger.Error(ctx, "Failed to update conversation title", err)
				}
			} else {
				a.logger.Error(ctx, "Failed to generate title", err)
			}

			usageLog := store.UsageLog{
				UserID:         userID,
				ConversationID: conversation.ID,
				TokensUsed:     resp.TotalTokens,
				CostInCents:    0,
				Model:          "gemini-2.5-pro-preview-03-25",
			}
			_, err = a.store.InsertUsageLog(ctx, usageLog)
			if err != nil {
				a.logger.Error(ctx, "Failed to insert usage log", err)
			}
		}
	}()

	return respChannel, nil
}

func (a *AIProcessor) ContinueImageGenerationConversation(ctx context.Context, userID uuid.UUID,
	conversationID uuid.UUID,
	msg string) (<-chan StreamResponse,
	error) {
	a.logger.Info(ctx, "Continuing conversation for user")
	msgs, err := a.store.GetAllMessagesByConversationID(ctx, conversationID)
	if err != nil {
		a.logger.Error(ctx, "Failed to get messages", err)
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	if len(msgs) == 0 {
		a.logger.Error(ctx, "No messages found for conversation", nil)
		return nil, fmt.Errorf("no messages found for conversation")
	}

	msgs = append(msgs, store.Message{
		Role:    "user",
		Content: msg,
	})

	respChannel, modelResponseChannel := a.GenerateImageAI(ctx, conversationID, msg)

	go func() {
		for resp := range modelResponseChannel {
			_, err = a.store.CreateMessage(ctx, conversationID, "user", msg)
			if err != nil {
				a.logger.Error(ctx, "Failed to create message", err)
				//return nil, fmt.Errorf("failed to create message: %w", err)
			}

			_, err := a.store.CreateMessage(ctx, conversationID, "assistant", resp.Message)
			if err != nil {
				a.logger.Error(ctx, "Failed to save assistant message", err)
				return
			}

			usageLog := store.UsageLog{
				UserID:         userID,
				ConversationID: conversationID,
				TokensUsed:     resp.TotalTokens,
				CostInCents:    0,
				Model:          "gemini-2.5-pro-preview-03-25",
			}
			_, err = a.store.InsertUsageLog(ctx, usageLog)
			if err != nil {
				a.logger.Error(ctx, "Failed to insert usage log", err)
			}
		}
	}()

	return respChannel, nil

}

// TranscribeWithWhisper sends audio bytes to the OpenAI Whisper API and returns the transcript using the openai-go SDK.
func (a *AIProcessor) TranscribeWithWhisper(ctx context.Context, audio []byte) (string, error) {
	if a.openAiApiKey == "" {
		return "", errors.New("OpenAI API key not set")
	}
	client := openai.NewClient(
		openaiOption.WithAPIKey(a.openAiApiKey),
	)

	file := openai.File(bytes.NewReader(audio), "audio.wav", "audio/wav")
	params := openai.AudioTranscriptionNewParams{
		Model: openai.AudioModelWhisper1,
		File:  file,
	}
	resp, err := client.Audio.Transcriptions.New(ctx, params)
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

// TranscribeWithWhisperRealtime streams audio to OpenAI's real-time endpoint and returns a channel of transcription results.
func (a *AIProcessor) TranscribeWithWhisperRealtime(ctx context.Context, audioStream <-chan []byte, cfg openai2.RealtimeTranscriptionConfig) (<-chan openai2.TranscriptionResult, error) {
	if a.openaiRealtime == nil {
		return nil, errors.New("OpenAIRealtimeClient not set in AIProcessor")
	}
	return a.openaiRealtime.StartRealtimeTranscription(ctx, audioStream, cfg)
}

func (a *AIProcessor) TranscribeTwilioCall(ctx context.Context) (chan []byte, chan struct{}) {
	// Channel for streaming audio to the AI processor
	audioChan := make(chan []byte, 32)
	// Channel for signaling when to stop transcription
	stopChan := make(chan struct{})

	// Start transcription goroutine
	go func() {
		cfg := openai2.RealtimeTranscriptionConfig{
			Model:    "whisper-1",
			Language: "en",
			// Add additional config as needed
		}

		results, err := a.openaiRealtime.StartRealtimeTranscription(ctx, audioChan, cfg)
		if err != nil {
			log.Println("❌ Real-time transcription failed:", err)
			return
		}
		for res := range results {
			if res.Type == "delta" && res.Delta != "" {
				log.Printf("📝 Whisper delta: %s", res.Delta)
				// Optionally: send delta to client (e.g., via WebSocket)
			} else if res.Type == "completed" && res.Transcript != "" {
				log.Printf("📝 Whisper transcript: %s", res.Transcript)
				// Optionally: send final transcript to client or store
			}
		}
		close(stopChan)
	}()

	return audioChan, stopChan
}
