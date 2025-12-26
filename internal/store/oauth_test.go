package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestStore_CreateUserOnGoogleSignIn(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("successful google user creation", func(t *testing.T) {
		t.Parallel()
		googleUserID := "google_" + uuid.New().String()
		email := uuid.New().String() + "@example.com"

		user, err := testDB.Store.CreateUserOnGoogleSignIn(ctx, googleUserID, email, "Google", "User")
		if err != nil {
			t.Errorf("CreateUserOnGoogleSignIn() error = %v", err)
			return
		}
		if user.ID == uuid.Nil {
			t.Error("expected user ID to be set")
		}
		if user.FirstName != "Google" {
			t.Errorf("FirstName = %v, want Google", user.FirstName)
		}
		if user.LastName != "User" {
			t.Errorf("LastName = %v, want User", user.LastName)
		}

		// Verify oauth auth was created
		oauthAuth, err := testDB.Store.GetOauthUserByEmail(ctx, email)
		if err != nil {
			t.Errorf("failed to get oauth user by email: %v", err)
			return
		}
		if oauthAuth.Email != email {
			t.Errorf("Email = %v, want %v", oauthAuth.Email, email)
		}
		if oauthAuth.ExternalID != googleUserID {
			t.Errorf("ExternalID = %v, want %v", oauthAuth.ExternalID, googleUserID)
		}
		if oauthAuth.AuthProvider != "google" {
			t.Errorf("AuthProvider = %v, want google", oauthAuth.AuthProvider)
		}
	})

	t.Run("google user with special characters", func(t *testing.T) {
		t.Parallel()
		googleUserID := "google_" + uuid.New().String()
		email := uuid.New().String() + "+tag@gmail.com"

		user, err := testDB.Store.CreateUserOnGoogleSignIn(ctx, googleUserID, email, "José", "García")
		if err != nil {
			t.Errorf("CreateUserOnGoogleSignIn() error = %v", err)
			return
		}
		if user.ID == uuid.Nil {
			t.Error("expected user ID to be set")
		}
	})
}

func TestStore_GetOauthUserByEmail(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("get existing oauth user", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "OAuth", "User")
		authID := createTestUserAuth(t, testDB, user.ID, "oauth")
		externalID := "google_" + uuid.New().String()
		email := uuid.New().String() + "@example.com"
		createTestOAuthAuth(t, testDB, authID, externalID, email, "OAuth User", "google")

		auth, err := testDB.Store.GetOauthUserByEmail(ctx, email)
		if err != nil {
			t.Errorf("GetOauthUserByEmail() error = %v", err)
			return
		}
		if auth.Email != email {
			t.Errorf("Email = %v, want %v", auth.Email, email)
		}
		if auth.ExternalID != externalID {
			t.Errorf("ExternalID = %v, want %v", auth.ExternalID, externalID)
		}
		if auth.AuthProvider != "google" {
			t.Errorf("AuthProvider = %v, want google", auth.AuthProvider)
		}
		if auth.FullName != "OAuth User" {
			t.Errorf("FullName = %v, want OAuth User", auth.FullName)
		}
		if auth.AuthID != authID {
			t.Errorf("AuthID = %v, want %v", auth.AuthID, authID)
		}
	})

	t.Run("oauth user does not exist", func(t *testing.T) {
		t.Parallel()
		_, err := testDB.Store.GetOauthUserByEmail(ctx, "nonexistent_"+uuid.New().String()+"@example.com")
		if err == nil {
			t.Error("GetOauthUserByEmail() expected error for non-existent user")
		}
	})
}

func TestStore_GetOauthUserByEmail_MultipleProviders(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	// Setup: Create users with different OAuth providers
	t.Run("get google oauth user", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "Google", "User")
		authID := createTestUserAuth(t, testDB, user.ID, "oauth")
		googleEmail := uuid.New().String() + "@gmail.com"
		googleExternalID := "google_" + uuid.New().String()
		createTestOAuthAuth(t, testDB, authID, googleExternalID, googleEmail, "Google User", "google")

		auth, err := testDB.Store.GetOauthUserByEmail(ctx, googleEmail)
		if err != nil {
			t.Errorf("GetOauthUserByEmail() error = %v", err)
			return
		}
		if auth.AuthProvider != "google" {
			t.Errorf("AuthProvider = %v, want google", auth.AuthProvider)
		}
		if auth.ExternalID != googleExternalID {
			t.Errorf("ExternalID = %v, want %v", auth.ExternalID, googleExternalID)
		}
	})

	t.Run("get apple oauth user", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "Apple", "User")
		authID := createTestUserAuth(t, testDB, user.ID, "oauth")
		appleEmail := uuid.New().String() + "@icloud.com"
		appleExternalID := "apple_" + uuid.New().String()
		createTestOAuthAuth(t, testDB, authID, appleExternalID, appleEmail, "Apple User", "apple")

		auth, err := testDB.Store.GetOauthUserByEmail(ctx, appleEmail)
		if err != nil {
			t.Errorf("GetOauthUserByEmail() error = %v", err)
			return
		}
		if auth.AuthProvider != "apple" {
			t.Errorf("AuthProvider = %v, want apple", auth.AuthProvider)
		}
		if auth.ExternalID != appleExternalID {
			t.Errorf("ExternalID = %v, want %v", auth.ExternalID, appleExternalID)
		}
	})
}
