package store

import (
	"context"
	"errors"
	"testing"

	"base-server/internal/integrations"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== Integration Subscription Store Tests =====

// createTestAPIKey creates an API key for testing subscriptions
func createTestAPIKey(t *testing.T, testDB *TestDB, accountID uuid.UUID) APIKey {
	t.Helper()
	ctx := context.Background()

	apiKey, err := testDB.Store.CreateAPIKey(ctx, CreateAPIKeyParams{
		AccountID:     accountID,
		Name:          "test-api-key-" + uuid.New().String(),
		KeyHash:       "hash_" + uuid.New().String(),
		KeyPrefix:     "test_",
		Scopes:        []string{"zapier"},
		RateLimitTier: "standard",
	})
	require.NoError(t, err)
	return apiKey
}

func TestStore_CreateIntegrationSubscription(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	tests := []struct {
		name    string
		setup   func(t *testing.T) integrations.CreateSubscriptionParams
		wantErr bool
	}{
		{
			name: "create subscription successfully",
			setup: func(t *testing.T) integrations.CreateSubscriptionParams {
				t.Helper()
				account := f.CreateAccount()
				apiKey := createTestAPIKey(t, testDB, account.ID)

				return integrations.CreateSubscriptionParams{
					AccountID:       account.ID,
					APIKeyID:        &apiKey.ID,
					IntegrationType: integrations.IntegrationZapier,
					TargetURL:       "https://hooks.zapier.com/hooks/catch/123/abc",
					EventType:       "user.created",
				}
			},
			wantErr: false,
		},
		{
			name: "create subscription with campaign filter",
			setup: func(t *testing.T) integrations.CreateSubscriptionParams {
				t.Helper()
				account := f.CreateAccount()
				apiKey := createTestAPIKey(t, testDB, account.ID)
				campaign := f.CreateCampaign(func(o *CampaignOpts) {
					o.AccountID = &account.ID
				})

				return integrations.CreateSubscriptionParams{
					AccountID:       account.ID,
					APIKeyID:        &apiKey.ID,
					IntegrationType: integrations.IntegrationZapier,
					TargetURL:       "https://hooks.zapier.com/hooks/catch/456/def",
					EventType:       "user.verified",
					CampaignID:      &campaign.ID,
				}
			},
			wantErr: false,
		},
		{
			name: "create subscription with config",
			setup: func(t *testing.T) integrations.CreateSubscriptionParams {
				t.Helper()
				account := f.CreateAccount()
				apiKey := createTestAPIKey(t, testDB, account.ID)

				return integrations.CreateSubscriptionParams{
					AccountID:       account.ID,
					APIKeyID:        &apiKey.ID,
					IntegrationType: integrations.IntegrationSlack,
					TargetURL:       "https://hooks.slack.com/services/T00/B00/XXX",
					EventType:       "reward.earned",
					Config: map[string]interface{}{
						"channel": "#alerts",
						"format":  "detailed",
					},
				}
			},
			wantErr: false,
		},
		{
			name: "create subscription without API key",
			setup: func(t *testing.T) integrations.CreateSubscriptionParams {
				t.Helper()
				account := f.CreateAccount()

				return integrations.CreateSubscriptionParams{
					AccountID:       account.ID,
					APIKeyID:        nil, // No API key
					IntegrationType: integrations.IntegrationZapier,
					TargetURL:       "https://hooks.zapier.com/hooks/catch/789/ghi",
					EventType:       "user.created",
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			params := tt.setup(t)

			subscription, err := testDB.Store.CreateIntegrationSubscription(ctx, params)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEqual(t, uuid.Nil, subscription.ID)
			assert.Equal(t, params.AccountID, subscription.AccountID)
			assert.Equal(t, params.APIKeyID, subscription.APIKeyID)
			assert.Equal(t, params.IntegrationType, subscription.IntegrationType)
			assert.Equal(t, params.TargetURL, subscription.TargetURL)
			assert.Equal(t, params.EventType, subscription.EventType)
			assert.Equal(t, "active", subscription.Status)
			assert.Equal(t, 0, subscription.TriggerCount)
			assert.Equal(t, 0, subscription.ErrorCount)
		})
	}
}

func TestStore_GetIntegrationSubscriptionByID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	tests := []struct {
		name    string
		setup   func(t *testing.T) uuid.UUID
		wantErr bool
		errType error
	}{
		{
			name: "get existing subscription",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				account := f.CreateAccount()
				apiKey := createTestAPIKey(t, testDB, account.ID)

				sub, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
					AccountID:       account.ID,
					APIKeyID:        &apiKey.ID,
					IntegrationType: integrations.IntegrationZapier,
					TargetURL:       "https://hooks.zapier.com/test",
					EventType:       "user.created",
				})
				require.NoError(t, err)
				return sub.ID
			},
			wantErr: false,
		},
		{
			name: "subscription not found",
			setup: func(t *testing.T) uuid.UUID {
				return uuid.New()
			},
			wantErr: true,
			errType: ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			subID := tt.setup(t)

			subscription, err := testDB.Store.GetIntegrationSubscriptionByID(ctx, subID)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					assert.True(t, errors.Is(err, tt.errType))
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, subID, subscription.ID)
		})
	}
}

func TestStore_GetIntegrationSubscriptionsByAccount(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	tests := []struct {
		name          string
		setup         func(t *testing.T) (uuid.UUID, *integrations.IntegrationType)
		expectedCount int
	}{
		{
			name: "get all subscriptions for account",
			setup: func(t *testing.T) (uuid.UUID, *integrations.IntegrationType) {
				t.Helper()
				account := f.CreateAccount()
				apiKey := createTestAPIKey(t, testDB, account.ID)

				for i := 0; i < 3; i++ {
					_, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
						AccountID:       account.ID,
						APIKeyID:        &apiKey.ID,
						IntegrationType: integrations.IntegrationZapier,
						TargetURL:       "https://hooks.zapier.com/" + uuid.New().String(),
						EventType:       "user.created",
					})
					require.NoError(t, err)
				}

				return account.ID, nil
			},
			expectedCount: 3,
		},
		{
			name: "filter by integration type",
			setup: func(t *testing.T) (uuid.UUID, *integrations.IntegrationType) {
				t.Helper()
				account := f.CreateAccount()
				apiKey := createTestAPIKey(t, testDB, account.ID)

				// Create Zapier subscriptions
				for i := 0; i < 2; i++ {
					_, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
						AccountID:       account.ID,
						APIKeyID:        &apiKey.ID,
						IntegrationType: integrations.IntegrationZapier,
						TargetURL:       "https://hooks.zapier.com/" + uuid.New().String(),
						EventType:       "user.created",
					})
					require.NoError(t, err)
				}

				// Create Slack subscription
				_, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
					AccountID:       account.ID,
					APIKeyID:        &apiKey.ID,
					IntegrationType: integrations.IntegrationSlack,
					TargetURL:       "https://hooks.slack.com/test",
					EventType:       "user.created",
				})
				require.NoError(t, err)

				zapierType := integrations.IntegrationZapier
				return account.ID, &zapierType
			},
			expectedCount: 2,
		},
		{
			name: "exclude deleted subscriptions",
			setup: func(t *testing.T) (uuid.UUID, *integrations.IntegrationType) {
				t.Helper()
				account := f.CreateAccount()
				apiKey := createTestAPIKey(t, testDB, account.ID)

				// Create 2 active subscriptions
				for i := 0; i < 2; i++ {
					_, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
						AccountID:       account.ID,
						APIKeyID:        &apiKey.ID,
						IntegrationType: integrations.IntegrationZapier,
						TargetURL:       "https://hooks.zapier.com/" + uuid.New().String(),
						EventType:       "user.created",
					})
					require.NoError(t, err)
				}

				// Create and delete 1 subscription
				deletedSub, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
					AccountID:       account.ID,
					APIKeyID:        &apiKey.ID,
					IntegrationType: integrations.IntegrationZapier,
					TargetURL:       "https://hooks.zapier.com/deleted",
					EventType:       "user.created",
				})
				require.NoError(t, err)
				err = testDB.Store.DeleteIntegrationSubscription(ctx, deletedSub.ID)
				require.NoError(t, err)

				return account.ID, nil
			},
			expectedCount: 2,
		},
		{
			name: "no subscriptions for account",
			setup: func(t *testing.T) (uuid.UUID, *integrations.IntegrationType) {
				t.Helper()
				account := f.CreateAccount()
				return account.ID, nil
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			accountID, intType := tt.setup(t)

			subscriptions, err := testDB.Store.GetIntegrationSubscriptionsByAccount(ctx, accountID, intType)
			require.NoError(t, err)
			assert.Len(t, subscriptions, tt.expectedCount)
		})
	}
}

func TestStore_GetActiveIntegrationSubscriptionsForEvent(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	tests := []struct {
		name          string
		setup         func(t *testing.T) (uuid.UUID, string, *uuid.UUID)
		expectedCount int
	}{
		{
			name: "get subscriptions for event type",
			setup: func(t *testing.T) (uuid.UUID, string, *uuid.UUID) {
				t.Helper()
				account := f.CreateAccount()
				apiKey := createTestAPIKey(t, testDB, account.ID)

				// Create subscriptions for user.created
				for i := 0; i < 2; i++ {
					_, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
						AccountID:       account.ID,
						APIKeyID:        &apiKey.ID,
						IntegrationType: integrations.IntegrationZapier,
						TargetURL:       "https://hooks.zapier.com/" + uuid.New().String(),
						EventType:       "user.created",
					})
					require.NoError(t, err)
				}

				// Create subscription for different event
				_, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
					AccountID:       account.ID,
					APIKeyID:        &apiKey.ID,
					IntegrationType: integrations.IntegrationZapier,
					TargetURL:       "https://hooks.zapier.com/other",
					EventType:       "user.verified",
				})
				require.NoError(t, err)

				return account.ID, "user.created", nil
			},
			expectedCount: 2,
		},
		{
			name: "filter by campaign",
			setup: func(t *testing.T) (uuid.UUID, string, *uuid.UUID) {
				t.Helper()
				account := f.CreateAccount()
				apiKey := createTestAPIKey(t, testDB, account.ID)
				campaign := f.CreateCampaign(func(o *CampaignOpts) {
					o.AccountID = &account.ID
				})

				// Create subscription for specific campaign
				_, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
					AccountID:       account.ID,
					APIKeyID:        &apiKey.ID,
					IntegrationType: integrations.IntegrationZapier,
					TargetURL:       "https://hooks.zapier.com/campaign",
					EventType:       "user.created",
					CampaignID:      &campaign.ID,
				})
				require.NoError(t, err)

				// Create subscription without campaign filter (should also match)
				_, err = testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
					AccountID:       account.ID,
					APIKeyID:        &apiKey.ID,
					IntegrationType: integrations.IntegrationZapier,
					TargetURL:       "https://hooks.zapier.com/all",
					EventType:       "user.created",
				})
				require.NoError(t, err)

				return account.ID, "user.created", &campaign.ID
			},
			expectedCount: 2, // Both campaign-specific and global subscriptions match
		},
		{
			name: "no matching subscriptions",
			setup: func(t *testing.T) (uuid.UUID, string, *uuid.UUID) {
				t.Helper()
				account := f.CreateAccount()
				return account.ID, "nonexistent.event", nil
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			accountID, eventType, campaignID := tt.setup(t)

			subscriptions, err := testDB.Store.GetActiveIntegrationSubscriptionsForEvent(ctx, accountID, eventType, campaignID)
			require.NoError(t, err)
			assert.Len(t, subscriptions, tt.expectedCount)
		})
	}
}

func TestStore_DeleteIntegrationSubscription(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	tests := []struct {
		name    string
		setup   func(t *testing.T) uuid.UUID
		wantErr bool
		errType error
	}{
		{
			name: "delete existing subscription",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				account := f.CreateAccount()
				apiKey := createTestAPIKey(t, testDB, account.ID)

				sub, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
					AccountID:       account.ID,
					APIKeyID:        &apiKey.ID,
					IntegrationType: integrations.IntegrationZapier,
					TargetURL:       "https://hooks.zapier.com/delete",
					EventType:       "user.created",
				})
				require.NoError(t, err)
				return sub.ID
			},
			wantErr: false,
		},
		{
			name: "delete non-existent subscription",
			setup: func(t *testing.T) uuid.UUID {
				return uuid.New()
			},
			wantErr: true,
			errType: ErrNotFound,
		},
		{
			name: "delete already deleted subscription",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				account := f.CreateAccount()
				apiKey := createTestAPIKey(t, testDB, account.ID)

				sub, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
					AccountID:       account.ID,
					APIKeyID:        &apiKey.ID,
					IntegrationType: integrations.IntegrationZapier,
					TargetURL:       "https://hooks.zapier.com/double-delete",
					EventType:       "user.created",
				})
				require.NoError(t, err)

				// Delete it first time
				err = testDB.Store.DeleteIntegrationSubscription(ctx, sub.ID)
				require.NoError(t, err)

				return sub.ID
			},
			wantErr: true,
			errType: ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			subID := tt.setup(t)

			err := testDB.Store.DeleteIntegrationSubscription(ctx, subID)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					assert.True(t, errors.Is(err, tt.errType))
				}
				return
			}

			require.NoError(t, err)

			// Verify subscription is deleted (not found)
			_, err = testDB.Store.GetIntegrationSubscriptionByID(ctx, subID)
			assert.True(t, errors.Is(err, ErrNotFound))
		})
	}
}

func TestStore_DeleteIntegrationSubscriptionsByAPIKey(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("delete all subscriptions for API key", func(t *testing.T) {
		t.Parallel()
		account := f.CreateAccount()

		// Create API key 1 with subscriptions
		apiKey1 := createTestAPIKey(t, testDB, account.ID)
		for i := 0; i < 3; i++ {
			_, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
				AccountID:       account.ID,
				APIKeyID:        &apiKey1.ID,
				IntegrationType: integrations.IntegrationZapier,
				TargetURL:       "https://hooks.zapier.com/key1/" + uuid.New().String(),
				EventType:       "user.created",
			})
			require.NoError(t, err)
		}

		// Create API key 2 with subscriptions (should not be deleted)
		apiKey2 := createTestAPIKey(t, testDB, account.ID)
		_, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
			AccountID:       account.ID,
			APIKeyID:        &apiKey2.ID,
			IntegrationType: integrations.IntegrationZapier,
			TargetURL:       "https://hooks.zapier.com/key2/1",
			EventType:       "user.created",
		})
		require.NoError(t, err)

		// Delete subscriptions for API key 1
		err = testDB.Store.DeleteIntegrationSubscriptionsByAPIKey(ctx, apiKey1.ID)
		require.NoError(t, err)

		// Verify API key 1 subscriptions are deleted
		subs, err := testDB.Store.GetIntegrationSubscriptionsByAccount(ctx, account.ID, nil)
		require.NoError(t, err)
		assert.Len(t, subs, 1) // Only API key 2's subscription remains
		assert.Equal(t, &apiKey2.ID, subs[0].APIKeyID)
	})
}

func TestStore_UpdateIntegrationSubscriptionStats(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("update stats on success", func(t *testing.T) {
		t.Parallel()
		account := f.CreateAccount()
		apiKey := createTestAPIKey(t, testDB, account.ID)

		sub, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
			AccountID:       account.ID,
			APIKeyID:        &apiKey.ID,
			IntegrationType: integrations.IntegrationZapier,
			TargetURL:       "https://hooks.zapier.com/stats",
			EventType:       "user.created",
		})
		require.NoError(t, err)
		assert.Equal(t, 0, sub.TriggerCount)

		// Update with success
		err = testDB.Store.UpdateIntegrationSubscriptionStats(ctx, sub.ID, true, nil)
		require.NoError(t, err)

		// Verify trigger count increased
		updatedSub, err := testDB.Store.GetIntegrationSubscriptionByID(ctx, sub.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, updatedSub.TriggerCount)
		assert.NotNil(t, updatedSub.LastTriggeredAt)
	})

	t.Run("update stats on failure", func(t *testing.T) {
		t.Parallel()
		account := f.CreateAccount()
		apiKey := createTestAPIKey(t, testDB, account.ID)

		sub, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
			AccountID:       account.ID,
			APIKeyID:        &apiKey.ID,
			IntegrationType: integrations.IntegrationZapier,
			TargetURL:       "https://hooks.zapier.com/error",
			EventType:       "user.created",
		})
		require.NoError(t, err)

		// Update with failure
		errorMsg := "connection timeout"
		err = testDB.Store.UpdateIntegrationSubscriptionStats(ctx, sub.ID, false, &errorMsg)
		require.NoError(t, err)

		// Verify error count and message
		updatedSub, err := testDB.Store.GetIntegrationSubscriptionByID(ctx, sub.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, updatedSub.ErrorCount)
		assert.Equal(t, &errorMsg, updatedSub.LastError)
		assert.Equal(t, 0, updatedSub.TriggerCount) // Trigger count should not increase
	})
}

// ===== Delivery Store Tests =====

func TestStore_CreateDelivery(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("create delivery successfully", func(t *testing.T) {
		t.Parallel()
		account := f.CreateAccount()
		apiKey := createTestAPIKey(t, testDB, account.ID)

		sub, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
			AccountID:       account.ID,
			APIKeyID:        &apiKey.ID,
			IntegrationType: integrations.IntegrationZapier,
			TargetURL:       "https://hooks.zapier.com/delivery",
			EventType:       "user.created",
		})
		require.NoError(t, err)

		delivery, err := testDB.Store.CreateDelivery(ctx, integrations.CreateDeliveryParams{
			SubscriptionID: sub.ID,
			EventType:      "user.created",
			Status:         integrations.DeliveryStatusPending,
		})
		require.NoError(t, err)

		assert.NotEqual(t, uuid.Nil, delivery.ID)
		assert.Equal(t, sub.ID, delivery.SubscriptionID)
		assert.Equal(t, "user.created", delivery.EventType)
		assert.Equal(t, integrations.DeliveryStatusPending, delivery.Status)
	})
}

func TestStore_UpdateDeliveryStatus(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("update delivery to success", func(t *testing.T) {
		t.Parallel()
		account := f.CreateAccount()
		apiKey := createTestAPIKey(t, testDB, account.ID)

		sub, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
			AccountID:       account.ID,
			APIKeyID:        &apiKey.ID,
			IntegrationType: integrations.IntegrationZapier,
			TargetURL:       "https://hooks.zapier.com/update",
			EventType:       "user.created",
		})
		require.NoError(t, err)

		delivery, err := testDB.Store.CreateDelivery(ctx, integrations.CreateDeliveryParams{
			SubscriptionID: sub.ID,
			EventType:      "user.created",
			Status:         integrations.DeliveryStatusPending,
		})
		require.NoError(t, err)

		responseStatus := 200
		durationMs := 150
		err = testDB.Store.UpdateDeliveryStatus(ctx, delivery.ID, integrations.DeliveryStatusSuccess, &responseStatus, &durationMs, nil)
		require.NoError(t, err)
	})

	t.Run("update delivery to failed", func(t *testing.T) {
		t.Parallel()
		account := f.CreateAccount()
		apiKey := createTestAPIKey(t, testDB, account.ID)

		sub, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
			AccountID:       account.ID,
			APIKeyID:        &apiKey.ID,
			IntegrationType: integrations.IntegrationZapier,
			TargetURL:       "https://hooks.zapier.com/failed",
			EventType:       "user.created",
		})
		require.NoError(t, err)

		delivery, err := testDB.Store.CreateDelivery(ctx, integrations.CreateDeliveryParams{
			SubscriptionID: sub.ID,
			EventType:      "user.created",
			Status:         integrations.DeliveryStatusPending,
		})
		require.NoError(t, err)

		responseStatus := 500
		durationMs := 5000
		errorMsg := "Internal Server Error"
		err = testDB.Store.UpdateDeliveryStatus(ctx, delivery.ID, integrations.DeliveryStatusFailed, &responseStatus, &durationMs, &errorMsg)
		require.NoError(t, err)
	})
}

func TestStore_GetDeliveriesBySubscription(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("get deliveries with pagination", func(t *testing.T) {
		t.Parallel()
		account := f.CreateAccount()
		apiKey := createTestAPIKey(t, testDB, account.ID)

		sub, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
			AccountID:       account.ID,
			APIKeyID:        &apiKey.ID,
			IntegrationType: integrations.IntegrationZapier,
			TargetURL:       "https://hooks.zapier.com/paginate",
			EventType:       "user.created",
		})
		require.NoError(t, err)

		// Create 5 deliveries
		for i := 0; i < 5; i++ {
			_, err := testDB.Store.CreateDelivery(ctx, integrations.CreateDeliveryParams{
				SubscriptionID: sub.ID,
				EventType:      "user.created",
				Status:         integrations.DeliveryStatusSuccess,
			})
			require.NoError(t, err)
		}

		// Get first page
		deliveries, err := testDB.Store.GetDeliveriesBySubscription(ctx, sub.ID, 3, 0)
		require.NoError(t, err)
		assert.Len(t, deliveries, 3)

		// Get second page
		deliveries, err = testDB.Store.GetDeliveriesBySubscription(ctx, sub.ID, 3, 3)
		require.NoError(t, err)
		assert.Len(t, deliveries, 2)
	})
}

func TestStore_GetDeliveriesByAccount(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)
	ctx := context.Background()
	f := NewFixtures(t, testDB)

	t.Run("get all deliveries for account", func(t *testing.T) {
		t.Parallel()
		account := f.CreateAccount()
		apiKey := createTestAPIKey(t, testDB, account.ID)

		// Create 2 subscriptions
		sub1, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
			AccountID:       account.ID,
			APIKeyID:        &apiKey.ID,
			IntegrationType: integrations.IntegrationZapier,
			TargetURL:       "https://hooks.zapier.com/sub1",
			EventType:       "user.created",
		})
		require.NoError(t, err)

		sub2, err := testDB.Store.CreateIntegrationSubscription(ctx, integrations.CreateSubscriptionParams{
			AccountID:       account.ID,
			APIKeyID:        &apiKey.ID,
			IntegrationType: integrations.IntegrationZapier,
			TargetURL:       "https://hooks.zapier.com/sub2",
			EventType:       "user.verified",
		})
		require.NoError(t, err)

		// Create deliveries for both subscriptions
		for i := 0; i < 2; i++ {
			_, err := testDB.Store.CreateDelivery(ctx, integrations.CreateDeliveryParams{
				SubscriptionID: sub1.ID,
				EventType:      "user.created",
				Status:         integrations.DeliveryStatusSuccess,
			})
			require.NoError(t, err)

			_, err = testDB.Store.CreateDelivery(ctx, integrations.CreateDeliveryParams{
				SubscriptionID: sub2.ID,
				EventType:      "user.verified",
				Status:         integrations.DeliveryStatusSuccess,
			})
			require.NoError(t, err)
		}

		// Get all deliveries for account
		deliveries, err := testDB.Store.GetDeliveriesByAccount(ctx, account.ID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, deliveries, 4)
	})
}
