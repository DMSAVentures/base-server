package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestStore_CreateConversation(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("create conversation successfully", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "John", "Doe")

		conversation, err := testDB.Store.CreateConversation(ctx, user.ID)
		if err != nil {
			t.Errorf("CreateConversation() error = %v", err)
			return
		}
		if conversation.UserID != user.ID {
			t.Errorf("UserID = %v, want %v", conversation.UserID, user.ID)
		}
		if conversation.ID == uuid.Nil {
			t.Error("expected non-nil conversation ID")
		}
	})

	t.Run("create another conversation for different user", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "Jane", "Smith")

		conversation, err := testDB.Store.CreateConversation(ctx, user.ID)
		if err != nil {
			t.Errorf("CreateConversation() error = %v", err)
			return
		}
		if conversation.UserID != user.ID {
			t.Errorf("UserID = %v, want %v", conversation.UserID, user.ID)
		}
	})
}

func TestStore_GetConversation(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("get existing conversation", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "John", "Doe")
		created, err := testDB.Store.CreateConversation(ctx, user.ID)
		if err != nil {
			t.Fatalf("failed to create conversation: %v", err)
		}

		conversation, err := testDB.Store.GetConversation(ctx, created.ID)
		if err != nil {
			t.Errorf("GetConversation() error = %v", err)
			return
		}
		if conversation.ID == uuid.Nil {
			t.Error("expected non-nil conversation ID")
		}
	})

	t.Run("conversation does not exist", func(t *testing.T) {
		t.Parallel()
		_, err := testDB.Store.GetConversation(ctx, uuid.New())
		if err == nil {
			t.Error("GetConversation() expected error for non-existent conversation")
		}
	})
}

func TestStore_GetAllConversationsByUserID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("get multiple conversations for user", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "John", "Doe")
		_, _ = testDB.Store.CreateConversation(ctx, user.ID)
		_, _ = testDB.Store.CreateConversation(ctx, user.ID)
		_, _ = testDB.Store.CreateConversation(ctx, user.ID)

		conversations, err := testDB.Store.GetAllConversationsByUserID(ctx, user.ID)
		if err != nil {
			t.Errorf("GetAllConversationsByUserID() error = %v", err)
			return
		}
		if len(conversations) != 3 {
			t.Errorf("GetAllConversationsByUserID() count = %v, want 3", len(conversations))
		}
	})

	t.Run("user with no conversations", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "Jane", "Smith")

		conversations, err := testDB.Store.GetAllConversationsByUserID(ctx, user.ID)
		if err != nil {
			t.Errorf("GetAllConversationsByUserID() error = %v", err)
			return
		}
		if len(conversations) != 0 {
			t.Errorf("GetAllConversationsByUserID() count = %v, want 0", len(conversations))
		}
	})
}

func TestStore_CreateMessage(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("create user message", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "John", "Doe")
		conversation, _ := testDB.Store.CreateConversation(ctx, user.ID)

		message, err := testDB.Store.CreateMessage(ctx, conversation.ID, MessageRoleUser, "Hello, how are you?")
		if err != nil {
			t.Errorf("CreateMessage() error = %v", err)
			return
		}
		if message.Role != MessageRoleUser {
			t.Errorf("Role = %v, want %v", message.Role, MessageRoleUser)
		}
		if message.Content != "Hello, how are you?" {
			t.Errorf("Content = %v, want Hello, how are you?", message.Content)
		}
	})

	t.Run("create assistant message", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "Jane", "Smith")
		conversation, _ := testDB.Store.CreateConversation(ctx, user.ID)

		message, err := testDB.Store.CreateMessage(ctx, conversation.ID, MessageRoleAssistant, "I'm doing well, thank you!")
		if err != nil {
			t.Errorf("CreateMessage() error = %v", err)
			return
		}
		if message.Role != MessageRoleAssistant {
			t.Errorf("Role = %v, want %v", message.Role, MessageRoleAssistant)
		}
	})
}

func TestStore_GetAllMessagesByConversationID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("get multiple messages in conversation", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "John", "Doe")
		conversation, _ := testDB.Store.CreateConversation(ctx, user.ID)
		_, _ = testDB.Store.CreateMessage(ctx, conversation.ID, MessageRoleUser, "Hello")
		_, _ = testDB.Store.CreateMessage(ctx, conversation.ID, MessageRoleAssistant, "Hi there!")
		_, _ = testDB.Store.CreateMessage(ctx, conversation.ID, MessageRoleUser, "How are you?")

		messages, err := testDB.Store.GetAllMessagesByConversationID(ctx, conversation.ID)
		if err != nil {
			t.Errorf("GetAllMessagesByConversationID() error = %v", err)
			return
		}
		if len(messages) != 3 {
			t.Errorf("GetAllMessagesByConversationID() count = %v, want 3", len(messages))
		}
	})

	t.Run("conversation with no messages", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "Jane", "Smith")
		conversation, _ := testDB.Store.CreateConversation(ctx, user.ID)

		messages, err := testDB.Store.GetAllMessagesByConversationID(ctx, conversation.ID)
		if err != nil {
			t.Errorf("GetAllMessagesByConversationID() error = %v", err)
			return
		}
		if len(messages) != 0 {
			t.Errorf("GetAllMessagesByConversationID() count = %v, want 0", len(messages))
		}
	})
}

func TestStore_UpdateConversationTitleByConversationID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("update conversation title", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "John", "Doe")
		conversation, _ := testDB.Store.CreateConversation(ctx, user.ID)

		newTitle := "Discussion about Go programming"
		err := testDB.Store.UpdateConversationTitleByConversationID(ctx, conversation.ID, newTitle)
		if err != nil {
			t.Errorf("UpdateConversationTitleByConversationID() error = %v", err)
			return
		}

		// Verify the update
		updated, err := testDB.Store.GetConversation(ctx, conversation.ID)
		if err != nil {
			t.Errorf("failed to get updated conversation: %v", err)
			return
		}
		if !updated.Title.Valid || updated.Title.String != newTitle {
			t.Errorf("Title = %v, want %v", updated.Title.String, newTitle)
		}
	})

	t.Run("update non-existent conversation", func(t *testing.T) {
		t.Parallel()
		err := testDB.Store.UpdateConversationTitleByConversationID(ctx, uuid.New(), "Some title")
		if err == nil {
			t.Error("UpdateConversationTitleByConversationID() expected error for non-existent conversation")
		}
	})
}
