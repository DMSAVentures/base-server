package ai_capabilities

import (
	"base-server/internal/observability"
	"context"
	"encoding/json"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type GeminiAI struct {
	// Add fields as needed
	logger *observability.Logger
	apiKey string
}

func New(logger *observability.Logger, apiKey string) *GeminiAI {
	return &GeminiAI{
		logger: logger,
		apiKey: apiKey,
	}
}

func (g *GeminiAI) DoSomething(ctx context.Context) {
	// Implement the functionality here
	g.logger.Info(ctx, "Doing something in GeminiAI")
	c, err := genai.NewClient(ctx, option.WithAPIKey(g.apiKey))
	if err != nil {
		g.logger.Error(ctx, "Failed to create client", err)
		return
	}

	model := c.GenerativeModel("gemini-2.5-pro-preview-03-25")
	resp, err := model.GenerateContent(ctx, genai.Text("Describe what is transformer model"))
	if err != nil {
		g.logger.Error(ctx, "Failed to generate content", err)
		return
	}
	bs, err := json.Marshal(resp.Candidates[0].Content.Parts[0])

	if err != nil {
		g.logger.Error(ctx, "Failed to marshal response", err)
		return
	}

	g.logger.Info(ctx, string(bs))
}
