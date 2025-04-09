package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type AIProcessor struct {
	logger *observability.Logger
	apiKey string
	store  store.Store
}

func New(logger *observability.Logger, apiKey string, store store.Store) *AIProcessor {
	return &AIProcessor{
		logger: logger,
		apiKey: apiKey,
		store:  store,
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

func (a *AIProcessor) DoSomething(ctx context.Context) <-chan StreamResponse {
	responseChan := make(chan StreamResponse)

	go func() {
		defer close(responseChan)

		a.logger.Info(ctx, "Starting AI stream")
		c, err := genai.NewClient(ctx, option.WithAPIKey(a.apiKey))
		if err != nil {
			a.logger.Error(ctx, "Failed to create client", err)
			responseChan <- StreamResponse{Error: fmt.Errorf("failed to create AI client: %w", err)}
			return
		}
		defer c.Close()

		model := c.GenerativeModel("gemini-2.5-pro-preview-03-25")
		iter := model.GenerateContentStream(ctx, genai.Text("Say hello to me and tell me about weather in Ottawa"))

		for {
			select {
			case <-ctx.Done():
				a.logger.Info(ctx, "Context cancelled, stopping stream")
				return
			default:
				resp, err := iter.Next()
				if err == iterator.Done {
					a.logger.Info(ctx, "Stream completed")
					responseChan <- StreamResponse{Completed: true}
					return
				}
				if err != nil {
					a.logger.Error(ctx, "Failed to get next response", err)
					responseChan <- StreamResponse{Error: fmt.Errorf("failed to get AI response: %w", err)}
					return
				}

				for _, part := range resp.Candidates[0].Content.Parts {
					bs, err := json.Marshal(part)
					if err != nil {
						a.logger.Error(ctx, "Failed to marshal response part", err)
						responseChan <- StreamResponse{Error: fmt.Errorf("failed to marshal response: %w", err)}
						continue
					}
					responseChan <- StreamResponse{Content: string(bs)}
				}
			}
		}
	}()

	return responseChan
}

func (a *AIProcessor) ChatWithGemini(ctx context.Context, messages []string) (<-chan StreamResponse,
	<-chan ModelResponse) {
	streamingResponseChan := make(chan StreamResponse)
	fullResponseChan := make(chan ModelResponse, 1)
	var geminiParts []genai.Part

	for _, message := range messages {
		part := genai.Text(message)
		geminiParts = append(geminiParts, part)
	}

	go func() {
		defer close(streamingResponseChan)
		defer close(fullResponseChan)

		a.logger.Info(ctx, "Starting AI stream")
		c, err := genai.NewClient(ctx, option.WithAPIKey(a.apiKey))
		if err != nil {
			a.logger.Error(ctx, "Failed to create client", err)
			streamingResponseChan <- StreamResponse{Error: fmt.Errorf("failed to create AI client: %w", err)}
			return
		}
		defer c.Close()

		model := c.GenerativeModel("gemini-2.5-pro-preview-03-25")
		iter := model.GenerateContentStream(ctx, geminiParts...)

		var totalTokens int32 = 0
		var fullAssistantMessage strings.Builder

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

	c, err := genai.NewClient(ctx, option.WithAPIKey(a.apiKey))
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

	respChannel, modelResponseChannel := a.ChatWithGemini(ctx, []string{msg})

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
