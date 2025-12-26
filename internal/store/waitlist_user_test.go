package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestStore_CreateWaitlistUser(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) CreateWaitlistUserParams
		wantErr  bool
		validate func(t *testing.T, user WaitlistUser, params CreateWaitlistUserParams)
	}{
		{
			name: "create waitlist user with required fields",
			setup: func(t *testing.T) CreateWaitlistUserParams {
				t.Helper()
				account := createTestAccount(t, testDB)
				campaign := createTestCampaign(t, testDB, account.ID, "Test-"+uuid.New().String(), "test-"+uuid.New().String())
				return CreateWaitlistUserParams{
					CampaignID:       campaign.ID,
					Email:            uuid.New().String() + "@example.com",
					ReferralCode:     "TEST-" + uuid.New().String(),
					Position:         1,
					OriginalPosition: 1,
					TermsAccepted:    true,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, user WaitlistUser, params CreateWaitlistUserParams) {
				t.Helper()
				if user.ID == uuid.Nil {
					t.Error("expected user ID to be set")
				}
				if user.Email != params.Email {
					t.Errorf("Email = %v, want %v", user.Email, params.Email)
				}
				if user.ReferralCode != params.ReferralCode {
					t.Errorf("ReferralCode = %v, want %v", user.ReferralCode, params.ReferralCode)
				}
				if user.Position != params.Position {
					t.Errorf("Position = %v, want %v", user.Position, params.Position)
				}
				if user.EmailVerified {
					t.Error("EmailVerified should be false initially")
				}
			},
		},
		{
			name: "create waitlist user with custom fields",
			setup: func(t *testing.T) CreateWaitlistUserParams {
				t.Helper()
				account := createTestAccount(t, testDB)
				campaign := createTestCampaign(t, testDB, account.ID, "Custom Fields-"+uuid.New().String(), "custom-fields-"+uuid.New().String())
				return CreateWaitlistUserParams{
					CampaignID:       campaign.ID,
					Email:            uuid.New().String() + "@example.com",
					ReferralCode:     "CUSTOM-" + uuid.New().String(),
					Position:         1,
					OriginalPosition: 1,
					Metadata: JSONB{
						"first_name": "John",
						"last_name":  "Doe",
						"company":    "Acme Inc",
					},
					TermsAccepted: true,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, user WaitlistUser, params CreateWaitlistUserParams) {
				t.Helper()
				if user.Metadata == nil {
					t.Fatal("Expected metadata to be set")
				}
				if user.Metadata["first_name"] != "John" {
					t.Errorf("Metadata first_name = %v, want John", user.Metadata["first_name"])
				}
				if user.Metadata["company"] != "Acme Inc" {
					t.Errorf("Metadata company = %v, want Acme Inc", user.Metadata["company"])
				}
			},
		},
		{
			name: "create waitlist user with UTM parameters",
			setup: func(t *testing.T) CreateWaitlistUserParams {
				t.Helper()
				account := createTestAccount(t, testDB)
				campaign := createTestCampaign(t, testDB, account.ID, "UTM Test-"+uuid.New().String(), "utm-test-"+uuid.New().String())
				utmSource := "facebook"
				utmMedium := "social"
				utmCampaign := "spring-launch"
				return CreateWaitlistUserParams{
					CampaignID:       campaign.ID,
					Email:            uuid.New().String() + "@example.com",
					ReferralCode:     "UTM-" + uuid.New().String(),
					Position:         1,
					OriginalPosition: 1,
					UTMSource:        &utmSource,
					UTMMedium:        &utmMedium,
					UTMCampaign:      &utmCampaign,
					TermsAccepted:    true,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, user WaitlistUser, params CreateWaitlistUserParams) {
				t.Helper()
				if user.UTMSource == nil || *user.UTMSource != "facebook" {
					t.Errorf("UTMSource = %v, want facebook", user.UTMSource)
				}
				if user.UTMMedium == nil || *user.UTMMedium != "social" {
					t.Errorf("UTMMedium = %v, want social", user.UTMMedium)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			params := tt.setup(t)

			user, err := testDB.Store.CreateWaitlistUser(ctx, params)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateWaitlistUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, user, params)
			}
		})
	}
}

func TestStore_GetWaitlistUserByEmail(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Email Test-"+uuid.New().String(), "email-test-"+uuid.New().String())

	// Create a user with unique email
	uniqueEmail := uuid.New().String() + "@example.com"
	created := createTestWaitlistUser(t, testDB, campaign.ID, uniqueEmail)

	// Find by email
	found, err := testDB.Store.GetWaitlistUserByEmail(ctx, campaign.ID, uniqueEmail)
	if err != nil {
		t.Fatalf("GetWaitlistUserByEmail() error = %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("Found user ID = %v, want %v", found.ID, created.ID)
	}
	if found.Email != uniqueEmail {
		t.Errorf("Email = %v, want %v", found.Email, uniqueEmail)
	}

	// Try to find non-existent email
	_, err = testDB.Store.GetWaitlistUserByEmail(ctx, campaign.ID, uuid.New().String()+"@notfound.com")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestStore_GetWaitlistUserByReferralCode(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Referral Test-"+uuid.New().String(), "referral-test-"+uuid.New().String())

	// Create a user with a unique referral code
	uniqueReferralCode := "MYCODE-" + uuid.New().String()
	created, err := testDB.Store.CreateWaitlistUser(ctx, CreateWaitlistUserParams{
		CampaignID:       campaign.ID,
		Email:            uuid.New().String() + "@example.com",
		ReferralCode:     uniqueReferralCode,
		Position:         1,
		OriginalPosition: 1,
		TermsAccepted:    true,
	})
	if err != nil {
		t.Fatalf("CreateWaitlistUser() error = %v", err)
	}

	// Find by referral code
	found, err := testDB.Store.GetWaitlistUserByReferralCode(ctx, uniqueReferralCode)
	if err != nil {
		t.Fatalf("GetWaitlistUserByReferralCode() error = %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("Found user ID = %v, want %v", found.ID, created.ID)
	}

	// Try to find non-existent code
	_, err = testDB.Store.GetWaitlistUserByReferralCode(ctx, "NOTFOUND-"+uuid.New().String())
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestStore_VerifyWaitlistUserEmail(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Verify Test-"+uuid.New().String(), "verify-test-"+uuid.New().String())

	user := createTestWaitlistUser(t, testDB, campaign.ID, uuid.New().String()+"@example.com")

	// Should be unverified initially
	if user.EmailVerified {
		t.Error("User should be unverified initially")
	}

	// Verify email
	err := testDB.Store.VerifyWaitlistUserEmail(ctx, user.ID)
	if err != nil {
		t.Fatalf("VerifyWaitlistUserEmail() error = %v", err)
	}

	// Check it's verified
	verified, err := testDB.Store.GetWaitlistUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetWaitlistUserByID() error = %v", err)
	}

	if !verified.EmailVerified {
		t.Error("User should be verified after calling VerifyWaitlistUserEmail")
	}
}

func TestStore_IncrementReferralCount(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Referral Count-"+uuid.New().String(), "referral-count-"+uuid.New().String())

	user := createTestWaitlistUser(t, testDB, campaign.ID, uuid.New().String()+"@example.com")

	// Initially should have 0 referrals
	if user.ReferralCount != 0 {
		t.Errorf("Initial ReferralCount = %v, want 0", user.ReferralCount)
	}

	// Increment referral count
	err := testDB.Store.IncrementReferralCount(ctx, user.ID)
	if err != nil {
		t.Fatalf("IncrementReferralCount() error = %v", err)
	}

	// Check count increased
	updated, err := testDB.Store.GetWaitlistUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetWaitlistUserByID() error = %v", err)
	}

	if updated.ReferralCount != 1 {
		t.Errorf("ReferralCount = %v, want 1", updated.ReferralCount)
	}

	// Increment again
	testDB.Store.IncrementReferralCount(ctx, user.ID)
	updated, _ = testDB.Store.GetWaitlistUserByID(ctx, user.ID)

	if updated.ReferralCount != 2 {
		t.Errorf("ReferralCount = %v, want 2", updated.ReferralCount)
	}
}

func TestStore_CountWaitlistUsersByCampaign(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Count Test-"+uuid.New().String(), "count-test-"+uuid.New().String())

	// Initially should be 0 for this specific campaign
	count, err := testDB.Store.CountWaitlistUsersByCampaign(ctx, campaign.ID)
	if err != nil {
		t.Fatalf("CountWaitlistUsersByCampaign() error = %v", err)
	}
	if count != 0 {
		t.Errorf("Initial count = %v, want 0", count)
	}

	// Create 5 users
	for i := 0; i < 5; i++ {
		createTestWaitlistUser(t, testDB, campaign.ID, uuid.New().String()+"@example.com")
	}

	// Should now be 5
	count, err = testDB.Store.CountWaitlistUsersByCampaign(ctx, campaign.ID)
	if err != nil {
		t.Fatalf("CountWaitlistUsersByCampaign() error = %v", err)
	}
	if count != 5 {
		t.Errorf("Count = %v, want 5", count)
	}
}

func TestStore_GetWaitlistUsersByCampaign(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "List Test-"+uuid.New().String(), "list-test-"+uuid.New().String())

	// Create 15 users
	for i := 1; i <= 15; i++ {
		createTestWaitlistUser(t, testDB, campaign.ID, uuid.New().String()+"@example.com")
	}

	// Test pagination - get first 10
	users, err := testDB.Store.GetWaitlistUsersByCampaign(ctx, campaign.ID, ListWaitlistUsersParams{
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("GetWaitlistUsersByCampaign() error = %v", err)
	}

	if len(users) != 10 {
		t.Errorf("First page got %d users, want 10", len(users))
	}

	// Test pagination - get next 5
	users, err = testDB.Store.GetWaitlistUsersByCampaign(ctx, campaign.ID, ListWaitlistUsersParams{
		Limit:  10,
		Offset: 10,
	})
	if err != nil {
		t.Fatalf("GetWaitlistUsersByCampaign() error = %v", err)
	}

	if len(users) != 5 {
		t.Errorf("Second page got %d users, want 5", len(users))
	}
}

func TestStore_GetWaitlistUsersByCampaignWithFilters(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Filter Test-"+uuid.New().String(), "filter-test-"+uuid.New().String())

	// Create verified user
	verifiedUser := createTestWaitlistUser(t, testDB, campaign.ID, uuid.New().String()+"@example.com")
	testDB.Store.VerifyWaitlistUserEmail(ctx, verifiedUser.ID)

	// Create unverified users
	createTestWaitlistUser(t, testDB, campaign.ID, uuid.New().String()+"@example.com")
	createTestWaitlistUser(t, testDB, campaign.ID, uuid.New().String()+"@example.com")

	// Filter by verified status
	verified := true
	users, err := testDB.Store.GetWaitlistUsersByCampaignWithFilters(ctx, ListWaitlistUsersWithFiltersParams{
		CampaignID: campaign.ID,
		Verified:   &verified,
		Limit:      10,
		Offset:     0,
	})
	if err != nil {
		t.Fatalf("GetWaitlistUsersByCampaignWithFilters() error = %v", err)
	}

	if len(users) != 1 {
		t.Errorf("Verified filter got %d users, want 1", len(users))
	}
	if users[0].ID != verifiedUser.ID {
		t.Error("Wrong user returned for verified filter")
	}

	// Filter by unverified
	unverified := false
	users, err = testDB.Store.GetWaitlistUsersByCampaignWithFilters(ctx, ListWaitlistUsersWithFiltersParams{
		CampaignID: campaign.ID,
		Verified:   &unverified,
		Limit:      10,
		Offset:     0,
	})
	if err != nil {
		t.Fatalf("GetWaitlistUsersByCampaignWithFilters() error = %v", err)
	}

	if len(users) != 2 {
		t.Errorf("Unverified filter got %d users, want 2", len(users))
	}
}

func TestStore_UpdateWaitlistUserPosition(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Position Test-"+uuid.New().String(), "position-test-"+uuid.New().String())

	user := createTestWaitlistUser(t, testDB, campaign.ID, uuid.New().String()+"@example.com")

	// Initially position 1
	if user.Position != 1 {
		t.Errorf("Initial position = %v, want 1", user.Position)
	}

	// Update position
	err := testDB.Store.UpdateWaitlistUserPosition(ctx, user.ID, 5)
	if err != nil {
		t.Fatalf("UpdateWaitlistUserPosition() error = %v", err)
	}

	// Check updated
	updated, err := testDB.Store.GetWaitlistUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetWaitlistUserByID() error = %v", err)
	}

	if updated.Position != 5 {
		t.Errorf("Position = %v, want 5", updated.Position)
	}
}

func TestStore_DeleteWaitlistUser(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Delete Test-"+uuid.New().String(), "delete-test-"+uuid.New().String())

	uniqueEmail := uuid.New().String() + "@example.com"
	user := createTestWaitlistUser(t, testDB, campaign.ID, uniqueEmail)

	// Delete the user
	err := testDB.Store.DeleteWaitlistUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("DeleteWaitlistUser() error = %v", err)
	}

	// Should not be found (soft delete)
	_, err = testDB.Store.GetWaitlistUserByID(ctx, user.ID)
	if err == nil {
		t.Error("Expected error when getting deleted user")
	}
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}

	// Should also not be found by email
	_, err = testDB.Store.GetWaitlistUserByEmail(ctx, campaign.ID, uniqueEmail)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound for deleted user by email, got %v", err)
	}
}

func TestStore_WaitlistUserWithReferral(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Referral Chain-"+uuid.New().String(), "referral-chain-"+uuid.New().String())

	// Create referrer
	referrer := createTestWaitlistUser(t, testDB, campaign.ID, uuid.New().String()+"@example.com")

	// Create referred user
	referred, err := testDB.Store.CreateWaitlistUser(ctx, CreateWaitlistUserParams{
		CampaignID:       campaign.ID,
		Email:            uuid.New().String() + "@example.com",
		ReferralCode:     "REF-" + uuid.New().String(),
		ReferredByID:     &referrer.ID,
		Position:         2,
		OriginalPosition: 2,
		TermsAccepted:    true,
	})
	if err != nil {
		t.Fatalf("CreateWaitlistUser() error = %v", err)
	}

	if referred.ReferredByID == nil {
		t.Fatal("ReferredByID should be set")
	}
	if *referred.ReferredByID != referrer.ID {
		t.Errorf("ReferredByID = %v, want %v", *referred.ReferredByID, referrer.ID)
	}
}

func TestStore_SearchWaitlistUsersByEmail(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	// Use a unique search prefix for this test
	searchPrefix := uuid.New().String()[:8]
	campaign := createTestCampaign(t, testDB, account.ID, "Search Test-"+uuid.New().String(), "search-test-"+uuid.New().String())

	// Create users with different emails containing the unique prefix
	createTestWaitlistUser(t, testDB, campaign.ID, searchPrefix+".john.doe@example.com")
	createTestWaitlistUser(t, testDB, campaign.ID, searchPrefix+".jane.smith@example.com")
	createTestWaitlistUser(t, testDB, campaign.ID, searchPrefix+".john.smith@test.com")

	// Search for the unique prefix + "john"
	query := searchPrefix + ".john"
	users, err := testDB.Store.SearchWaitlistUsers(ctx, SearchWaitlistUsersParams{
		CampaignID: campaign.ID,
		Query:      &query,
		Limit:      10,
		Offset:     0,
	})
	if err != nil {
		t.Fatalf("SearchWaitlistUsers() error = %v", err)
	}

	if len(users) != 2 {
		t.Errorf("Search '%s' got %d users, want 2", query, len(users))
	}
}

func TestStore_ListWaitlistUsersWithExtendedFilters(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Extended Filter Test-"+uuid.New().String(), "extended-filter-test-"+uuid.New().String())

	// Create users with different attributes for filtering
	// User 1: verified, organic source, with referrals, position 1, company=Acme
	user1, err := testDB.Store.CreateWaitlistUser(ctx, CreateWaitlistUserParams{
		CampaignID:       campaign.ID,
		Email:            uuid.New().String() + "@example.com",
		ReferralCode:     "REF1-" + uuid.New().String(),
		Position:         1,
		OriginalPosition: 1,
		Source:           stringPtr("organic"),
		Metadata: JSONB{
			"company": "Acme",
			"role":    "Developer",
		},
		TermsAccepted: true,
	})
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}
	testDB.Store.VerifyWaitlistUserEmail(ctx, user1.ID)
	testDB.Store.IncrementReferralCount(ctx, user1.ID)
	testDB.Store.IncrementReferralCount(ctx, user1.ID)

	// User 2: unverified, referral source, no referrals, position 2, company=Beta
	_, err = testDB.Store.CreateWaitlistUser(ctx, CreateWaitlistUserParams{
		CampaignID:       campaign.ID,
		Email:            uuid.New().String() + "@example.com",
		ReferralCode:     "REF2-" + uuid.New().String(),
		Position:         2,
		OriginalPosition: 2,
		Source:           stringPtr("referral"),
		Metadata: JSONB{
			"company": "Beta",
			"role":    "Manager",
		},
		TermsAccepted: true,
	})
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	// User 3: verified, twitter source, with referrals, position 5, company=Acme
	user3, err := testDB.Store.CreateWaitlistUser(ctx, CreateWaitlistUserParams{
		CampaignID:       campaign.ID,
		Email:            uuid.New().String() + "@example.com",
		ReferralCode:     "REF3-" + uuid.New().String(),
		Position:         5,
		OriginalPosition: 5,
		Source:           stringPtr("twitter"),
		Metadata: JSONB{
			"company": "Acme",
			"role":    "Designer",
		},
		TermsAccepted: true,
	})
	if err != nil {
		t.Fatalf("Failed to create user3: %v", err)
	}
	testDB.Store.VerifyWaitlistUserEmail(ctx, user3.ID)
	testDB.Store.IncrementReferralCount(ctx, user3.ID)

	// User 4: unverified, organic source, no referrals, position 10, company=Gamma
	_, err = testDB.Store.CreateWaitlistUser(ctx, CreateWaitlistUserParams{
		CampaignID:       campaign.ID,
		Email:            uuid.New().String() + "@example.com",
		ReferralCode:     "REF4-" + uuid.New().String(),
		Position:         10,
		OriginalPosition: 10,
		Source:           stringPtr("organic"),
		Metadata: JSONB{
			"company": "Gamma",
			"role":    "Developer",
		},
		TermsAccepted: true,
	})
	if err != nil {
		t.Fatalf("Failed to create user4: %v", err)
	}

	tests := []struct {
		name          string
		params        ExtendedListWaitlistUsersParams
		expectedCount int
		description   string
	}{
		{
			name: "filter by multiple statuses",
			params: ExtendedListWaitlistUsersParams{
				CampaignID: campaign.ID,
				Statuses:   []string{"pending", "verified"},
				Limit:      10,
				Offset:     0,
			},
			expectedCount: 4, // All users are pending or verified
			description:   "Should return all users with pending or verified status",
		},
		{
			name: "filter by single source",
			params: ExtendedListWaitlistUsersParams{
				CampaignID: campaign.ID,
				Sources:    []string{"organic"},
				Limit:      10,
				Offset:     0,
			},
			expectedCount: 2, // Users 1 and 4
			description:   "Should return only organic source users",
		},
		{
			name: "filter by multiple sources",
			params: ExtendedListWaitlistUsersParams{
				CampaignID: campaign.ID,
				Sources:    []string{"organic", "referral"},
				Limit:      10,
				Offset:     0,
			},
			expectedCount: 3, // Users 1, 2, and 4
			description:   "Should return organic and referral source users",
		},
		{
			name: "filter by hasReferrals",
			params: ExtendedListWaitlistUsersParams{
				CampaignID:   campaign.ID,
				HasReferrals: boolPtr(true),
				Limit:        10,
				Offset:       0,
			},
			expectedCount: 2, // Users 1 and 3
			description:   "Should return only users with referrals",
		},
		{
			name: "filter by position range",
			params: ExtendedListWaitlistUsersParams{
				CampaignID:  campaign.ID,
				MinPosition: intPtr(2),
				MaxPosition: intPtr(6),
				Limit:       10,
				Offset:      0,
			},
			expectedCount: 2, // Users 2 (position 2) and 3 (position 5)
			description:   "Should return users with position between 2 and 6",
		},
		{
			name: "filter by custom field (company=Acme)",
			params: ExtendedListWaitlistUsersParams{
				CampaignID:   campaign.ID,
				CustomFields: map[string]string{"company": "Acme"},
				Limit:        10,
				Offset:       0,
			},
			expectedCount: 2, // Users 1 and 3
			description:   "Should return only users from Acme company",
		},
		{
			name: "filter by custom field (role=Developer)",
			params: ExtendedListWaitlistUsersParams{
				CampaignID:   campaign.ID,
				CustomFields: map[string]string{"role": "Developer"},
				Limit:        10,
				Offset:       0,
			},
			expectedCount: 2, // Users 1 and 4
			description:   "Should return only developers",
		},
		{
			name: "combine source and hasReferrals filters",
			params: ExtendedListWaitlistUsersParams{
				CampaignID:   campaign.ID,
				Sources:      []string{"organic"},
				HasReferrals: boolPtr(true),
				Limit:        10,
				Offset:       0,
			},
			expectedCount: 1, // Only User 1
			description:   "Should return organic users with referrals",
		},
		{
			name: "combine custom fields and source filter",
			params: ExtendedListWaitlistUsersParams{
				CampaignID:   campaign.ID,
				CustomFields: map[string]string{"company": "Acme"},
				Sources:      []string{"organic"},
				Limit:        10,
				Offset:       0,
			},
			expectedCount: 1, // Only User 1 (Acme + organic)
			description:   "Should return organic users from Acme",
		},
		{
			name: "sort by position ascending",
			params: ExtendedListWaitlistUsersParams{
				CampaignID: campaign.ID,
				SortBy:     "position",
				SortOrder:  "asc",
				Limit:      10,
				Offset:     0,
			},
			expectedCount: 4,
			description:   "Should return all users sorted by position ascending",
		},
		{
			name: "sort by position descending",
			params: ExtendedListWaitlistUsersParams{
				CampaignID: campaign.ID,
				SortBy:     "position",
				SortOrder:  "desc",
				Limit:      10,
				Offset:     0,
			},
			expectedCount: 4,
			description:   "Should return all users sorted by position descending",
		},
		{
			name: "pagination with limit and offset",
			params: ExtendedListWaitlistUsersParams{
				CampaignID: campaign.ID,
				SortBy:     "position",
				SortOrder:  "asc",
				Limit:      2,
				Offset:     1,
			},
			expectedCount: 2, // Skip first, get next 2
			description:   "Should return 2 users starting from offset 1",
		},
		{
			name: "no results for non-matching filter",
			params: ExtendedListWaitlistUsersParams{
				CampaignID:   campaign.ID,
				CustomFields: map[string]string{"company": "NonExistent"},
				Limit:        10,
				Offset:       0,
			},
			expectedCount: 0,
			description:   "Should return no users for non-existing company",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users, err := testDB.Store.ListWaitlistUsersWithExtendedFilters(ctx, tt.params)
			if err != nil {
				t.Fatalf("ListWaitlistUsersWithExtendedFilters() error = %v", err)
			}

			if len(users) != tt.expectedCount {
				t.Errorf("%s: got %d users, want %d", tt.description, len(users), tt.expectedCount)
			}
		})
	}

	// Test sorting order verification
	t.Run("verify sort order ascending", func(t *testing.T) {
		users, err := testDB.Store.ListWaitlistUsersWithExtendedFilters(ctx, ExtendedListWaitlistUsersParams{
			CampaignID: campaign.ID,
			SortBy:     "position",
			SortOrder:  "asc",
			Limit:      10,
			Offset:     0,
		})
		if err != nil {
			t.Fatalf("ListWaitlistUsersWithExtendedFilters() error = %v", err)
		}

		for i := 1; i < len(users); i++ {
			if users[i].Position < users[i-1].Position {
				t.Errorf("Users not sorted in ascending order: position %d came after %d",
					users[i].Position, users[i-1].Position)
			}
		}
	})

	t.Run("verify sort order descending", func(t *testing.T) {
		users, err := testDB.Store.ListWaitlistUsersWithExtendedFilters(ctx, ExtendedListWaitlistUsersParams{
			CampaignID: campaign.ID,
			SortBy:     "position",
			SortOrder:  "desc",
			Limit:      10,
			Offset:     0,
		})
		if err != nil {
			t.Fatalf("ListWaitlistUsersWithExtendedFilters() error = %v", err)
		}

		for i := 1; i < len(users); i++ {
			if users[i].Position > users[i-1].Position {
				t.Errorf("Users not sorted in descending order: position %d came after %d",
					users[i].Position, users[i-1].Position)
			}
		}
	})
}

func TestStore_CountWaitlistUsersWithExtendedFilters(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Count Extended Test-"+uuid.New().String(), "count-extended-test-"+uuid.New().String())

	// Create 5 users with different attributes
	for i := 0; i < 5; i++ {
		source := "organic"
		if i%2 == 0 {
			source = "referral"
		}
		company := "Acme"
		if i > 2 {
			company = "Beta"
		}

		user, err := testDB.Store.CreateWaitlistUser(ctx, CreateWaitlistUserParams{
			CampaignID:       campaign.ID,
			Email:            uuid.New().String() + "@example.com",
			ReferralCode:     "CREF-" + uuid.New().String(),
			Position:         i + 1,
			OriginalPosition: i + 1,
			Source:           stringPtr(source),
			Metadata: JSONB{
				"company": company,
			},
			TermsAccepted: true,
		})
		if err != nil {
			t.Fatalf("Failed to create user %d: %v", i, err)
		}

		// Give some users referrals
		if i < 2 {
			testDB.Store.IncrementReferralCount(ctx, user.ID)
		}
	}

	tests := []struct {
		name          string
		params        ExtendedListWaitlistUsersParams
		expectedCount int
	}{
		{
			name: "count all users",
			params: ExtendedListWaitlistUsersParams{
				CampaignID: campaign.ID,
			},
			expectedCount: 5,
		},
		{
			name: "count by source organic",
			params: ExtendedListWaitlistUsersParams{
				CampaignID: campaign.ID,
				Sources:    []string{"organic"},
			},
			expectedCount: 2,
		},
		{
			name: "count by source referral",
			params: ExtendedListWaitlistUsersParams{
				CampaignID: campaign.ID,
				Sources:    []string{"referral"},
			},
			expectedCount: 3,
		},
		{
			name: "count with hasReferrals",
			params: ExtendedListWaitlistUsersParams{
				CampaignID:   campaign.ID,
				HasReferrals: boolPtr(true),
			},
			expectedCount: 2,
		},
		{
			name: "count by custom field company=Acme",
			params: ExtendedListWaitlistUsersParams{
				CampaignID:   campaign.ID,
				CustomFields: map[string]string{"company": "Acme"},
			},
			expectedCount: 3,
		},
		{
			name: "count by position range",
			params: ExtendedListWaitlistUsersParams{
				CampaignID:  campaign.ID,
				MinPosition: intPtr(2),
				MaxPosition: intPtr(4),
			},
			expectedCount: 3,
		},
		{
			name: "count with combined filters",
			params: ExtendedListWaitlistUsersParams{
				CampaignID:   campaign.ID,
				Sources:      []string{"referral"},
				CustomFields: map[string]string{"company": "Acme"},
			},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := testDB.Store.CountWaitlistUsersWithExtendedFilters(ctx, tt.params)
			if err != nil {
				t.Fatalf("CountWaitlistUsersWithExtendedFilters() error = %v", err)
			}

			if count != tt.expectedCount {
				t.Errorf("got count %d, want %d", count, tt.expectedCount)
			}
		})
	}
}

// Helper functions for test data
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func intPtr(i int) *int {
	return &i
}
