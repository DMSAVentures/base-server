package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type AIProcessor struct {
	logger *observability.Logger
	apiKey string
	store  *store.Store
}

func New(logger *observability.Logger, apiKey string, store *store.Store) *AIProcessor {
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

type TokenConsumedCount struct {
	TotalTokens int32
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
	<-chan TokenConsumedCount) {
	responseChan := make(chan StreamResponse)
	tokensConsumeChan := make(chan TokenConsumedCount)
	var geminiParts []genai.Part

	for _, message := range messages {
		part := genai.Text(message)
		geminiParts = append(geminiParts, part)
	}

	go func() {
		defer close(responseChan)
		defer close(tokensConsumeChan)

		a.logger.Info(ctx, "Starting AI stream")
		c, err := genai.NewClient(ctx, option.WithAPIKey(a.apiKey))
		if err != nil {
			a.logger.Error(ctx, "Failed to create client", err)
			responseChan <- StreamResponse{Error: fmt.Errorf("failed to create AI client: %w", err)}
			return
		}
		defer c.Close()

		model := c.GenerativeModel("gemini-2.5-pro-preview-03-25")
		iter := model.GenerateContentStream(ctx, geminiParts...)

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
				//tokensConsumeChan <- TokenConsumedCount{TotalTokens: resp.UsageMetadata.CandidatesTokenCount}
			}
		}
	}()

	return responseChan, tokensConsumeChan
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

	_, err = a.store.CreateMessage(ctx, conversation.ID, "user", msg, len(msg))
	if err != nil {
		a.logger.Error(ctx, "Failed to create message", err)
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	respChannel, totalTokensConsumedChannel := a.ChatWithGemini(ctx, []string{msg})
	go func() {
		for tokenCount := range totalTokensConsumedChannel {
			usageLog := store.UsageLog{
				UserID:         userID,
				ConversationID: conversation.ID,
				MessageID:      conversation.ID,
				TokensUsed:     int(tokenCount.TotalTokens),
				Model:          "gemini-2.5-pro-preview-03-25",
			}
			_, err := a.store.InsertUsageLog(ctx, usageLog)
			if err != nil {
				a.logger.Error(ctx, "Failed to insert usage log", err)
			}
		}
	}()

	return respChannel, nil
}
