package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestStore_CreateAccount(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) CreateAccountParams
		wantErr  bool
		validate func(t *testing.T, account Account, params CreateAccountParams)
	}{
		{
			name: "create account with all fields",
			setup: func(t *testing.T) CreateAccountParams {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Owner", "User")
				stripeID := "cus_account123"
				return CreateAccountParams{
					Name:             "Test Company",
					Slug:             "test-company",
					OwnerUserID:      user.ID,
					Plan:             "pro",
					StripeCustomerID: &stripeID,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, account Account, params CreateAccountParams) {
				t.Helper()
				if account.ID == uuid.Nil {
					t.Error("expected account ID to be set")
				}
				if account.Name != params.Name {
					t.Errorf("Name = %v, want %v", account.Name, params.Name)
				}
				if account.Slug != params.Slug {
					t.Errorf("Slug = %v, want %v", account.Slug, params.Slug)
				}
				if account.OwnerUserID != params.OwnerUserID {
					t.Errorf("OwnerUserID = %v, want %v", account.OwnerUserID, params.OwnerUserID)
				}
				if account.Plan != params.Plan {
					t.Errorf("Plan = %v, want %v", account.Plan, params.Plan)
				}
				if account.StripeCustomerID != nil && *account.StripeCustomerID != *params.StripeCustomerID {
					t.Errorf("StripeCustomerID = %v, want %v", *account.StripeCustomerID, *params.StripeCustomerID)
				}
			},
		},
		{
			name: "create account without stripe customer ID",
			setup: func(t *testing.T) CreateAccountParams {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Another", "Owner")
				return CreateAccountParams{
					Name:             "Startup Inc",
					Slug:             "startup-inc",
					OwnerUserID:      user.ID,
					Plan:             "free",
					StripeCustomerID: nil,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, account Account, params CreateAccountParams) {
				t.Helper()
				if account.ID == uuid.Nil {
					t.Error("expected account ID to be set")
				}
				if account.Name != params.Name {
					t.Errorf("Name = %v, want %v", account.Name, params.Name)
				}
				if account.StripeCustomerID != nil {
					t.Errorf("StripeCustomerID should be nil, got %v", *account.StripeCustomerID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			params := tt.setup(t)

			account, err := testDB.Store.CreateAccount(ctx, params)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateAccount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, account, params)
			}
		})
	}
}

func TestStore_GetAccountByID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) uuid.UUID
		wantErr  bool
		validate func(t *testing.T, account Account)
	}{
		{
			name: "get existing account",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Owner", "User")
				account, _ := testDB.Store.CreateAccount(ctx, CreateAccountParams{
					Name:        "Test Account",
					Slug:        "test-account",
					OwnerUserID: user.ID,
					Plan:        "pro",
				})
				return account.ID
			},
			wantErr: false,
			validate: func(t *testing.T, account Account) {
				t.Helper()
				if account.Name != "Test Account" {
					t.Errorf("Name = %v, want Test Account", account.Name)
				}
				if account.Slug != "test-account" {
					t.Errorf("Slug = %v, want test-account", account.Slug)
				}
			},
		},
		{
			name: "account does not exist",
			setup: func(t *testing.T) uuid.UUID {
				return uuid.New()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			accountID := tt.setup(t)

			account, err := testDB.Store.GetAccountByID(ctx, accountID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAccountByID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if account.ID != accountID {
					t.Errorf("ID = %v, want %v", account.ID, accountID)
				}
				if tt.validate != nil {
					tt.validate(t, account)
				}
			}
		})
	}
}

func TestStore_GetAccountBySlug(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) string
		slug     string
		wantErr  bool
		validate func(t *testing.T, account Account)
	}{
		{
			name: "get account by valid slug",
			setup: func(t *testing.T) string {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Owner", "User")
				testDB.Store.CreateAccount(ctx, CreateAccountParams{
					Name:        "Acme Corp",
					Slug:        "acme-corp",
					OwnerUserID: user.ID,
					Plan:        "enterprise",
				})
				return "acme-corp"
			},
			wantErr: false,
			validate: func(t *testing.T, account Account) {
				t.Helper()
				if account.Name != "Acme Corp" {
					t.Errorf("Name = %v, want Acme Corp", account.Name)
				}
				if account.Plan != "enterprise" {
					t.Errorf("Plan = %v, want enterprise", account.Plan)
				}
			},
		},
		{
			name: "slug does not exist",
			setup: func(t *testing.T) string {
				return "nonexistent-slug"
			},
			slug:    "nonexistent-slug",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			slug := tt.setup(t)
			if tt.slug != "" {
				slug = tt.slug
			}

			account, err := testDB.Store.GetAccountBySlug(ctx, slug)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAccountBySlug() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if account.Slug != slug {
					t.Errorf("Slug = %v, want %v", account.Slug, slug)
				}
				if tt.validate != nil {
					tt.validate(t, account)
				}
			}
		})
	}
}

func TestStore_GetAccountsByOwnerUserID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) uuid.UUID
		wantErr  bool
		wantLen  int
		validate func(t *testing.T, accounts []Account)
	}{
		{
			name: "get accounts for user with multiple accounts",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Multi", "Owner")
				testDB.Store.CreateAccount(ctx, CreateAccountParams{
					Name:        "Account 1",
					Slug:        "account-1",
					OwnerUserID: user.ID,
					Plan:        "free",
				})
				testDB.Store.CreateAccount(ctx, CreateAccountParams{
					Name:        "Account 2",
					Slug:        "account-2",
					OwnerUserID: user.ID,
					Plan:        "pro",
				})
				return user.ID
			},
			wantErr: false,
			wantLen: 2,
			validate: func(t *testing.T, accounts []Account) {
				t.Helper()
				if len(accounts) != 2 {
					t.Errorf("got %d accounts, want 2", len(accounts))
				}
			},
		},
		{
			name: "get accounts for user with no accounts",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "No", "Accounts")
				return user.ID
			},
			wantErr: false,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			userID := tt.setup(t)

			accounts, err := testDB.Store.GetAccountsByOwnerUserID(ctx, userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAccountsByOwnerUserID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(accounts) != tt.wantLen {
					t.Errorf("got %d accounts, want %d", len(accounts), tt.wantLen)
				}
				if tt.validate != nil {
					tt.validate(t, accounts)
				}
			}
		})
	}
}

func TestStore_UpdateAccount(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) (uuid.UUID, UpdateAccountParams)
		wantErr  bool
		validate func(t *testing.T, updated Account, params UpdateAccountParams)
	}{
		{
			name: "update account name",
			setup: func(t *testing.T) (uuid.UUID, UpdateAccountParams) {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Owner", "User")
				account, _ := testDB.Store.CreateAccount(ctx, CreateAccountParams{
					Name:        "Old Name",
					Slug:        "old-name",
					OwnerUserID: user.ID,
					Plan:        "free",
				})
				newName := "New Name"
				return account.ID, UpdateAccountParams{
					Name: &newName,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, updated Account, params UpdateAccountParams) {
				t.Helper()
				if params.Name != nil && updated.Name != *params.Name {
					t.Errorf("Name = %v, want %v", updated.Name, *params.Name)
				}
			},
		},
		{
			name: "update account plan and status",
			setup: func(t *testing.T) (uuid.UUID, UpdateAccountParams) {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Owner", "User")
				account, _ := testDB.Store.CreateAccount(ctx, CreateAccountParams{
					Name:        "Test Account",
					Slug:        "test",
					OwnerUserID: user.ID,
					Plan:        "free",
				})
				newPlan := "pro"
				newStatus := "active"
				return account.ID, UpdateAccountParams{
					Plan:   &newPlan,
					Status: &newStatus,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, updated Account, params UpdateAccountParams) {
				t.Helper()
				if params.Plan != nil && updated.Plan != *params.Plan {
					t.Errorf("Plan = %v, want %v", updated.Plan, *params.Plan)
				}
				if params.Status != nil && updated.Status != *params.Status {
					t.Errorf("Status = %v, want %v", updated.Status, *params.Status)
				}
			},
		},
		{
			name: "update non-existent account",
			setup: func(t *testing.T) (uuid.UUID, UpdateAccountParams) {
				name := "Updated Name"
				return uuid.New(), UpdateAccountParams{
					Name: &name,
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			accountID, params := tt.setup(t)

			updated, err := testDB.Store.UpdateAccount(ctx, accountID, params)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateAccount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, updated, params)
			}
		})
	}
}

func TestStore_DeleteAccount(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func(t *testing.T) uuid.UUID
		wantErr bool
	}{
		{
			name: "delete existing account",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Owner", "User")
				account, _ := testDB.Store.CreateAccount(ctx, CreateAccountParams{
					Name:        "To Delete",
					Slug:        "to-delete",
					OwnerUserID: user.ID,
					Plan:        "free",
				})
				return account.ID
			},
			wantErr: false,
		},
		{
			name: "delete non-existent account",
			setup: func(t *testing.T) uuid.UUID {
				return uuid.New()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			accountID := tt.setup(t)

			err := testDB.Store.DeleteAccount(ctx, accountID)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteAccount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify account is soft-deleted
				_, err := testDB.Store.GetAccountByID(ctx, accountID)
				if !errors.Is(err, ErrNotFound) {
					t.Error("expected account to be soft-deleted")
				}
			}
		})
	}
}

func TestStore_UpdateAccountStripeCustomerID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) uuid.UUID
		stripeID string
		wantErr  bool
	}{
		{
			name: "update stripe customer ID",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Owner", "User")
				account, _ := testDB.Store.CreateAccount(ctx, CreateAccountParams{
					Name:        "Test Account",
					Slug:        "test",
					OwnerUserID: user.ID,
					Plan:        "free",
				})
				return account.ID
			},
			stripeID: "cus_new_stripe_id",
			wantErr:  false,
		},
		{
			name: "update non-existent account",
			setup: func(t *testing.T) uuid.UUID {
				return uuid.New()
			},
			stripeID: "cus_fail",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			accountID := tt.setup(t)

			err := testDB.Store.UpdateAccountStripeCustomerID(ctx, accountID, tt.stripeID)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateAccountStripeCustomerID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the update
				account, err := testDB.Store.GetAccountByID(ctx, accountID)
				if err != nil {
					t.Errorf("failed to get account: %v", err)
					return
				}
				if account.StripeCustomerID == nil || *account.StripeCustomerID != tt.stripeID {
					t.Errorf("StripeCustomerID = %v, want %v", account.StripeCustomerID, tt.stripeID)
				}
			}
		})
	}
}

// Team Member Tests

func TestStore_CreateTeamMember(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) CreateTeamMemberParams
		wantErr  bool
		validate func(t *testing.T, member TeamMember, params CreateTeamMemberParams)
	}{
		{
			name: "add team member",
			setup: func(t *testing.T) CreateTeamMemberParams {
				t.Helper()
				owner, _ := createTestUser(t, testDB, "Owner", "User")
				member, _ := createTestUser(t, testDB, "Team", "Member")
				account, _ := testDB.Store.CreateAccount(ctx, CreateAccountParams{
					Name:        "Test Account",
					Slug:        "test",
					OwnerUserID: owner.ID,
					Plan:        "pro",
				})
				return CreateTeamMemberParams{
					AccountID:   account.ID,
					UserID:      member.ID,
					Role:        "admin",
					Permissions: JSONB{"read": true, "write": true},
					InvitedBy:   &owner.ID,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, member TeamMember, params CreateTeamMemberParams) {
				t.Helper()
				if member.ID == uuid.Nil {
					t.Error("expected team member ID to be set")
				}
				if member.AccountID != params.AccountID {
					t.Errorf("AccountID = %v, want %v", member.AccountID, params.AccountID)
				}
				if member.UserID != params.UserID {
					t.Errorf("UserID = %v, want %v", member.UserID, params.UserID)
				}
				if member.Role != params.Role {
					t.Errorf("Role = %v, want %v", member.Role, params.Role)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			params := tt.setup(t)

			member, err := testDB.Store.CreateTeamMember(ctx, params)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateTeamMember() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, member, params)
			}
		})
	}
}

func TestStore_GetTeamMembersByAccountID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func(t *testing.T) uuid.UUID
		wantLen int
		wantErr bool
	}{
		{
			name: "get team members for account",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				owner, _ := createTestUser(t, testDB, "Owner", "User")
				member1, _ := createTestUser(t, testDB, "Member", "One")
				member2, _ := createTestUser(t, testDB, "Member", "Two")
				account, _ := testDB.Store.CreateAccount(ctx, CreateAccountParams{
					Name:        "Test Account",
					Slug:        "test",
					OwnerUserID: owner.ID,
					Plan:        "pro",
				})
				testDB.Store.CreateTeamMember(ctx, CreateTeamMemberParams{
					AccountID:   account.ID,
					UserID:      member1.ID,
					Role:        "admin",
					Permissions: JSONB{},
				})
				testDB.Store.CreateTeamMember(ctx, CreateTeamMemberParams{
					AccountID:   account.ID,
					UserID:      member2.ID,
					Role:        "viewer",
					Permissions: JSONB{},
				})
				return account.ID
			},
			wantLen: 2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			accountID := tt.setup(t)

			members, err := testDB.Store.GetTeamMembersByAccountID(ctx, accountID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTeamMembersByAccountID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(members) != tt.wantLen {
				t.Errorf("got %d members, want %d", len(members), tt.wantLen)
			}
		})
	}
}

func TestStore_DeleteTeamMember(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func(t *testing.T) (uuid.UUID, uuid.UUID)
		wantErr bool
	}{
		{
			name: "delete team member",
			setup: func(t *testing.T) (uuid.UUID, uuid.UUID) {
				t.Helper()
				owner, _ := createTestUser(t, testDB, "Owner", "User")
				member, _ := createTestUser(t, testDB, "Team", "Member")
				account, _ := testDB.Store.CreateAccount(ctx, CreateAccountParams{
					Name:        "Test Account",
					Slug:        "test",
					OwnerUserID: owner.ID,
					Plan:        "pro",
				})
				testDB.Store.CreateTeamMember(ctx, CreateTeamMemberParams{
					AccountID:   account.ID,
					UserID:      member.ID,
					Role:        "editor",
					Permissions: JSONB{},
				})
				return account.ID, member.ID
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			accountID, userID := tt.setup(t)

			err := testDB.Store.DeleteTeamMember(ctx, accountID, userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteTeamMember() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify member is deleted
				_, err := testDB.Store.GetTeamMemberByAccountAndUserID(ctx, accountID, userID)
				if !errors.Is(err, ErrNotFound) {
					t.Error("expected team member to be deleted")
				}
			}
		})
	}
}
