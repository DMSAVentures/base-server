package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/genai"
)

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

func (a *AIProcessor) ChatTextAI(ctx context.Context, conversationID uuid.UUID,
	messages []store.Message) (<-chan StreamResponse,
	<-chan ModelResponse) {
	streamingResponseChan := make(chan StreamResponse)
	fullResponseChan := make(chan ModelResponse, 1)

	var allMessages []*genai.Content

	for _, m := range messages {
		role := "user"
		if m.Role == "assistant" {
			role = "model" // Gemini SDK expects "model"
		}
		// Use genai.Text helper which returns []*Content
		content := genai.Text(m.Content)
		if len(content) > 0 {
			content[0].Role = role
			allMessages = append(allMessages, content[0])
		}
	}

	go func() {
		defer close(streamingResponseChan)
		defer close(fullResponseChan)

		a.logger.Info(ctx, "Starting AI stream")
		c, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey: a.geminiApiKey,
		})
		if err != nil {
			a.logger.Error(ctx, "Failed to create client", err)
			streamingResponseChan <- StreamResponse{Error: fmt.Errorf("failed to create AI client: %w", err)}
			return
		}

		// Use the Models API with GenerateContentStream
		iter := c.Models.GenerateContentStream(ctx, "gemini-2.5-pro-preview-03-25", allMessages, nil)

		var totalTokens int32 = 0
		var fullAssistantMessage strings.Builder

		streamingResponseChan <- StreamResponse{Content: "[Conversation_ID]: " + conversationID.String()}

		// New iterator API uses range-over-func
		for resp, err := range iter {
			select {
			case <-ctx.Done():
				a.logger.Info(ctx, "Context cancelled, stopping stream")
				fullResponseChan <- ModelResponse{TotalTokens: totalTokens, Message: fullAssistantMessage.String()}
				return
			default:
				if err != nil {
					a.logger.Error(ctx, "Failed to get next response", err)
					streamingResponseChan <- StreamResponse{Error: fmt.Errorf("failed to get AI response: %w", err)}
					return
				}

				for _, part := range resp.Candidates[0].Content.Parts {
					// Check if the part has text
					if part.Text != "" {
						streamingResponseChan <- StreamResponse{Content: part.Text}
						fullAssistantMessage.WriteString(part.Text)
					} else {
						// For non-text parts, marshal to JSON
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
				}

				if resp.UsageMetadata != nil {
					totalTokens = resp.UsageMetadata.CandidatesTokenCount + resp.UsageMetadata.PromptTokenCount
				}
			}
		}
		// Stream completed
		a.logger.Info(ctx, "Stream completed")
		fullResponseChan <- ModelResponse{TotalTokens: totalTokens, Message: fullAssistantMessage.String()}
		streamingResponseChan <- StreamResponse{Completed: true}
	}()

	return streamingResponseChan, fullResponseChan
}
