package ai_capabilities

import (
	"base-server/internal/observability"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type GeminiAI struct {
	logger *observability.Logger
	apiKey string
}

func New(logger *observability.Logger, apiKey string) *GeminiAI {
	return &GeminiAI{
		logger: logger,
		apiKey: apiKey,
	}
}

type StreamResponse struct {
	Content string
	Error   error
}

func (g *GeminiAI) DoSomething(ctx context.Context) <-chan StreamResponse {
	responseChan := make(chan StreamResponse)
	
	go func() {
		defer close(responseChan)
		
		g.logger.Info(ctx, "Starting AI stream")
		c, err := genai.NewClient(ctx, option.WithAPIKey(g.apiKey))
		if err != nil {
			g.logger.Error(ctx, "Failed to create client", err)
			responseChan <- StreamResponse{Error: fmt.Errorf("failed to create AI client: %w", err)}
			return
		}
		defer c.Close()

		model := c.GenerativeModel("gemini-2.5-pro-preview-03-25")
		iter := model.GenerateContentStream(ctx, genai.Text("Describe what is transformer model"))
		
		for {
			select {
			case <-ctx.Done():
				g.logger.Info(ctx, "Context cancelled, stopping stream")
				return
			default:
				resp, err := iter.Next()
				if err == iterator.Done {
					g.logger.Info(ctx, "Stream completed")
					return
				}
				if err != nil {
					g.logger.Error(ctx, "Failed to get next response", err)
					responseChan <- StreamResponse{Error: fmt.Errorf("failed to get AI response: %w", err)}
					return
				}

				for _, part := range resp.Candidates[0].Content.Parts {
					bs, err := json.Marshal(part)
					if err != nil {
						g.logger.Error(ctx, "Failed to marshal response part", err)
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