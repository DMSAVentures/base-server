package processor

import (
	"context"
)

func (v *VoiceCallProcessor) StartTwilioVoiceAgent(ctx context.Context) (chan []byte, chan []byte) {
	return v.aiProcessor.StartVoiceAgent(ctx)
}

func (v *VoiceCallProcessor) StartTwilioTranscribeAgent(ctx context.Context) (chan []byte, chan struct{}) {
	return v.aiProcessor.TranscribeAudio(ctx)
}
