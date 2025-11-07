package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type Conversation struct {
	ID        uuid.UUID      `db:"id"`
	UserID    uuid.UUID      `db:"user_id"`
	Title     sql.NullString `db:"title"`
	CreatedAt string         `db:"created_at"`
	UpdatedAt string         `db:"updated_at"`
}

type Message struct {
	ID             uuid.UUID `db:"id"`
	ConversationID uuid.UUID `db:"conversation_id"`
	Role           string    `db:"role"`
	Content        string    `db:"content"`
	CreatedAt      string    `db:"created_at"`
}

const MessageRoleUser = "user"
const MessageRoleAssistant = "assistant"

const sqlGetConversationByID = `
SELECT * FROM conversations WHERE id = $1`

func (s *Store) GetConversation(ctx context.Context, id uuid.UUID) (*Conversation, error) {
	var conversation Conversation
	err := s.db.GetContext(ctx, &conversation, sqlGetConversationByID, id)
	if err != nil {
		s.logger.Error(ctx, "failed to get conversation by ID", err)
		return nil, fmt.Errorf("failed to get conversation by ID: %w", err)
	}
	return &conversation, nil
}

const sqlCreateConversationForUserID = `
INSERT INTO conversations (user_id)
VALUES ($1)
RETURNING id, user_id, title, created_at, updated_at`

func (s *Store) CreateConversation(ctx context.Context, userID uuid.UUID) (*Conversation, error) {
	var conversation Conversation
	err := s.db.GetContext(ctx, &conversation, sqlCreateConversationForUserID, userID)
	if err != nil {
		s.logger.Error(ctx, "failed to create conversation", err)
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}
	return &conversation, nil
}

const sqlGetAllConversationsByUserID = `
SELECT * FROM conversations WHERE user_id = $1`

func (s *Store) GetAllConversationsByUserID(ctx context.Context, userID uuid.UUID) ([]Conversation, error) {
	var conversations []Conversation
	err := s.db.SelectContext(ctx, &conversations, sqlGetAllConversationsByUserID, userID)
	if err != nil {
		s.logger.Error(ctx, "failed to get all conversations by user ID", err)
		return nil, fmt.Errorf("failed to get all conversations by user ID: %w", err)
	}
	return conversations, nil
}

const sqlGetAllMessagesByConversationID = `
SELECT * FROM messages WHERE conversation_id = $1 ORDER BY created_at ASC`

func (s *Store) GetAllMessagesByConversationID(ctx context.Context, conversationID uuid.UUID) ([]Message, error) {
	var messages []Message
	err := s.db.SelectContext(ctx, &messages, sqlGetAllMessagesByConversationID, conversationID)
	if err != nil {
		s.logger.Error(ctx, "failed to get all messages by conversation ID", err)
		return nil, fmt.Errorf("failed to get all messages by conversation ID: %w", err)
	}
	return messages, nil
}

const sqlCreateMessageForConversationID = `
INSERT INTO messages (conversation_id, role, content)
VALUES ($1, $2, $3)
RETURNING id, conversation_id, role, content, created_at`

func (s *Store) CreateMessage(ctx context.Context, conversationID uuid.UUID, role, content string) (*Message, error) {
	var message Message
	err := s.db.GetContext(ctx, &message, sqlCreateMessageForConversationID, conversationID, role, content)
	if err != nil {
		s.logger.Error(ctx, "failed to create message", err)
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	return &message, nil
}

const sqlUpdateConversationTitleByConversationID = `
UPDATE conversations SET title = $1 WHERE id = $2
`

func (s *Store) UpdateConversationTitleByConversationID(ctx context.Context, conversationID uuid.UUID,
	title string) error {
	result, err := s.db.ExecContext(ctx, sqlUpdateConversationTitleByConversationID, title, conversationID)
	if err != nil {
		s.logger.Error(ctx, "failed to update conversation title", err)
		return fmt.Errorf("failed to update conversation title: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		s.logger.Error(ctx, "failed to get rows affected", err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}
