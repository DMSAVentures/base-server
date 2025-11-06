package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestStore_CheckIfEmailExists(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(t *testing.T)
		email     string
		wantExist bool
		wantErr   bool
	}{
		{
			name: "email exists in email_auth",
			setup: func(t *testing.T) {
				t.Helper()
				// Create user and email auth
				user, err := createTestUser(t, testDB, "John", "Doe")
				if err != nil {
					t.Fatalf("failed to create test user: %v", err)
				}
				authID := createTestUserAuth(t, testDB, user.ID, "email")
				createTestEmailAuth(t, testDB, authID, "john@example.com", "hashedpassword")
			},
			email:     "john@example.com",
			wantExist: true,
			wantErr:   false,
		},
		{
			name: "email exists in oauth_auth",
			setup: func(t *testing.T) {
				t.Helper()
				user, err := createTestUser(t, testDB, "Jane", "Smith")
				if err != nil {
					t.Fatalf("failed to create test user: %v", err)
				}
				authID := createTestUserAuth(t, testDB, user.ID, "oauth")
				createTestOAuthAuth(t, testDB, authID, "google123", "jane@example.com", "Jane Smith", "google")
			},
			email:     "jane@example.com",
			wantExist: true,
			wantErr:   false,
		},
		{
			name:      "email does not exist",
			setup:     func(t *testing.T) {},
			email:     "nonexistent@example.com",
			wantExist: false,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			tt.setup(t)

			exists, err := testDB.Store.CheckIfEmailExists(ctx, tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckIfEmailExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if exists != tt.wantExist {
				t.Errorf("CheckIfEmailExists() = %v, want %v", exists, tt.wantExist)
			}
		})
	}
}

func TestStore_CreateUserOnEmailSignup(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name           string
		firstName      string
		lastName       string
		email          string
		hashedPassword string
		wantErr        bool
		validate       func(t *testing.T, user User)
	}{
		{
			name:           "successful user creation",
			firstName:      "Alice",
			lastName:       "Johnson",
			email:          "alice@example.com",
			hashedPassword: "hashed_password_123",
			wantErr:        false,
			validate: func(t *testing.T, user User) {
				t.Helper()
				if user.ID == uuid.Nil {
					t.Error("expected user ID to be set")
				}
				if user.FirstName != "Alice" {
					t.Errorf("FirstName = %v, want Alice", user.FirstName)
				}
				if user.LastName != "Johnson" {
					t.Errorf("LastName = %v, want Johnson", user.LastName)
				}
			},
		},
		{
			name:           "user creation with minimal data",
			firstName:      "Bob",
			lastName:       "Smith",
			email:          "bob@example.com",
			hashedPassword: "password_hash",
			wantErr:        false,
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

			user, err := testDB.Store.CreateUserOnEmailSignup(ctx, tt.firstName, tt.lastName, tt.email, tt.hashedPassword)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateUserOnEmailSignup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, user)

				// Verify email auth was created
				emailAuth, err := testDB.Store.GetCredentialsByEmail(ctx, tt.email)
				if err != nil {
					t.Errorf("failed to get credentials by email: %v", err)
					return
				}
				if emailAuth.Email != tt.email {
					t.Errorf("Email = %v, want %v", emailAuth.Email, tt.email)
				}
				if emailAuth.HashedPassword != tt.hashedPassword {
					t.Errorf("HashedPassword = %v, want %v", emailAuth.HashedPassword, tt.hashedPassword)
				}
			}
		})
	}
}

func TestStore_GetCredentialsByEmail(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func(t *testing.T) uuid.UUID
		email   string
		wantErr bool
		validate func(t *testing.T, auth EmailAuth, expectedAuthID uuid.UUID)
	}{
		{
			name: "get existing credentials",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Test", "User")
				authID := createTestUserAuth(t, testDB, user.ID, "email")
				createTestEmailAuth(t, testDB, authID, "test@example.com", "hashed_pass")
				return authID
			},
			email:   "test@example.com",
			wantErr: false,
			validate: func(t *testing.T, auth EmailAuth, expectedAuthID uuid.UUID) {
				t.Helper()
				if auth.Email != "test@example.com" {
					t.Errorf("Email = %v, want test@example.com", auth.Email)
				}
				if auth.HashedPassword != "hashed_pass" {
					t.Errorf("HashedPassword = %v, want hashed_pass", auth.HashedPassword)
				}
				if auth.AuthID != expectedAuthID {
					t.Errorf("AuthID = %v, want %v", auth.AuthID, expectedAuthID)
				}
			},
		},
		{
			name:    "email does not exist",
			setup:   func(t *testing.T) uuid.UUID { return uuid.Nil },
			email:   "nonexistent@example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			expectedAuthID := tt.setup(t)

			auth, err := testDB.Store.GetCredentialsByEmail(ctx, tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCredentialsByEmail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, auth, expectedAuthID)
			}
		})
	}
}

func TestStore_GetUserByAuthID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) (uuid.UUID, User)
		wantErr  bool
		validate func(t *testing.T, got AuthenticatedUser, expectedUser User, expectedAuthID uuid.UUID)
	}{
		{
			name: "get user by valid auth ID",
			setup: func(t *testing.T) (uuid.UUID, User) {
				t.Helper()
				user, _ := createTestUser(t, testDB, "John", "Doe")
				authID := createTestUserAuth(t, testDB, user.ID, "email")
				return authID, user
			},
			wantErr: false,
			validate: func(t *testing.T, got AuthenticatedUser, expectedUser User, expectedAuthID uuid.UUID) {
				t.Helper()
				if got.UserID != expectedUser.ID {
					t.Errorf("UserID = %v, want %v", got.UserID, expectedUser.ID)
				}
				if got.FirstName != expectedUser.FirstName {
					t.Errorf("FirstName = %v, want %v", got.FirstName, expectedUser.FirstName)
				}
				if got.LastName != expectedUser.LastName {
					t.Errorf("LastName = %v, want %v", got.LastName, expectedUser.LastName)
				}
				if got.AuthID != expectedAuthID {
					t.Errorf("AuthID = %v, want %v", got.AuthID, expectedAuthID)
				}
				if got.AuthType != "email" {
					t.Errorf("AuthType = %v, want email", got.AuthType)
				}
			},
		},
		{
			name: "auth ID does not exist",
			setup: func(t *testing.T) (uuid.UUID, User) {
				return uuid.New(), User{}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			authID, expectedUser := tt.setup(t)

			got, err := testDB.Store.GetUserByAuthID(ctx, authID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserByAuthID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, got, expectedUser, authID)
			}
		})
	}
}

// Helper functions for creating test data

func createTestUser(t *testing.T, testDB *TestDB, firstName, lastName string) (User, error) {
	t.Helper()
	var user User
	query := `INSERT INTO users (first_name, last_name) VALUES ($1, $2) RETURNING id, first_name, last_name`
	err := testDB.GetDB().Get(&user, query, firstName, lastName)
	return user, err
}

func createTestUserAuth(t *testing.T, testDB *TestDB, userID uuid.UUID, authType string) uuid.UUID {
	t.Helper()
	var authID uuid.UUID
	query := `INSERT INTO user_auth (user_id, auth_type) VALUES ($1, $2) RETURNING id`
	err := testDB.GetDB().Get(&authID, query, userID, authType)
	if err != nil {
		t.Fatalf("failed to create user auth: %v", err)
	}
	return authID
}

func createTestEmailAuth(t *testing.T, testDB *TestDB, authID uuid.UUID, email, hashedPassword string) {
	t.Helper()
	query := `INSERT INTO email_auth (auth_id, email, hashed_password) VALUES ($1, $2, $3)`
	_, err := testDB.GetDB().Exec(query, authID, email, hashedPassword)
	if err != nil {
		t.Fatalf("failed to create email auth: %v", err)
	}
}

func createTestOAuthAuth(t *testing.T, testDB *TestDB, authID uuid.UUID, externalID, email, fullName, provider string) {
	t.Helper()
	query := `INSERT INTO oauth_auth (auth_id, external_id, email, full_name, auth_provider) VALUES ($1, $2, $3, $4, $5)`
	_, err := testDB.GetDB().Exec(query, authID, externalID, email, fullName, provider)
	if err != nil {
		t.Fatalf("failed to create oauth auth: %v", err)
	}
}
