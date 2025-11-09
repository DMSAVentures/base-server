package processor

import (
	"base-server/internal/store"
	"context"

	"github.com/google/uuid"
)

// AIStore defines the database operations required by AIProcessor
type AIStore interface {
	CreateConversation(ctx context.Context, userID uuid.UUID) (*store.Conversation, error)
	CreateMessage(ctx context.Context, conversationID uuid.UUID, role, content string) (*store.Message, error)
	UpdateConversationTitleByConversationID(ctx context.Context, conversationID uuid.UUID, title string) error
	InsertUsageLog(ctx context.Context, usageLog store.UsageLog) (store.UsageLog, error)
	GetAllMessagesByConversationID(ctx context.Context, conversationID uuid.UUID) ([]store.Message, error)
}
