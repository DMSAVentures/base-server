package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestStore_CreateConversation(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) uuid.UUID
		wantErr  bool
		validate func(t *testing.T, conversation *Conversation, userID uuid.UUID)
	}{
		{
			name: "create conversation successfully",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "John", "Doe")
				return user.ID
			},
			wantErr: false,
			validate: func(t *testing.T, conversation *Conversation, userID uuid.UUID) {
				t.Helper()
				if conversation.UserID != userID {
					t.Errorf("UserID = %v, want %v", conversation.UserID, userID)
				}
				if conversation.ID == uuid.Nil {
					t.Error("expected non-nil conversation ID")
				}
			},
		},
		{
			name: "create another conversation for different user",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Jane", "Smith")
				return user.ID
			},
			wantErr: false,
			validate: func(t *testing.T, conversation *Conversation, userID uuid.UUID) {
				t.Helper()
				if conversation.UserID != userID {
					t.Errorf("UserID = %v, want %v", conversation.UserID, userID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			userID := tt.setup(t)

			conversation, err := testDB.Store.CreateConversation(ctx, userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateConversation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, conversation, userID)
			}
		})
	}
}

func TestStore_GetConversation(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) uuid.UUID
		wantErr  bool
		validate func(t *testing.T, conversation *Conversation)
	}{
		{
			name: "get existing conversation",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "John", "Doe")
				conversation, _ := testDB.Store.CreateConversation(ctx, user.ID)
				return conversation.ID
			},
			wantErr: false,
			validate: func(t *testing.T, conversation *Conversation) {
				t.Helper()
				if conversation.ID == uuid.Nil {
					t.Error("expected non-nil conversation ID")
				}
			},
		},
		{
			name: "conversation does not exist",
			setup: func(t *testing.T) uuid.UUID {
				return uuid.New()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			conversationID := tt.setup(t)

			conversation, err := testDB.Store.GetConversation(ctx, conversationID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetConversation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, conversation)
			}
		})
	}
}

func TestStore_GetAllConversationsByUserID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(t *testing.T) uuid.UUID
		wantCount int
		wantErr   bool
	}{
		{
			name: "get multiple conversations for user",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "John", "Doe")
				testDB.Store.CreateConversation(ctx, user.ID)
				testDB.Store.CreateConversation(ctx, user.ID)
				testDB.Store.CreateConversation(ctx, user.ID)
				return user.ID
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name: "user with no conversations",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Jane", "Smith")
				return user.ID
			},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			userID := tt.setup(t)

			conversations, err := testDB.Store.GetAllConversationsByUserID(ctx, userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAllConversationsByUserID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(conversations) != tt.wantCount {
				t.Errorf("GetAllConversationsByUserID() count = %v, want %v", len(conversations), tt.wantCount)
			}
		})
	}
}

func TestStore_CreateMessage(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) (uuid.UUID, string, string)
		wantErr  bool
		validate func(t *testing.T, message *Message, role, content string)
	}{
		{
			name: "create user message",
			setup: func(t *testing.T) (uuid.UUID, string, string) {
				t.Helper()
				user, _ := createTestUser(t, testDB, "John", "Doe")
				conversation, _ := testDB.Store.CreateConversation(ctx, user.ID)
				return conversation.ID, MessageRoleUser, "Hello, how are you?"
			},
			wantErr: false,
			validate: func(t *testing.T, message *Message, role, content string) {
				t.Helper()
				if message.Role != role {
					t.Errorf("Role = %v, want %v", message.Role, role)
				}
				if message.Content != content {
					t.Errorf("Content = %v, want %v", message.Content, content)
				}
			},
		},
		{
			name: "create assistant message",
			setup: func(t *testing.T) (uuid.UUID, string, string) {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Jane", "Smith")
				conversation, _ := testDB.Store.CreateConversation(ctx, user.ID)
				return conversation.ID, MessageRoleAssistant, "I'm doing well, thank you!"
			},
			wantErr: false,
			validate: func(t *testing.T, message *Message, role, content string) {
				t.Helper()
				if message.Role != MessageRoleAssistant {
					t.Errorf("Role = %v, want %v", message.Role, MessageRoleAssistant)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			conversationID, role, content := tt.setup(t)

			message, err := testDB.Store.CreateMessage(ctx, conversationID, role, content)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, message, role, content)
			}
		})
	}
}

func TestStore_GetAllMessagesByConversationID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(t *testing.T) uuid.UUID
		wantCount int
		wantErr   bool
	}{
		{
			name: "get multiple messages in conversation",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "John", "Doe")
				conversation, _ := testDB.Store.CreateConversation(ctx, user.ID)
				testDB.Store.CreateMessage(ctx, conversation.ID, MessageRoleUser, "Hello")
				testDB.Store.CreateMessage(ctx, conversation.ID, MessageRoleAssistant, "Hi there!")
				testDB.Store.CreateMessage(ctx, conversation.ID, MessageRoleUser, "How are you?")
				return conversation.ID
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name: "conversation with no messages",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Jane", "Smith")
				conversation, _ := testDB.Store.CreateConversation(ctx, user.ID)
				return conversation.ID
			},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			conversationID := tt.setup(t)

			messages, err := testDB.Store.GetAllMessagesByConversationID(ctx, conversationID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAllMessagesByConversationID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(messages) != tt.wantCount {
				t.Errorf("GetAllMessagesByConversationID() count = %v, want %v", len(messages), tt.wantCount)
			}
		})
	}
}

func TestStore_UpdateConversationTitleByConversationID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) uuid.UUID
		newTitle string
		wantErr  bool
	}{
		{
			name: "update conversation title",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "John", "Doe")
				conversation, _ := testDB.Store.CreateConversation(ctx, user.ID)
				return conversation.ID
			},
			newTitle: "Discussion about Go programming",
			wantErr:  false,
		},
		{
			name: "update non-existent conversation",
			setup: func(t *testing.T) uuid.UUID {
				return uuid.New()
			},
			newTitle: "Some title",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			conversationID := tt.setup(t)

			err := testDB.Store.UpdateConversationTitleByConversationID(ctx, conversationID, tt.newTitle)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateConversationTitleByConversationID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the update
				conversation, err := testDB.Store.GetConversation(ctx, conversationID)
				if err != nil {
					t.Errorf("failed to get updated conversation: %v", err)
					return
				}
				if !conversation.Title.Valid || conversation.Title.String != tt.newTitle {
					t.Errorf("Title = %v, want %v", conversation.Title.String, tt.newTitle)
				}
			}
		})
	}
}
