package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_CreateCampaign(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	tests := []struct {
		name     string
		setup    func() CreateCampaignParams
		wantErr  bool
		validate func(t *testing.T, campaign Campaign, params CreateCampaignParams)
	}{
		{
			name: "creates campaign with required fields",
			setup: func() CreateCampaignParams {
				account := f.CreateAccount()
				return CreateCampaignParams{
					AccountID: account.ID,
					Name:      "Test Campaign",
					Slug:      "test-campaign-" + uuid.New().String()[:8],
					Type:      "waitlist",
				}
			},
			validate: func(t *testing.T, campaign Campaign, params CreateCampaignParams) {
				assert.NotEqual(t, uuid.Nil, campaign.ID)
				assert.Equal(t, params.Name, campaign.Name)
				assert.Equal(t, params.Slug, campaign.Slug)
				assert.Equal(t, params.Type, campaign.Type)
				assert.Equal(t, "draft", campaign.Status)
				assert.Equal(t, 0, campaign.TotalSignups)
				assert.Equal(t, 0, campaign.TotalVerified)
			},
		},
		{
			name: "creates campaign with optional fields",
			setup: func() CreateCampaignParams {
				account := f.CreateAccount()
				return CreateCampaignParams{
					AccountID:        account.ID,
					Name:             "Full Campaign",
					Slug:             "full-campaign-" + uuid.New().String()[:8],
					Description:      Ptr("Test Description"),
					Type:             "referral",
					MaxSignups:       Ptr(1000),
					PrivacyPolicyURL: Ptr("https://example.com/privacy"),
					TermsURL:         Ptr("https://example.com/terms"),
				}
			},
			validate: func(t *testing.T, campaign Campaign, params CreateCampaignParams) {
				assert.Equal(t, *params.Description, *campaign.Description)
				assert.Equal(t, *params.MaxSignups, *campaign.MaxSignups)
				assert.Equal(t, *params.PrivacyPolicyURL, *campaign.PrivacyPolicyURL)
				assert.Equal(t, *params.TermsURL, *campaign.TermsURL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			params := tt.setup()

			campaign, err := testDB.Store.CreateCampaign(ctx, params)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, campaign, params)
			}
		})
	}
}

func TestStore_GetCampaignByID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("returns campaign with signup counters", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()
		campaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })

		// Initially should have 0 signups
		retrieved, err := testDB.Store.GetCampaignByID(ctx, campaign.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, retrieved.TotalSignups)
		assert.Equal(t, 0, retrieved.TotalVerified)

		// Create 3 waitlist users
		for i := 0; i < 3; i++ {
			f.CreateWaitlistUser(func(o *WaitlistUserOpts) { o.CampaignID = &campaign.ID })
		}

		// Should now have 3 signups
		retrieved, err = testDB.Store.GetCampaignByID(ctx, campaign.ID)
		require.NoError(t, err)
		assert.Equal(t, 3, retrieved.TotalSignups)
	})

	t.Run("returns ErrNotFound for non-existent campaign", func(t *testing.T) {
		testDB.Truncate(t)
		nonExistentID := uuid.New()

		_, err := testDB.Store.GetCampaignByID(ctx, nonExistentID)

		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestStore_GetCampaignBySlug(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("returns campaign with signup counters", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()
		slug := "slug-test-" + uuid.New().String()[:8]
		campaign := f.CreateCampaign(func(o *CampaignOpts) {
			o.AccountID = &account.ID
			o.Slug = slug
		})

		// Add 2 users
		for i := 0; i < 2; i++ {
			f.CreateWaitlistUser(func(o *WaitlistUserOpts) { o.CampaignID = &campaign.ID })
		}

		retrieved, err := testDB.Store.GetCampaignBySlug(ctx, account.ID, slug)
		require.NoError(t, err)
		assert.Equal(t, 2, retrieved.TotalSignups)
	})

	t.Run("returns ErrNotFound for non-existent slug", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()

		_, err := testDB.Store.GetCampaignBySlug(ctx, account.ID, "non-existent-slug")

		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestStore_UpdateCampaign(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	tests := []struct {
		name     string
		params   UpdateCampaignParams
		validate func(t *testing.T, updated Campaign)
	}{
		{
			name: "updates name",
			params: UpdateCampaignParams{
				Name: Ptr("Updated Name"),
			},
			validate: func(t *testing.T, updated Campaign) {
				assert.Equal(t, "Updated Name", updated.Name)
			},
		},
		{
			name: "updates description",
			params: UpdateCampaignParams{
				Description: Ptr("Updated Description"),
			},
			validate: func(t *testing.T, updated Campaign) {
				assert.Equal(t, "Updated Description", *updated.Description)
			},
		},
		{
			name: "updates multiple fields",
			params: UpdateCampaignParams{
				Name:             Ptr("Multi Update"),
				Description:      Ptr("Multi field update"),
				PrivacyPolicyURL: Ptr("https://new-privacy.com"),
			},
			validate: func(t *testing.T, updated Campaign) {
				assert.Equal(t, "Multi Update", updated.Name)
				assert.Equal(t, "Multi field update", *updated.Description)
				assert.Equal(t, "https://new-privacy.com", *updated.PrivacyPolicyURL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			account := f.CreateAccount()
			campaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })

			updated, err := testDB.Store.UpdateCampaign(ctx, account.ID, campaign.ID, tt.params)

			require.NoError(t, err)
			tt.validate(t, updated)
		})
	}

	t.Run("returns ErrNotFound for non-existent campaign", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()

		_, err := testDB.Store.UpdateCampaign(ctx, account.ID, uuid.New(), UpdateCampaignParams{
			Name: Ptr("Updated"),
		})

		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestStore_UpdateCampaignStatus(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	statuses := []string{"active", "paused", "completed", "draft"}

	for _, status := range statuses {
		t.Run("updates status to "+status, func(t *testing.T) {
			testDB.Truncate(t)
			account := f.CreateAccount()
			campaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })

			updated, err := testDB.Store.UpdateCampaignStatus(ctx, account.ID, campaign.ID, status)

			require.NoError(t, err)
			assert.Equal(t, status, updated.Status)
		})
	}

	t.Run("returns ErrNotFound for non-existent campaign", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()

		_, err := testDB.Store.UpdateCampaignStatus(ctx, account.ID, uuid.New(), "active")

		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestStore_DeleteCampaign(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("soft deletes campaign", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()
		campaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })

		err := testDB.Store.DeleteCampaign(ctx, account.ID, campaign.ID)
		require.NoError(t, err)

		// Should not be found after deletion
		_, err = testDB.Store.GetCampaignByID(ctx, campaign.ID)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("returns ErrNotFound for non-existent campaign", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()

		err := testDB.Store.DeleteCampaign(ctx, account.ID, uuid.New())

		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestStore_ListCampaigns(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("returns paginated results", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()

		// Create 5 campaigns
		for i := 0; i < 5; i++ {
			f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })
		}

		result, err := testDB.Store.ListCampaigns(ctx, ListCampaignsParams{
			AccountID: account.ID,
			Page:      1,
			Limit:     2,
		})

		require.NoError(t, err)
		assert.Len(t, result.Campaigns, 2)
		assert.Equal(t, 5, result.TotalCount)
		assert.Equal(t, 3, result.TotalPages)
	})

	t.Run("filters by status", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()

		draftCampaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })
		activeCampaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })
		testDB.Store.UpdateCampaignStatus(ctx, account.ID, activeCampaign.ID, "active")

		result, err := testDB.Store.ListCampaigns(ctx, ListCampaignsParams{
			AccountID: account.ID,
			Page:      1,
			Limit:     10,
			Status:    Ptr("active"),
		})

		require.NoError(t, err)
		assert.Len(t, result.Campaigns, 1)
		assert.Equal(t, activeCampaign.ID, result.Campaigns[0].ID)
		_ = draftCampaign // suppress unused variable warning
	})

	t.Run("includes signup counters", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()
		campaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })

		// Add 3 waitlist users
		for i := 0; i < 3; i++ {
			f.CreateWaitlistUser(func(o *WaitlistUserOpts) { o.CampaignID = &campaign.ID })
		}

		result, err := testDB.Store.ListCampaigns(ctx, ListCampaignsParams{
			AccountID: account.ID,
			Page:      1,
			Limit:     10,
		})

		require.NoError(t, err)
		require.Len(t, result.Campaigns, 1)
		assert.Equal(t, 3, result.Campaigns[0].TotalSignups)
	})
}

func TestStore_GetCampaignsByAccountID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("returns only campaigns for specified account", func(t *testing.T) {
		testDB.Truncate(t)
		account1 := f.CreateAccount()
		account2 := f.CreateAccount()

		// Create campaigns for account1
		f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account1.ID })
		f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account1.ID })
		// Create campaign for account2
		f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account2.ID })

		campaigns, err := testDB.Store.GetCampaignsByAccountID(ctx, account1.ID)

		require.NoError(t, err)
		assert.Len(t, campaigns, 2)
		for _, c := range campaigns {
			assert.Equal(t, account1.ID, c.AccountID)
		}
	})
}

func TestStore_GetCampaignsByStatus(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("returns only campaigns with specified status", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()

		draftCampaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })
		activeCampaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })
		testDB.Store.UpdateCampaignStatus(ctx, account.ID, activeCampaign.ID, "active")

		draftCampaigns, err := testDB.Store.GetCampaignsByStatus(ctx, account.ID, "draft")
		require.NoError(t, err)
		assert.Len(t, draftCampaigns, 1)
		assert.Equal(t, draftCampaign.ID, draftCampaigns[0].ID)

		activeCampaigns, err := testDB.Store.GetCampaignsByStatus(ctx, account.ID, "active")
		require.NoError(t, err)
		assert.Len(t, activeCampaigns, 1)
		assert.Equal(t, activeCampaign.ID, activeCampaigns[0].ID)
	})
}

func TestStore_CountersWithSoftDeletedUsers(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	account := f.CreateAccount()
	campaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })

	// Create 3 users
	user1 := f.CreateWaitlistUser(func(o *WaitlistUserOpts) { o.CampaignID = &campaign.ID })
	f.CreateWaitlistUser(func(o *WaitlistUserOpts) { o.CampaignID = &campaign.ID })
	f.CreateWaitlistUser(func(o *WaitlistUserOpts) { o.CampaignID = &campaign.ID })

	// Should have 3 signups
	retrieved, _ := testDB.Store.GetCampaignByID(ctx, campaign.ID)
	assert.Equal(t, 3, retrieved.TotalSignups)

	// Soft delete one user
	err := testDB.Store.DeleteWaitlistUser(ctx, user1.ID)
	require.NoError(t, err)

	// Should now have 2 signups
	retrieved, _ = testDB.Store.GetCampaignByID(ctx, campaign.ID)
	assert.Equal(t, 2, retrieved.TotalSignups)
}

func TestStore_CampaignEmailSettings(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("CRUD operations", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()
		campaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })

		// Create
		created, err := testDB.Store.CreateCampaignEmailSettings(ctx, CreateCampaignEmailSettingsParams{
			CampaignID:           campaign.ID,
			FromName:             Ptr("Test Sender"),
			FromEmail:            Ptr("sender@example.com"),
			ReplyTo:              Ptr("reply@example.com"),
			VerificationRequired: true,
			SendWelcomeEmail:     true,
		})
		require.NoError(t, err)
		assert.Equal(t, campaign.ID, created.CampaignID)
		assert.Equal(t, "Test Sender", *created.FromName)
		assert.True(t, created.VerificationRequired)

		// Read
		retrieved, err := testDB.Store.GetCampaignEmailSettings(ctx, campaign.ID)
		require.NoError(t, err)
		assert.Equal(t, "sender@example.com", *retrieved.FromEmail)

		// Update
		updated, err := testDB.Store.UpdateCampaignEmailSettings(ctx, campaign.ID, UpdateCampaignEmailSettingsParams{
			FromName:         Ptr("Updated Sender"),
			SendWelcomeEmail: Ptr(false),
		})
		require.NoError(t, err)
		assert.Equal(t, "Updated Sender", *updated.FromName)
		assert.False(t, updated.SendWelcomeEmail)

		// Delete
		err = testDB.Store.DeleteCampaignEmailSettings(ctx, campaign.ID)
		require.NoError(t, err)

		_, err = testDB.Store.GetCampaignEmailSettings(ctx, campaign.ID)
		assert.Error(t, err)
	})
}

func TestStore_CampaignFormSettings(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("create and upsert", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()
		campaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })

		captchaProvider := CaptchaProvider("turnstile")

		// Create
		created, err := testDB.Store.CreateCampaignFormSettings(ctx, CreateCampaignFormSettingsParams{
			CampaignID:      campaign.ID,
			CaptchaEnabled:  true,
			CaptchaProvider: &captchaProvider,
			CaptchaSiteKey:  Ptr("test-site-key"),
			DoubleOptIn:     true,
			Design:          JSONB{"theme": "dark"},
		})
		require.NoError(t, err)
		assert.True(t, created.CaptchaEnabled)
		assert.True(t, created.DoubleOptIn)

		// Upsert (update existing)
		newProvider := CaptchaProvider("recaptcha")
		upserted, err := testDB.Store.UpsertCampaignFormSettings(ctx, CreateCampaignFormSettingsParams{
			CampaignID:      campaign.ID,
			CaptchaEnabled:  true,
			CaptchaProvider: &newProvider,
			DoubleOptIn:     false,
			Design:          JSONB{"theme": "light"},
		})
		require.NoError(t, err)
		assert.Equal(t, newProvider, *upserted.CaptchaProvider)
		assert.False(t, upserted.DoubleOptIn)
	})
}

func TestStore_CampaignReferralSettings(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("create and update", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()
		campaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })

		// Create
		created, err := testDB.Store.CreateCampaignReferralSettings(ctx, CreateCampaignReferralSettingsParams{
			CampaignID:              campaign.ID,
			Enabled:                 true,
			PointsPerReferral:       10,
			VerifiedOnly:            true,
			PositionsToJump:         5,
			ReferrerPositionsToJump: 2,
			SharingChannels:         []SharingChannel{"email", "twitter"},
		})
		require.NoError(t, err)
		assert.True(t, created.Enabled)
		assert.Equal(t, 10, created.PointsPerReferral)
		assert.Len(t, created.SharingChannels, 2)

		// Update
		updated, err := testDB.Store.UpdateCampaignReferralSettings(ctx, campaign.ID, UpdateCampaignReferralSettingsParams{
			PointsPerReferral: Ptr(25),
			PositionsToJump:   Ptr(15),
		})
		require.NoError(t, err)
		assert.Equal(t, 25, updated.PointsPerReferral)
		assert.Equal(t, 15, updated.PositionsToJump)
	})
}

func TestStore_CampaignBrandingSettings(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("create and upsert", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()
		campaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })

		// Create
		created, err := testDB.Store.CreateCampaignBrandingSettings(ctx, CreateCampaignBrandingSettingsParams{
			CampaignID:   campaign.ID,
			LogoURL:      Ptr("https://example.com/logo.png"),
			PrimaryColor: Ptr("#FF5733"),
			FontFamily:   Ptr("Inter"),
			CustomDomain: Ptr("campaign.example.com"),
		})
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/logo.png", *created.LogoURL)
		assert.Equal(t, "#FF5733", *created.PrimaryColor)

		// Upsert
		upserted, err := testDB.Store.UpsertCampaignBrandingSettings(ctx, CreateCampaignBrandingSettingsParams{
			CampaignID:   campaign.ID,
			LogoURL:      Ptr("https://example.com/new-logo.png"),
			PrimaryColor: Ptr("#00FF00"),
		})
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/new-logo.png", *upserted.LogoURL)
	})
}

func TestStore_CampaignFormFields(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("CRUD and bulk operations", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()
		campaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })

		// Create single field
		field, err := testDB.Store.CreateCampaignFormField(ctx, CreateCampaignFormFieldParams{
			CampaignID:   campaign.ID,
			Name:         "email",
			FieldType:    FormFieldType("email"),
			Label:        "Email Address",
			Required:     true,
			DisplayOrder: 1,
		})
		require.NoError(t, err)
		assert.Equal(t, "email", field.Name)
		assert.Equal(t, FormFieldType("email"), field.FieldType)

		// Bulk create
		fields, err := testDB.Store.BulkCreateCampaignFormFields(ctx, []CreateCampaignFormFieldParams{
			{CampaignID: campaign.ID, Name: "name", FieldType: FormFieldType("text"), Label: "Full Name", DisplayOrder: 2},
			{CampaignID: campaign.ID, Name: "company", FieldType: FormFieldType("text"), Label: "Company", DisplayOrder: 3},
		})
		require.NoError(t, err)
		assert.Len(t, fields, 2)

		// Get all
		allFields, err := testDB.Store.GetCampaignFormFields(ctx, campaign.ID)
		require.NoError(t, err)
		assert.Len(t, allFields, 3)

		// Replace all
		replaced, err := testDB.Store.ReplaceCampaignFormFields(ctx, campaign.ID, []CreateCampaignFormFieldParams{
			{CampaignID: campaign.ID, Name: "new_email", FieldType: FormFieldType("email"), Label: "New Email", DisplayOrder: 1},
		})
		require.NoError(t, err)
		assert.Len(t, replaced, 1)
		assert.Equal(t, "new_email", replaced[0].Name)
	})
}

func TestStore_CampaignShareMessages(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("CRUD and replace operations", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()
		campaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })

		// Create
		msg, err := testDB.Store.CreateCampaignShareMessage(ctx, CreateCampaignShareMessageParams{
			CampaignID: campaign.ID,
			Channel:    SharingChannel("email"),
			Message:    "Check out this campaign!",
		})
		require.NoError(t, err)
		assert.Equal(t, SharingChannel("email"), msg.Channel)

		// Get by channel
		retrieved, err := testDB.Store.GetCampaignShareMessageByChannel(ctx, campaign.ID, SharingChannel("email"))
		require.NoError(t, err)
		assert.Equal(t, "Check out this campaign!", retrieved.Message)

		// Replace all
		replaced, err := testDB.Store.ReplaceCampaignShareMessages(ctx, campaign.ID, []CreateCampaignShareMessageParams{
			{CampaignID: campaign.ID, Channel: SharingChannel("twitter"), Message: "Tweet this!"},
			{CampaignID: campaign.ID, Channel: SharingChannel("facebook"), Message: "Share on FB!"},
		})
		require.NoError(t, err)
		assert.Len(t, replaced, 2)
	})
}

func TestStore_CampaignTrackingIntegrations(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("CRUD and filter by enabled", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()
		campaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })

		// Create
		integration, err := testDB.Store.CreateCampaignTrackingIntegration(ctx, CreateCampaignTrackingIntegrationParams{
			CampaignID:      campaign.ID,
			IntegrationType: TrackingIntegrationType("google_analytics"),
			Enabled:         true,
			TrackingID:      "GA-12345678",
			TrackingLabel:   Ptr("signup_conversion"),
		})
		require.NoError(t, err)
		assert.True(t, integration.Enabled)
		assert.Equal(t, "GA-12345678", integration.TrackingID)

		// Get by type
		byType, err := testDB.Store.GetCampaignTrackingIntegrationByType(ctx, campaign.ID, TrackingIntegrationType("google_analytics"))
		require.NoError(t, err)
		assert.Equal(t, "GA-12345678", byType.TrackingID)

		// Replace with mixed enabled states
		replaced, err := testDB.Store.ReplaceCampaignTrackingIntegrations(ctx, campaign.ID, []CreateCampaignTrackingIntegrationParams{
			{CampaignID: campaign.ID, IntegrationType: TrackingIntegrationType("meta_pixel"), Enabled: true, TrackingID: "123456789"},
			{CampaignID: campaign.ID, IntegrationType: TrackingIntegrationType("tiktok_pixel"), Enabled: false, TrackingID: "TT-999"},
		})
		require.NoError(t, err)
		assert.Len(t, replaced, 2)

		// Get only enabled
		enabled, err := testDB.Store.GetEnabledCampaignTrackingIntegrations(ctx, campaign.ID)
		require.NoError(t, err)
		assert.Len(t, enabled, 1)
		assert.Equal(t, "123456789", enabled[0].TrackingID)
	})
}

func TestStore_CampaignWithSettings(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("loads all settings", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()
		campaign := f.CreateCampaign(func(o *CampaignOpts) { o.AccountID = &account.ID })

		// Create all settings using fixtures
		f.CreateEmailSettings(campaign.ID, func(o *EmailSettingsOpts) {
			o.FromName = Ptr("Test")
			o.SendWelcomeEmail = true
		})
		f.CreateBrandingSettings(campaign.ID, func(o *BrandingSettingsOpts) {
			o.LogoURL = Ptr("https://example.com/logo.png")
		})
		f.CreateFormSettings(campaign.ID, func(o *FormSettingsOpts) {
			o.CaptchaEnabled = true
			o.Design = JSONB{"theme": "light"}
		})
		f.CreateReferralSettings(campaign.ID, func(o *ReferralSettingsOpts) {
			o.Enabled = true
			o.PointsPerReferral = 10
		})

		testDB.Store.CreateCampaignFormField(ctx, CreateCampaignFormFieldParams{
			CampaignID: campaign.ID, Name: "email", FieldType: FormFieldType("email"),
			Label: "Email", Required: true, DisplayOrder: 1,
		})
		testDB.Store.CreateCampaignShareMessage(ctx, CreateCampaignShareMessageParams{
			CampaignID: campaign.ID, Channel: SharingChannel("email"), Message: "Share!",
		})
		testDB.Store.CreateCampaignTrackingIntegration(ctx, CreateCampaignTrackingIntegrationParams{
			CampaignID: campaign.ID, IntegrationType: TrackingIntegrationType("google_analytics"),
			Enabled: true, TrackingID: "GA-123",
		})

		// Load campaign with all settings
		loaded, err := testDB.Store.GetCampaignWithSettings(ctx, campaign.ID)
		require.NoError(t, err)

		assert.NotNil(t, loaded.EmailSettings)
		assert.NotNil(t, loaded.BrandingSettings)
		assert.NotNil(t, loaded.FormSettings)
		assert.NotNil(t, loaded.ReferralSettings)
		assert.NotEmpty(t, loaded.FormFields)
		assert.NotEmpty(t, loaded.ShareMessages)
		assert.NotEmpty(t, loaded.TrackingIntegrations)
	})

	t.Run("by slug loads settings", func(t *testing.T) {
		testDB.Truncate(t)
		account := f.CreateAccount()
		slug := "settings-test-" + uuid.New().String()[:8]
		campaign := f.CreateCampaign(func(o *CampaignOpts) {
			o.AccountID = &account.ID
			o.Slug = slug
		})

		f.CreateEmailSettings(campaign.ID, func(o *EmailSettingsOpts) {
			o.SendWelcomeEmail = true
		})

		loaded, err := testDB.Store.GetCampaignBySlugWithSettings(ctx, account.ID, slug)
		require.NoError(t, err)
		assert.NotNil(t, loaded.EmailSettings)
	})
}
