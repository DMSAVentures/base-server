package processor

import (
	"base-server/internal/ai-capabilities/processor"
	"base-server/internal/observability"
)

type VoiceCallProcessor struct {
	aiProcessor *processor.AIProcessor
	logger      *observability.Logger
}

func NewVoiceCallProcessor(aiProcessor *processor.AIProcessor, logger *observability.Logger) *VoiceCallProcessor {
	return &VoiceCallProcessor{
		aiProcessor: aiProcessor,
		logger:      logger,
	}
}
