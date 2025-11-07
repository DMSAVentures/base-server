package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestStore_CreateUserOnGoogleSignIn(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name         string
		googleUserID string
		email        string
		firstName    string
		lastName     string
		wantErr      bool
		validate     func(t *testing.T, user User)
	}{
		{
			name:         "successful google user creation",
			googleUserID: "google_12345",
			email:        "google_user@example.com",
			firstName:    "Google",
			lastName:     "User",
			wantErr:      false,
			validate: func(t *testing.T, user User) {
				t.Helper()
				if user.ID == uuid.Nil {
					t.Error("expected user ID to be set")
				}
				if user.FirstName != "Google" {
					t.Errorf("FirstName = %v, want Google", user.FirstName)
				}
				if user.LastName != "User" {
					t.Errorf("LastName = %v, want User", user.LastName)
				}
			},
		},
		{
			name:         "google user with special characters",
			googleUserID: "google_xyz_789",
			email:        "special.user+tag@gmail.com",
			firstName:    "José",
			lastName:     "García",
			wantErr:      false,
			validate: func(t *testing.T, user User) {
				t.Helper()
				if user.ID == uuid.Nil {
					t.Error("expected user ID to be set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)

			user, err := testDB.Store.CreateUserOnGoogleSignIn(ctx, tt.googleUserID, tt.email, tt.firstName, tt.lastName)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateUserOnGoogleSignIn() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, user)

				// Verify oauth auth was created
				oauthAuth, err := testDB.Store.GetOauthUserByEmail(ctx, tt.email)
				if err != nil {
					t.Errorf("failed to get oauth user by email: %v", err)
					return
				}
				if oauthAuth.Email != tt.email {
					t.Errorf("Email = %v, want %v", oauthAuth.Email, tt.email)
				}
				if oauthAuth.ExternalID != tt.googleUserID {
					t.Errorf("ExternalID = %v, want %v", oauthAuth.ExternalID, tt.googleUserID)
				}
				if oauthAuth.AuthProvider != "google" {
					t.Errorf("AuthProvider = %v, want google", oauthAuth.AuthProvider)
				}
				expectedFullName := tt.firstName + " " + tt.lastName
				if oauthAuth.FullName != expectedFullName {
					t.Errorf("FullName = %v, want %v", oauthAuth.FullName, expectedFullName)
				}
			}
		})
	}
}

func TestStore_GetOauthUserByEmail(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) (uuid.UUID, string)
		email    string
		wantErr  bool
		validate func(t *testing.T, auth OauthAuth, expectedAuthID uuid.UUID, expectedExternalID string)
	}{
		{
			name: "get existing oauth user",
			setup: func(t *testing.T) (uuid.UUID, string) {
				t.Helper()
				user, _ := createTestUser(t, testDB, "OAuth", "User")
				authID := createTestUserAuth(t, testDB, user.ID, "oauth")
				externalID := "google_123456"
				createTestOAuthAuth(t, testDB, authID, externalID, "oauth@example.com", "OAuth User", "google")
				return authID, externalID
			},
			email:   "oauth@example.com",
			wantErr: false,
			validate: func(t *testing.T, auth OauthAuth, expectedAuthID uuid.UUID, expectedExternalID string) {
				t.Helper()
				if auth.Email != "oauth@example.com" {
					t.Errorf("Email = %v, want oauth@example.com", auth.Email)
				}
				if auth.ExternalID != expectedExternalID {
					t.Errorf("ExternalID = %v, want %v", auth.ExternalID, expectedExternalID)
				}
				if auth.AuthProvider != "google" {
					t.Errorf("AuthProvider = %v, want google", auth.AuthProvider)
				}
				if auth.FullName != "OAuth User" {
					t.Errorf("FullName = %v, want OAuth User", auth.FullName)
				}
				if auth.AuthID != expectedAuthID {
					t.Errorf("AuthID = %v, want %v", auth.AuthID, expectedAuthID)
				}
			},
		},
		{
			name: "oauth user does not exist",
			setup: func(t *testing.T) (uuid.UUID, string) {
				return uuid.Nil, ""
			},
			email:   "nonexistent@example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			expectedAuthID, expectedExternalID := tt.setup(t)

			auth, err := testDB.Store.GetOauthUserByEmail(ctx, tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetOauthUserByEmail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, auth, expectedAuthID, expectedExternalID)
			}
		})
	}
}

func TestStore_GetOauthUserByEmail_MultipleProviders(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	// Setup: Create users with different OAuth providers
	user1, _ := createTestUser(t, testDB, "Google", "User")
	authID1 := createTestUserAuth(t, testDB, user1.ID, "oauth")
	createTestOAuthAuth(t, testDB, authID1, "google_123", "user@gmail.com", "Google User", "google")

	user2, _ := createTestUser(t, testDB, "Apple", "User")
	authID2 := createTestUserAuth(t, testDB, user2.ID, "oauth")
	createTestOAuthAuth(t, testDB, authID2, "apple_456", "user@icloud.com", "Apple User", "apple")

	tests := []struct {
		name           string
		email          string
		wantProvider   string
		wantExternalID string
		wantErr        bool
	}{
		{
			name:           "get google oauth user",
			email:          "user@gmail.com",
			wantProvider:   "google",
			wantExternalID: "google_123",
			wantErr:        false,
		},
		{
			name:           "get apple oauth user",
			email:          "user@icloud.com",
			wantProvider:   "apple",
			wantExternalID: "apple_456",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := testDB.Store.GetOauthUserByEmail(ctx, tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetOauthUserByEmail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if auth.AuthProvider != tt.wantProvider {
					t.Errorf("AuthProvider = %v, want %v", auth.AuthProvider, tt.wantProvider)
				}
				if auth.ExternalID != tt.wantExternalID {
					t.Errorf("ExternalID = %v, want %v", auth.ExternalID, tt.wantExternalID)
				}
			}
		})
	}
}
