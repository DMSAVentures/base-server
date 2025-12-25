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

func TestStore_GetCampaignByID_NotFound(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()

	// Try to get a non-existent campaign
	nonExistentID := uuid.New()
	_, err := testDB.Store.GetCampaignByID(ctx, nonExistentID)
	if err == nil {
		t.Error("Expected error when getting non-existent campaign")
	}
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestStore_GetCampaignBySlug_NotFound(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)

	// Try to get a campaign with non-existent slug
	_, err := testDB.Store.GetCampaignBySlug(ctx, account.ID, "non-existent-slug")
	if err == nil {
		t.Error("Expected error when getting campaign with non-existent slug")
	}
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestStore_UpdateCampaign_NotFound(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)

	// Try to update a non-existent campaign
	nonExistentID := uuid.New()
	newName := "Updated Name"
	_, err := testDB.Store.UpdateCampaign(ctx, account.ID, nonExistentID, UpdateCampaignParams{
		Name: &newName,
	})
	if err == nil {
		t.Error("Expected error when updating non-existent campaign")
	}
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestStore_UpdateCampaignStatus_NotFound(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)

	// Try to update status of a non-existent campaign
	nonExistentID := uuid.New()
	_, err := testDB.Store.UpdateCampaignStatus(ctx, account.ID, nonExistentID, "active")
	if err == nil {
		t.Error("Expected error when updating status of non-existent campaign")
	}
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestStore_DeleteCampaign_NotFound(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)

	// Try to delete a non-existent campaign
	nonExistentID := uuid.New()
	err := testDB.Store.DeleteCampaign(ctx, account.ID, nonExistentID)
	if err == nil {
		t.Error("Expected error when deleting non-existent campaign")
	}
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestStore_CampaignEmailSettings(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Email Settings Test", "email-settings-test")

	fromName := "Test Sender"
	fromEmail := "sender@example.com"
	replyTo := "reply@example.com"

	t.Run("create email settings", func(t *testing.T) {
		settings, err := testDB.Store.CreateCampaignEmailSettings(ctx, CreateCampaignEmailSettingsParams{
			CampaignID:           campaign.ID,
			FromName:             &fromName,
			FromEmail:            &fromEmail,
			ReplyTo:              &replyTo,
			VerificationRequired: true,
			SendWelcomeEmail:     true,
		})
		if err != nil {
			t.Fatalf("CreateCampaignEmailSettings() error = %v", err)
		}

		if settings.CampaignID != campaign.ID {
			t.Errorf("CampaignID = %v, want %v", settings.CampaignID, campaign.ID)
		}
		if settings.FromName == nil || *settings.FromName != fromName {
			t.Errorf("FromName = %v, want %v", settings.FromName, fromName)
		}
		if !settings.VerificationRequired {
			t.Error("Expected VerificationRequired to be true")
		}
		if !settings.SendWelcomeEmail {
			t.Error("Expected SendWelcomeEmail to be true")
		}
	})

	t.Run("get email settings", func(t *testing.T) {
		settings, err := testDB.Store.GetCampaignEmailSettings(ctx, campaign.ID)
		if err != nil {
			t.Fatalf("GetCampaignEmailSettings() error = %v", err)
		}

		if settings.FromEmail == nil || *settings.FromEmail != fromEmail {
			t.Errorf("FromEmail = %v, want %v", settings.FromEmail, fromEmail)
		}
	})

	t.Run("update email settings", func(t *testing.T) {
		newFromName := "Updated Sender"
		settings, err := testDB.Store.UpdateCampaignEmailSettings(ctx, campaign.ID, UpdateCampaignEmailSettingsParams{
			FromName:         &newFromName,
			SendWelcomeEmail: boolPtr(false),
		})
		if err != nil {
			t.Fatalf("UpdateCampaignEmailSettings() error = %v", err)
		}

		if settings.FromName == nil || *settings.FromName != newFromName {
			t.Errorf("FromName = %v, want %v", settings.FromName, newFromName)
		}
		if settings.SendWelcomeEmail {
			t.Error("Expected SendWelcomeEmail to be false after update")
		}
	})

	t.Run("delete email settings", func(t *testing.T) {
		err := testDB.Store.DeleteCampaignEmailSettings(ctx, campaign.ID)
		if err != nil {
			t.Fatalf("DeleteCampaignEmailSettings() error = %v", err)
		}

		_, err = testDB.Store.GetCampaignEmailSettings(ctx, campaign.ID)
		if err == nil {
			t.Error("Expected error when getting deleted email settings")
		}
	})
}

func TestStore_CampaignFormSettings(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Form Settings Test", "form-settings-test")

	captchaProvider := CaptchaProvider("turnstile")
	captchaSiteKey := "test-site-key"
	successTitle := "Thank you!"
	successMessage := "You've been added."

	t.Run("create form settings", func(t *testing.T) {
		settings, err := testDB.Store.CreateCampaignFormSettings(ctx, CreateCampaignFormSettingsParams{
			CampaignID:      campaign.ID,
			CaptchaEnabled:  true,
			CaptchaProvider: &captchaProvider,
			CaptchaSiteKey:  &captchaSiteKey,
			DoubleOptIn:     true,
			Design:          JSONB{"theme": "dark"},
			SuccessTitle:    &successTitle,
			SuccessMessage:  &successMessage,
		})
		if err != nil {
			t.Fatalf("CreateCampaignFormSettings() error = %v", err)
		}

		if settings.CampaignID != campaign.ID {
			t.Errorf("CampaignID = %v, want %v", settings.CampaignID, campaign.ID)
		}
		if !settings.CaptchaEnabled {
			t.Error("Expected CaptchaEnabled to be true")
		}
		if settings.CaptchaProvider == nil || *settings.CaptchaProvider != captchaProvider {
			t.Errorf("CaptchaProvider = %v, want %v", settings.CaptchaProvider, captchaProvider)
		}
	})

	t.Run("get form settings", func(t *testing.T) {
		settings, err := testDB.Store.GetCampaignFormSettings(ctx, campaign.ID)
		if err != nil {
			t.Fatalf("GetCampaignFormSettings() error = %v", err)
		}

		if !settings.DoubleOptIn {
			t.Error("Expected DoubleOptIn to be true")
		}
	})

	t.Run("upsert form settings", func(t *testing.T) {
		newCaptchaProvider := CaptchaProvider("recaptcha")
		settings, err := testDB.Store.UpsertCampaignFormSettings(ctx, CreateCampaignFormSettingsParams{
			CampaignID:      campaign.ID,
			CaptchaEnabled:  true,
			CaptchaProvider: &newCaptchaProvider,
			CaptchaSiteKey:  &captchaSiteKey,
			DoubleOptIn:     false,
			Design:          JSONB{"theme": "light"},
		})
		if err != nil {
			t.Fatalf("UpsertCampaignFormSettings() error = %v", err)
		}

		if settings.CaptchaProvider == nil || *settings.CaptchaProvider != newCaptchaProvider {
			t.Errorf("CaptchaProvider = %v, want %v", settings.CaptchaProvider, newCaptchaProvider)
		}
		if settings.DoubleOptIn {
			t.Error("Expected DoubleOptIn to be false after upsert")
		}
	})
}

func TestStore_CampaignReferralSettings(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Referral Settings Test", "referral-settings-test")

	t.Run("create referral settings", func(t *testing.T) {
		settings, err := testDB.Store.CreateCampaignReferralSettings(ctx, CreateCampaignReferralSettingsParams{
			CampaignID:              campaign.ID,
			Enabled:                 true,
			PointsPerReferral:       10,
			VerifiedOnly:            true,
			PositionsToJump:         5,
			ReferrerPositionsToJump: 2,
			SharingChannels:         []SharingChannel{"email", "twitter"},
		})
		if err != nil {
			t.Fatalf("CreateCampaignReferralSettings() error = %v", err)
		}

		if !settings.Enabled {
			t.Error("Expected Enabled to be true")
		}
		if settings.PointsPerReferral != 10 {
			t.Errorf("PointsPerReferral = %v, want 10", settings.PointsPerReferral)
		}
		if len(settings.SharingChannels) != 2 {
			t.Errorf("SharingChannels length = %v, want 2", len(settings.SharingChannels))
		}
	})

	t.Run("get referral settings", func(t *testing.T) {
		settings, err := testDB.Store.GetCampaignReferralSettings(ctx, campaign.ID)
		if err != nil {
			t.Fatalf("GetCampaignReferralSettings() error = %v", err)
		}

		if settings.PositionsToJump != 5 {
			t.Errorf("PositionsToJump = %v, want 5", settings.PositionsToJump)
		}
	})

	t.Run("update referral settings", func(t *testing.T) {
		pointsPerReferral := 25
		positionsToJump := 15
		settings, err := testDB.Store.UpdateCampaignReferralSettings(ctx, campaign.ID, UpdateCampaignReferralSettingsParams{
			PointsPerReferral: &pointsPerReferral,
			PositionsToJump:   &positionsToJump,
		})
		if err != nil {
			t.Fatalf("UpdateCampaignReferralSettings() error = %v", err)
		}

		if settings.PointsPerReferral != 25 {
			t.Errorf("PointsPerReferral = %v, want 25", settings.PointsPerReferral)
		}
		if settings.PositionsToJump != 15 {
			t.Errorf("PositionsToJump = %v, want 15", settings.PositionsToJump)
		}
	})
}

func TestStore_CampaignBrandingSettings(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Branding Settings Test", "branding-settings-test")

	logoURL := "https://example.com/logo.png"
	primaryColor := "#FF5733"
	fontFamily := "Inter"
	customDomain := "campaign.example.com"

	t.Run("create branding settings", func(t *testing.T) {
		settings, err := testDB.Store.CreateCampaignBrandingSettings(ctx, CreateCampaignBrandingSettingsParams{
			CampaignID:   campaign.ID,
			LogoURL:      &logoURL,
			PrimaryColor: &primaryColor,
			FontFamily:   &fontFamily,
			CustomDomain: &customDomain,
		})
		if err != nil {
			t.Fatalf("CreateCampaignBrandingSettings() error = %v", err)
		}

		if settings.LogoURL == nil || *settings.LogoURL != logoURL {
			t.Errorf("LogoURL = %v, want %v", settings.LogoURL, logoURL)
		}
		if settings.PrimaryColor == nil || *settings.PrimaryColor != primaryColor {
			t.Errorf("PrimaryColor = %v, want %v", settings.PrimaryColor, primaryColor)
		}
	})

	t.Run("upsert branding settings", func(t *testing.T) {
		newLogoURL := "https://example.com/new-logo.png"
		settings, err := testDB.Store.UpsertCampaignBrandingSettings(ctx, CreateCampaignBrandingSettingsParams{
			CampaignID:   campaign.ID,
			LogoURL:      &newLogoURL,
			PrimaryColor: &primaryColor,
		})
		if err != nil {
			t.Fatalf("UpsertCampaignBrandingSettings() error = %v", err)
		}

		if settings.LogoURL == nil || *settings.LogoURL != newLogoURL {
			t.Errorf("LogoURL = %v, want %v", settings.LogoURL, newLogoURL)
		}
	})
}

func TestStore_CampaignFormFields(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Form Fields Test", "form-fields-test")

	t.Run("create form field", func(t *testing.T) {
		field, err := testDB.Store.CreateCampaignFormField(ctx, CreateCampaignFormFieldParams{
			CampaignID:   campaign.ID,
			Name:         "email",
			FieldType:    FormFieldType("email"),
			Label:        "Email Address",
			Required:     true,
			DisplayOrder: 1,
		})
		if err != nil {
			t.Fatalf("CreateCampaignFormField() error = %v", err)
		}

		if field.Name != "email" {
			t.Errorf("Name = %v, want email", field.Name)
		}
		if field.FieldType != FormFieldType("email") {
			t.Errorf("FieldType = %v, want email", field.FieldType)
		}
	})

	t.Run("bulk create form fields", func(t *testing.T) {
		placeholder := "Enter your name"
		fields, err := testDB.Store.BulkCreateCampaignFormFields(ctx, []CreateCampaignFormFieldParams{
			{
				CampaignID:   campaign.ID,
				Name:         "name",
				FieldType:    FormFieldType("text"),
				Label:        "Full Name",
				Placeholder:  &placeholder,
				Required:     true,
				DisplayOrder: 2,
			},
			{
				CampaignID:   campaign.ID,
				Name:         "company",
				FieldType:    FormFieldType("text"),
				Label:        "Company",
				Required:     false,
				DisplayOrder: 3,
			},
		})
		if err != nil {
			t.Fatalf("BulkCreateCampaignFormFields() error = %v", err)
		}

		if len(fields) != 2 {
			t.Errorf("Created %d fields, want 2", len(fields))
		}
	})

	t.Run("get form fields", func(t *testing.T) {
		fields, err := testDB.Store.GetCampaignFormFields(ctx, campaign.ID)
		if err != nil {
			t.Fatalf("GetCampaignFormFields() error = %v", err)
		}

		if len(fields) != 3 {
			t.Errorf("Got %d fields, want 3", len(fields))
		}
	})

	t.Run("replace form fields", func(t *testing.T) {
		fields, err := testDB.Store.ReplaceCampaignFormFields(ctx, campaign.ID, []CreateCampaignFormFieldParams{
			{
				CampaignID:   campaign.ID,
				Name:         "new_email",
				FieldType:    FormFieldType("email"),
				Label:        "New Email Field",
				Required:     true,
				DisplayOrder: 1,
			},
		})
		if err != nil {
			t.Fatalf("ReplaceCampaignFormFields() error = %v", err)
		}

		if len(fields) != 1 {
			t.Errorf("Replaced with %d fields, want 1", len(fields))
		}
		if fields[0].Name != "new_email" {
			t.Errorf("Name = %v, want new_email", fields[0].Name)
		}
	})
}

func TestStore_CampaignShareMessages(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Share Messages Test", "share-messages-test")

	t.Run("create share message", func(t *testing.T) {
		msg, err := testDB.Store.CreateCampaignShareMessage(ctx, CreateCampaignShareMessageParams{
			CampaignID: campaign.ID,
			Channel:    SharingChannel("email"),
			Message:    "Check out this campaign!",
		})
		if err != nil {
			t.Fatalf("CreateCampaignShareMessage() error = %v", err)
		}

		if msg.Channel != SharingChannel("email") {
			t.Errorf("Channel = %v, want email", msg.Channel)
		}
	})

	t.Run("get share messages", func(t *testing.T) {
		messages, err := testDB.Store.GetCampaignShareMessages(ctx, campaign.ID)
		if err != nil {
			t.Fatalf("GetCampaignShareMessages() error = %v", err)
		}

		if len(messages) != 1 {
			t.Errorf("Got %d messages, want 1", len(messages))
		}
	})

	t.Run("get share message by channel", func(t *testing.T) {
		msg, err := testDB.Store.GetCampaignShareMessageByChannel(ctx, campaign.ID, SharingChannel("email"))
		if err != nil {
			t.Fatalf("GetCampaignShareMessageByChannel() error = %v", err)
		}

		if msg.Message != "Check out this campaign!" {
			t.Errorf("Message = %v, want 'Check out this campaign!'", msg.Message)
		}
	})

	t.Run("replace share messages", func(t *testing.T) {
		messages, err := testDB.Store.ReplaceCampaignShareMessages(ctx, campaign.ID, []CreateCampaignShareMessageParams{
			{CampaignID: campaign.ID, Channel: SharingChannel("twitter"), Message: "Tweet this!"},
			{CampaignID: campaign.ID, Channel: SharingChannel("facebook"), Message: "Share on FB!"},
		})
		if err != nil {
			t.Fatalf("ReplaceCampaignShareMessages() error = %v", err)
		}

		if len(messages) != 2 {
			t.Errorf("Replaced with %d messages, want 2", len(messages))
		}
	})
}

func TestStore_CampaignTrackingIntegrations(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Tracking Test", "tracking-test")

	trackingLabel := "signup_conversion"

	t.Run("create tracking integration", func(t *testing.T) {
		integration, err := testDB.Store.CreateCampaignTrackingIntegration(ctx, CreateCampaignTrackingIntegrationParams{
			CampaignID:      campaign.ID,
			IntegrationType: TrackingIntegrationType("google_analytics"),
			Enabled:         true,
			TrackingID:      "GA-12345678",
			TrackingLabel:   &trackingLabel,
		})
		if err != nil {
			t.Fatalf("CreateCampaignTrackingIntegration() error = %v", err)
		}

		if integration.IntegrationType != TrackingIntegrationType("google_analytics") {
			t.Errorf("IntegrationType = %v, want google_analytics", integration.IntegrationType)
		}
		if !integration.Enabled {
			t.Error("Expected Enabled to be true")
		}
	})

	t.Run("get tracking integrations", func(t *testing.T) {
		integrations, err := testDB.Store.GetCampaignTrackingIntegrations(ctx, campaign.ID)
		if err != nil {
			t.Fatalf("GetCampaignTrackingIntegrations() error = %v", err)
		}

		if len(integrations) != 1 {
			t.Errorf("Got %d integrations, want 1", len(integrations))
		}
	})

	t.Run("get enabled tracking integrations", func(t *testing.T) {
		integrations, err := testDB.Store.GetEnabledCampaignTrackingIntegrations(ctx, campaign.ID)
		if err != nil {
			t.Fatalf("GetEnabledCampaignTrackingIntegrations() error = %v", err)
		}

		if len(integrations) != 1 {
			t.Errorf("Got %d enabled integrations, want 1", len(integrations))
		}
	})

	t.Run("get tracking integration by type", func(t *testing.T) {
		integration, err := testDB.Store.GetCampaignTrackingIntegrationByType(ctx, campaign.ID, TrackingIntegrationType("google_analytics"))
		if err != nil {
			t.Fatalf("GetCampaignTrackingIntegrationByType() error = %v", err)
		}

		if integration.TrackingID != "GA-12345678" {
			t.Errorf("TrackingID = %v, want GA-12345678", integration.TrackingID)
		}
	})

	t.Run("replace tracking integrations", func(t *testing.T) {
		integrations, err := testDB.Store.ReplaceCampaignTrackingIntegrations(ctx, campaign.ID, []CreateCampaignTrackingIntegrationParams{
			{CampaignID: campaign.ID, IntegrationType: TrackingIntegrationType("meta_pixel"), Enabled: true, TrackingID: "123456789"},
			{CampaignID: campaign.ID, IntegrationType: TrackingIntegrationType("tiktok_pixel"), Enabled: false, TrackingID: "TT-999"},
		})
		if err != nil {
			t.Fatalf("ReplaceCampaignTrackingIntegrations() error = %v", err)
		}

		if len(integrations) != 2 {
			t.Errorf("Replaced with %d integrations, want 2", len(integrations))
		}

		// Verify only 1 is enabled
		enabled, _ := testDB.Store.GetEnabledCampaignTrackingIntegrations(ctx, campaign.ID)
		if len(enabled) != 1 {
			t.Errorf("Got %d enabled integrations, want 1", len(enabled))
		}
	})
}

func TestStore_CampaignWithSettings(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "With Settings Test", "with-settings-test")

	// Create all settings
	fromName := "Test"
	testDB.Store.CreateCampaignEmailSettings(ctx, CreateCampaignEmailSettingsParams{
		CampaignID:       campaign.ID,
		FromName:         &fromName,
		SendWelcomeEmail: true,
	})

	logoURL := "https://example.com/logo.png"
	testDB.Store.CreateCampaignBrandingSettings(ctx, CreateCampaignBrandingSettingsParams{
		CampaignID: campaign.ID,
		LogoURL:    &logoURL,
	})

	testDB.Store.CreateCampaignFormSettings(ctx, CreateCampaignFormSettingsParams{
		CampaignID:     campaign.ID,
		CaptchaEnabled: true,
		DoubleOptIn:    false,
		Design:         JSONB{"theme": "light"},
	})

	testDB.Store.CreateCampaignReferralSettings(ctx, CreateCampaignReferralSettingsParams{
		CampaignID:        campaign.ID,
		Enabled:           true,
		PointsPerReferral: 10,
		SharingChannels:   []SharingChannel{"email"},
	})

	testDB.Store.CreateCampaignFormField(ctx, CreateCampaignFormFieldParams{
		CampaignID:   campaign.ID,
		Name:         "email",
		FieldType:    FormFieldType("email"),
		Label:        "Email",
		Required:     true,
		DisplayOrder: 1,
	})

	testDB.Store.CreateCampaignShareMessage(ctx, CreateCampaignShareMessageParams{
		CampaignID: campaign.ID,
		Channel:    SharingChannel("email"),
		Message:    "Share via email",
	})

	testDB.Store.CreateCampaignTrackingIntegration(ctx, CreateCampaignTrackingIntegrationParams{
		CampaignID:      campaign.ID,
		IntegrationType: TrackingIntegrationType("google_analytics"),
		Enabled:         true,
		TrackingID:      "GA-123",
	})

	t.Run("get campaign with settings loads all settings", func(t *testing.T) {
		campaignWithSettings, err := testDB.Store.GetCampaignWithSettings(ctx, campaign.ID)
		if err != nil {
			t.Fatalf("GetCampaignWithSettings() error = %v", err)
		}

		if campaignWithSettings.EmailSettings == nil {
			t.Error("Expected EmailSettings to be loaded")
		}
		if campaignWithSettings.BrandingSettings == nil {
			t.Error("Expected BrandingSettings to be loaded")
		}
		if campaignWithSettings.FormSettings == nil {
			t.Error("Expected FormSettings to be loaded")
		}
		if campaignWithSettings.ReferralSettings == nil {
			t.Error("Expected ReferralSettings to be loaded")
		}
		if len(campaignWithSettings.FormFields) == 0 {
			t.Error("Expected FormFields to be loaded")
		}
		if len(campaignWithSettings.ShareMessages) == 0 {
			t.Error("Expected ShareMessages to be loaded")
		}
		if len(campaignWithSettings.TrackingIntegrations) == 0 {
			t.Error("Expected TrackingIntegrations to be loaded")
		}
	})

	t.Run("get campaign by slug with settings", func(t *testing.T) {
		campaignWithSettings, err := testDB.Store.GetCampaignBySlugWithSettings(ctx, account.ID, "with-settings-test")
		if err != nil {
			t.Fatalf("GetCampaignBySlugWithSettings() error = %v", err)
		}

		if campaignWithSettings.EmailSettings == nil {
			t.Error("Expected EmailSettings to be loaded")
		}
	})
}

func TestStore_ListCampaigns_Pagination(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()
	account := createTestAccount(t, testDB)

	// Create 5 campaigns
	for i := 1; i <= 5; i++ {
		createTestCampaign(t, testDB, account.ID, "Campaign "+string(rune('0'+i)), "campaign-"+string(rune('0'+i)))
	}

	t.Run("first page", func(t *testing.T) {
		result, err := testDB.Store.ListCampaigns(ctx, ListCampaignsParams{
			AccountID: account.ID,
			Page:      1,
			Limit:     2,
		})
		if err != nil {
			t.Fatalf("ListCampaigns() error = %v", err)
		}

		if len(result.Campaigns) != 2 {
			t.Errorf("Got %d campaigns, want 2", len(result.Campaigns))
		}
		if result.TotalCount != 5 {
			t.Errorf("TotalCount = %v, want 5", result.TotalCount)
		}
		if result.TotalPages != 3 {
			t.Errorf("TotalPages = %v, want 3", result.TotalPages)
		}
	})

	t.Run("last page with fewer items", func(t *testing.T) {
		result, err := testDB.Store.ListCampaigns(ctx, ListCampaignsParams{
			AccountID: account.ID,
			Page:      3,
			Limit:     2,
		})
		if err != nil {
			t.Fatalf("ListCampaigns() error = %v", err)
		}

		if len(result.Campaigns) != 1 {
			t.Errorf("Got %d campaigns, want 1", len(result.Campaigns))
		}
	})
}

func TestStore_ListCampaigns_Filters(t *testing.T) {
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

	t.Run("filter by status", func(t *testing.T) {
		status := "active"
		result, err := testDB.Store.ListCampaigns(ctx, ListCampaignsParams{
			AccountID: account.ID,
			Page:      1,
			Limit:     10,
			Status:    &status,
		})
		if err != nil {
			t.Fatalf("ListCampaigns() error = %v", err)
		}

		if len(result.Campaigns) != 1 {
			t.Errorf("Got %d campaigns, want 1", len(result.Campaigns))
		}
		if result.Campaigns[0].ID != campaign2.ID {
			t.Error("Expected active campaign to be returned")
		}
	})

	t.Run("filter by draft status", func(t *testing.T) {
		status := "draft"
		result, err := testDB.Store.ListCampaigns(ctx, ListCampaignsParams{
			AccountID: account.ID,
			Page:      1,
			Limit:     10,
			Status:    &status,
		})
		if err != nil {
			t.Fatalf("ListCampaigns() error = %v", err)
		}

		if len(result.Campaigns) != 1 {
			t.Errorf("Got %d campaigns, want 1", len(result.Campaigns))
		}
		if result.Campaigns[0].ID != campaign1.ID {
			t.Error("Expected draft campaign to be returned")
		}
	})
}

// Helper function for bool pointers
func boolPtr(b bool) *bool {
	return &b
}
