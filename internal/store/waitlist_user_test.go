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
