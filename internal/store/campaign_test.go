package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// Helper to create a test account
func createTestAccount(t *testing.T, testDB *TestDB) Account {
	t.Helper()
	user, err := createTestUser(t, testDB, "Test", "User")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	account, err := testDB.Store.CreateAccount(context.Background(), CreateAccountParams{
		Name:        "Test Account",
		Slug:        "test-account-" + uuid.New().String()[:8],
		OwnerUserID: user.ID,
		Plan:        "pro",
	})
	if err != nil {
		t.Fatalf("failed to create test account: %v", err)
	}
	return account
}

// Helper to create a test campaign
func createTestCampaign(t *testing.T, testDB *TestDB, accountID uuid.UUID, name, slug string) Campaign {
	t.Helper()
	campaign, err := testDB.Store.CreateCampaign(context.Background(), CreateCampaignParams{
		AccountID: accountID,
		Name:      name,
		Slug:      slug,
		Type:      "waitlist",
	})
	if err != nil {
		t.Fatalf("failed to create test campaign: %v", err)
	}
	return campaign
}

// Helper to create a test waitlist user
func createTestWaitlistUser(t *testing.T, testDB *TestDB, campaignID uuid.UUID, email string) WaitlistUser {
	t.Helper()
	user, err := testDB.Store.CreateWaitlistUser(context.Background(), CreateWaitlistUserParams{
		CampaignID:       campaignID,
		Email:            email,
		ReferralCode:     "TEST" + uuid.New().String()[:6],
		Position:         1,
		OriginalPosition: 1,
		TermsAccepted:    true,
	})
	if err != nil {
		t.Fatalf("failed to create test waitlist user: %v", err)
	}
	return user
}

func TestStore_CreateCampaign(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) CreateCampaignParams
		wantErr  bool
		validate func(t *testing.T, campaign Campaign, params CreateCampaignParams)
	}{
		{
			name: "create campaign with all required fields",
			setup: func(t *testing.T) CreateCampaignParams {
				t.Helper()
				account := createTestAccount(t, testDB)
				return CreateCampaignParams{
					AccountID: account.ID,
					Name:      "Test Campaign",
					Slug:      "test-campaign",
					Type:      "waitlist",
				}
			},
			wantErr: false,
			validate: func(t *testing.T, campaign Campaign, params CreateCampaignParams) {
				t.Helper()
				if campaign.ID == uuid.Nil {
					t.Error("expected campaign ID to be set")
				}
				if campaign.Name != params.Name {
					t.Errorf("Name = %v, want %v", campaign.Name, params.Name)
				}
				if campaign.Slug != params.Slug {
					t.Errorf("Slug = %v, want %v", campaign.Slug, params.Slug)
				}
				if campaign.Type != params.Type {
					t.Errorf("Type = %v, want %v", campaign.Type, params.Type)
				}
				if campaign.Status != "draft" {
					t.Errorf("Status = %v, want draft", campaign.Status)
				}
				if campaign.TotalSignups != 0 {
					t.Errorf("TotalSignups = %v, want 0", campaign.TotalSignups)
				}
				if campaign.TotalVerified != 0 {
					t.Errorf("TotalVerified = %v, want 0", campaign.TotalVerified)
				}
			},
		},
		{
			name: "create campaign with optional fields",
			setup: func(t *testing.T) CreateCampaignParams {
				t.Helper()
				account := createTestAccount(t, testDB)
				description := "Test Description"
				maxSignups := 1000
				privacyURL := "https://example.com/privacy"
				termsURL := "https://example.com/terms"
				return CreateCampaignParams{
					AccountID:        account.ID,
					Name:             "Full Campaign",
					Slug:             "full-campaign",
					Description:      &description,
					Type:             "referral",
					MaxSignups:       &maxSignups,
					PrivacyPolicyURL: &privacyURL,
					TermsURL:         &termsURL,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, campaign Campaign, params CreateCampaignParams) {
				t.Helper()
				if campaign.Description == nil || *campaign.Description != *params.Description {
					t.Errorf("Description = %v, want %v", campaign.Description, params.Description)
				}
				if campaign.MaxSignups == nil || *campaign.MaxSignups != *params.MaxSignups {
					t.Errorf("MaxSignups = %v, want %v", campaign.MaxSignups, params.MaxSignups)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			params := tt.setup(t)

			campaign, err := testDB.Store.CreateCampaign(ctx, params)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateCampaign() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, campaign, params)
			}
		})
	}
}

func TestStore_GetCampaignByID_WithCounters(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Counter Test", "counter-test")

	// Initially should have 0 signups
	retrieved, err := testDB.Store.GetCampaignByID(ctx, campaign.ID)
	if err != nil {
		t.Fatalf("GetCampaignByID() error = %v", err)
	}
	if retrieved.TotalSignups != 0 {
		t.Errorf("TotalSignups = %v, want 0", retrieved.TotalSignups)
	}
	if retrieved.TotalVerified != 0 {
		t.Errorf("TotalVerified = %v, want 0", retrieved.TotalVerified)
	}

	// Create 3 waitlist users
	createTestWaitlistUser(t, testDB, campaign.ID, "user1@example.com")
	createTestWaitlistUser(t, testDB, campaign.ID, "user2@example.com")
	createTestWaitlistUser(t, testDB, campaign.ID, "user3@example.com")

	// Should now have 3 signups
	retrieved, err = testDB.Store.GetCampaignByID(ctx, campaign.ID)
	if err != nil {
		t.Fatalf("GetCampaignByID() error = %v", err)
	}
	if retrieved.TotalSignups != 3 {
		t.Errorf("TotalSignups = %v, want 3", retrieved.TotalSignups)
	}

	// Verify a specific user as verified
	user1, _ := testDB.Store.GetWaitlistUserByEmail(ctx, campaign.ID, "user1@example.com")
	err = testDB.Store.VerifyWaitlistUserEmail(ctx, user1.ID)
	if err != nil {
		t.Fatalf("VerifyWaitlistUserEmail() error = %v", err)
	}

	// Should now have 1 verified
	retrieved, err = testDB.Store.GetCampaignByID(ctx, campaign.ID)
	if err != nil {
		t.Fatalf("GetCampaignByID() error = %v", err)
	}
	if retrieved.TotalVerified != 1 {
		t.Errorf("TotalVerified = %v, want 1", retrieved.TotalVerified)
	}
	if retrieved.TotalSignups != 3 {
		t.Errorf("TotalSignups = %v, want 3 (should remain same)", retrieved.TotalSignups)
	}
}

func TestStore_ListCampaigns_WithCounters(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)

	// Create 2 campaigns
	campaign1 := createTestCampaign(t, testDB, account.ID, "Campaign 1", "campaign-1")
	campaign2 := createTestCampaign(t, testDB, account.ID, "Campaign 2", "campaign-2")

	// Add users to campaign1
	createTestWaitlistUser(t, testDB, campaign1.ID, "c1-user1@example.com")
	createTestWaitlistUser(t, testDB, campaign1.ID, "c1-user2@example.com")

	// Add users to campaign2
	createTestWaitlistUser(t, testDB, campaign2.ID, "c2-user1@example.com")

	// List campaigns
	result, err := testDB.Store.ListCampaigns(ctx, ListCampaignsParams{
		AccountID: account.ID,
		Page:      1,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("ListCampaigns() error = %v", err)
	}

	if len(result.Campaigns) != 2 {
		t.Fatalf("got %d campaigns, want 2", len(result.Campaigns))
	}

	// Find campaigns and verify counts
	var c1Found, c2Found bool
	for _, c := range result.Campaigns {
		if c.ID == campaign1.ID {
			c1Found = true
			if c.TotalSignups != 2 {
				t.Errorf("Campaign1 TotalSignups = %v, want 2", c.TotalSignups)
			}
		}
		if c.ID == campaign2.ID {
			c2Found = true
			if c.TotalSignups != 1 {
				t.Errorf("Campaign2 TotalSignups = %v, want 1", c.TotalSignups)
			}
		}
	}

	if !c1Found || !c2Found {
		t.Error("Not all campaigns found in list")
	}
}

func TestStore_GetCampaignBySlug_WithCounters(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Slug Test", "slug-test")

	// Add 2 users
	createTestWaitlistUser(t, testDB, campaign.ID, "slug1@example.com")
	createTestWaitlistUser(t, testDB, campaign.ID, "slug2@example.com")

	// Get by slug
	retrieved, err := testDB.Store.GetCampaignBySlug(ctx, account.ID, "slug-test")
	if err != nil {
		t.Fatalf("GetCampaignBySlug() error = %v", err)
	}

	if retrieved.TotalSignups != 2 {
		t.Errorf("TotalSignups = %v, want 2", retrieved.TotalSignups)
	}
}

func TestStore_UpdateCampaign(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Original Name", "original-slug")

	newName := "Updated Name"
	newDescription := "Updated Description"
	params := UpdateCampaignParams{
		Name:        &newName,
		Description: &newDescription,
	}

	updated, err := testDB.Store.UpdateCampaign(ctx, account.ID, campaign.ID, params)
	if err != nil {
		t.Fatalf("UpdateCampaign() error = %v", err)
	}

	if updated.Name != newName {
		t.Errorf("Name = %v, want %v", updated.Name, newName)
	}
	if updated.Description == nil || *updated.Description != newDescription {
		t.Errorf("Description = %v, want %v", updated.Description, newDescription)
	}
}

func TestStore_UpdateCampaignStatus(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Status Test", "status-test")

	// Initially should be draft
	if campaign.Status != "draft" {
		t.Errorf("Initial status = %v, want draft", campaign.Status)
	}

	// Update to active
	updated, err := testDB.Store.UpdateCampaignStatus(ctx, account.ID, campaign.ID, "active")
	if err != nil {
		t.Fatalf("UpdateCampaignStatus() error = %v", err)
	}

	if updated.Status != "active" {
		t.Errorf("Status = %v, want active", updated.Status)
	}
}

func TestStore_DeleteCampaign(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Delete Test", "delete-test")

	// Delete the campaign
	err := testDB.Store.DeleteCampaign(ctx, account.ID, campaign.ID)
	if err != nil {
		t.Fatalf("DeleteCampaign() error = %v", err)
	}

	// Should not be found (soft delete)
	_, err = testDB.Store.GetCampaignByID(ctx, campaign.ID)
	if err == nil {
		t.Error("Expected error when getting deleted campaign")
	}
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestStore_GetCampaignsByAccountID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account1 := createTestAccount(t, testDB)
	account2 := createTestAccount(t, testDB)

	// Create campaigns for account1
	createTestCampaign(t, testDB, account1.ID, "Account1 Campaign 1", "a1-c1")
	createTestCampaign(t, testDB, account1.ID, "Account1 Campaign 2", "a1-c2")

	// Create campaign for account2
	createTestCampaign(t, testDB, account2.ID, "Account2 Campaign 1", "a2-c1")

	// Get campaigns for account1
	campaigns, err := testDB.Store.GetCampaignsByAccountID(ctx, account1.ID)
	if err != nil {
		t.Fatalf("GetCampaignsByAccountID() error = %v", err)
	}

	if len(campaigns) != 2 {
		t.Errorf("got %d campaigns for account1, want 2", len(campaigns))
	}

	// Verify all campaigns belong to account1
	for _, c := range campaigns {
		if c.AccountID != account1.ID {
			t.Errorf("Campaign %s has wrong AccountID = %v, want %v", c.ID, c.AccountID, account1.ID)
		}
	}
}

func TestStore_GetCampaignsByStatus(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)

	// Create campaigns with different statuses
	campaign1 := createTestCampaign(t, testDB, account.ID, "Draft Campaign", "draft-campaign")
	campaign2 := createTestCampaign(t, testDB, account.ID, "Active Campaign", "active-campaign")

	// Update one to active
	testDB.Store.UpdateCampaignStatus(ctx, account.ID, campaign2.ID, "active")

	// Get draft campaigns
	draftCampaigns, err := testDB.Store.GetCampaignsByStatus(ctx, account.ID, "draft")
	if err != nil {
		t.Fatalf("GetCampaignsByStatus() error = %v", err)
	}

	if len(draftCampaigns) != 1 {
		t.Errorf("got %d draft campaigns, want 1", len(draftCampaigns))
	}

	if draftCampaigns[0].ID != campaign1.ID {
		t.Error("Wrong campaign returned for draft status")
	}

	// Get active campaigns
	activeCampaigns, err := testDB.Store.GetCampaignsByStatus(ctx, account.ID, "active")
	if err != nil {
		t.Fatalf("GetCampaignsByStatus() error = %v", err)
	}

	if len(activeCampaigns) != 1 {
		t.Errorf("got %d active campaigns, want 1", len(activeCampaigns))
	}

	if activeCampaigns[0].ID != campaign2.ID {
		t.Error("Wrong campaign returned for active status")
	}
}

func TestStore_CountersWithSoftDeletedUsers(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Soft Delete Test", "soft-delete-test")

	// Create 3 users
	user1 := createTestWaitlistUser(t, testDB, campaign.ID, "delete1@example.com")
	createTestWaitlistUser(t, testDB, campaign.ID, "delete2@example.com")
	createTestWaitlistUser(t, testDB, campaign.ID, "delete3@example.com")

	// Should have 3 signups
	retrieved, _ := testDB.Store.GetCampaignByID(ctx, campaign.ID)
	if retrieved.TotalSignups != 3 {
		t.Errorf("TotalSignups = %v, want 3", retrieved.TotalSignups)
	}

	// Soft delete one user
	err := testDB.Store.DeleteWaitlistUser(ctx, user1.ID)
	if err != nil {
		t.Fatalf("DeleteWaitlistUser() error = %v", err)
	}

	// Should now have 2 signups (soft deleted users not counted)
	retrieved, _ = testDB.Store.GetCampaignByID(ctx, campaign.ID)
	if retrieved.TotalSignups != 2 {
		t.Errorf("TotalSignups after soft delete = %v, want 2", retrieved.TotalSignups)
	}
}
