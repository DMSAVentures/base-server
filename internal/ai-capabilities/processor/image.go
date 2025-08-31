package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/openai/openai-go"
	openaiOption "github.com/openai/openai-go/option"
)

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
			Prompt: prompt,
			Size:   openai.ImageGenerateParamsSize1024x1024,
			Model:  openai.ImageModelGPTImage1,
			//ResponseFormat: openai.ImageGenerateParamsResponseFormatB64JSON,
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
