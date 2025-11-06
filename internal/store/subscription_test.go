package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestStore_CreateSubscription(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func(t *testing.T) CreateSubscriptionsParams
		wantErr bool
	}{
		{
			name: "create subscription successfully",
			setup: func(t *testing.T) CreateSubscriptionsParams {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Sub", "User")
				product, _ := testDB.Store.CreateProduct(ctx, "prod_test123", "Test Product", "A test product")
				price := Price{
					ProductID:   product.ID,
					StripeID:    "price_test123",
					Description: "Test Price",
				}
				testDB.Store.CreatePrice(ctx, price)
				priceRecord, _ := testDB.Store.GetPriceByStripeID(ctx, "price_test123")

				return CreateSubscriptionsParams{
					UserID:          user.ID,
					PriceID:         priceRecord.ID,
					StripeID:        "sub_test123",
					Status:          "active",
					StartDate:       time.Now(),
					EndDate:         time.Now().AddDate(1, 0, 0),
					NextBillingDate: time.Now().AddDate(0, 1, 0),
				}
			},
			wantErr: false,
		},
		{
			name: "create subscription with trial period",
			setup: func(t *testing.T) CreateSubscriptionsParams {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Trial", "User")
				product, _ := testDB.Store.CreateProduct(ctx, "prod_trial123", "Trial Product", "A trial product")
				price := Price{
					ProductID:   product.ID,
					StripeID:    "price_trial123",
					Description: "Trial Price",
				}
				testDB.Store.CreatePrice(ctx, price)
				priceRecord, _ := testDB.Store.GetPriceByStripeID(ctx, "price_trial123")

				return CreateSubscriptionsParams{
					UserID:          user.ID,
					PriceID:         priceRecord.ID,
					StripeID:        "sub_trial123",
					Status:          "trialing",
					StartDate:       time.Now(),
					EndDate:         time.Now().AddDate(1, 0, 0),
					NextBillingDate: time.Now().AddDate(0, 0, 14), // 14-day trial
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			params := tt.setup(t)

			err := testDB.Store.CreateSubscription(ctx, params)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSubscription() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify subscription was created
				sub, err := testDB.Store.GetSubscription(ctx, params.StripeID)
				if err != nil {
					t.Errorf("failed to get subscription: %v", err)
					return
				}
				if sub.UserID != params.UserID {
					t.Errorf("UserID = %v, want %v", sub.UserID, params.UserID)
				}
				if sub.Status != params.Status {
					t.Errorf("Status = %v, want %v", sub.Status, params.Status)
				}
			}
		})
	}
}

func TestStore_GetSubscription(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) string
		wantErr  bool
		validate func(t *testing.T, sub Subscription)
	}{
		{
			name: "get existing subscription",
			setup: func(t *testing.T) string {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Sub", "User")
				product, _ := testDB.Store.CreateProduct(ctx, "prod_get123", "Get Product", "Product for get test")
				price := Price{
					ProductID:   product.ID,
					StripeID:    "price_get123",
					Description: "Get Price",
				}
				testDB.Store.CreatePrice(ctx, price)
				priceRecord, _ := testDB.Store.GetPriceByStripeID(ctx, "price_get123")

				params := CreateSubscriptionsParams{
					UserID:          user.ID,
					PriceID:         priceRecord.ID,
					StripeID:        "sub_get123",
					Status:          "active",
					StartDate:       time.Now(),
					EndDate:         time.Now().AddDate(1, 0, 0),
					NextBillingDate: time.Now().AddDate(0, 1, 0),
				}
				testDB.Store.CreateSubscription(ctx, params)
				return "sub_get123"
			},
			wantErr: false,
			validate: func(t *testing.T, sub Subscription) {
				t.Helper()
				if sub.StripeID != "sub_get123" {
					t.Errorf("StripeID = %v, want sub_get123", sub.StripeID)
				}
				if sub.Status != "active" {
					t.Errorf("Status = %v, want active", sub.Status)
				}
			},
		},
		{
			name: "subscription does not exist",
			setup: func(t *testing.T) string {
				return "sub_nonexistent"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			stripeID := tt.setup(t)

			sub, err := testDB.Store.GetSubscription(ctx, stripeID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSubscription() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, sub)
			}
		})
	}
}

func TestStore_GetSubscriptionByUserID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) uuid.UUID
		wantErr  bool
		validate func(t *testing.T, sub Subscription, userID uuid.UUID)
	}{
		{
			name: "get subscription for user",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "User", "WithSub")
				product, _ := testDB.Store.CreateProduct(ctx, "prod_user123", "User Product", "Product for user")
				price := Price{
					ProductID:   product.ID,
					StripeID:    "price_user123",
					Description: "User Price",
				}
				testDB.Store.CreatePrice(ctx, price)
				priceRecord, _ := testDB.Store.GetPriceByStripeID(ctx, "price_user123")

				params := CreateSubscriptionsParams{
					UserID:          user.ID,
					PriceID:         priceRecord.ID,
					StripeID:        "sub_user123",
					Status:          "active",
					StartDate:       time.Now(),
					EndDate:         time.Now().AddDate(1, 0, 0),
					NextBillingDate: time.Now().AddDate(0, 1, 0),
				}
				testDB.Store.CreateSubscription(ctx, params)
				return user.ID
			},
			wantErr: false,
			validate: func(t *testing.T, sub Subscription, userID uuid.UUID) {
				t.Helper()
				if sub.UserID != userID {
					t.Errorf("UserID = %v, want %v", sub.UserID, userID)
				}
			},
		},
		{
			name: "user has no subscription",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "No", "Sub")
				return user.ID
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			userID := tt.setup(t)

			sub, err := testDB.Store.GetSubscriptionByUserID(ctx, userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSubscriptionByUserID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, sub, userID)
			}
		})
	}
}

func TestStore_UpdateSubscription(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) (string, UpdateSubscriptionParams)
		wantErr  bool
		validate func(t *testing.T, stripeID string, params UpdateSubscriptionParams)
	}{
		{
			name: "update subscription status",
			setup: func(t *testing.T) (string, UpdateSubscriptionParams) {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Update", "User")
				product, _ := testDB.Store.CreateProduct(ctx, "prod_update123", "Update Product", "Product for update")
				price := Price{
					ProductID:   product.ID,
					StripeID:    "price_update123",
					Description: "Update Price",
				}
				testDB.Store.CreatePrice(ctx, price)
				priceRecord, _ := testDB.Store.GetPriceByStripeID(ctx, "price_update123")

				params := CreateSubscriptionsParams{
					UserID:          user.ID,
					PriceID:         priceRecord.ID,
					StripeID:        "sub_update123",
					Status:          "active",
					StartDate:       time.Now(),
					EndDate:         time.Now().AddDate(1, 0, 0),
					NextBillingDate: time.Now().AddDate(0, 1, 0),
				}
				testDB.Store.CreateSubscription(ctx, params)

				updateParams := UpdateSubscriptionParams{
					Status:          "past_due",
					EndDate:         time.Now().AddDate(1, 0, 0),
					NextBillingDate: time.Now().AddDate(0, 1, 0),
					StripeID:        "sub_update123",
					StripePriceID:   "price_update123",
				}
				return "sub_update123", updateParams
			},
			wantErr: false,
			validate: func(t *testing.T, stripeID string, params UpdateSubscriptionParams) {
				t.Helper()
				sub, err := testDB.Store.GetSubscription(ctx, stripeID)
				if err != nil {
					t.Errorf("failed to get subscription: %v", err)
					return
				}
				if sub.Status != params.Status {
					t.Errorf("Status = %v, want %v", sub.Status, params.Status)
				}
			},
		},
		{
			name: "update subscription price",
			setup: func(t *testing.T) (string, UpdateSubscriptionParams) {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Price", "User")
				product, _ := testDB.Store.CreateProduct(ctx, "prod_price123", "Price Product", "Product for price test")

				// Old price
				oldPrice := Price{
					ProductID:   product.ID,
					StripeID:    "price_old123",
					Description: "Old Price",
				}
				testDB.Store.CreatePrice(ctx, oldPrice)
				oldPriceRecord, _ := testDB.Store.GetPriceByStripeID(ctx, "price_old123")

				// New price
				newPrice := Price{
					ProductID:   product.ID,
					StripeID:    "price_new123",
					Description: "New Price",
				}
				testDB.Store.CreatePrice(ctx, newPrice)

				params := CreateSubscriptionsParams{
					UserID:          user.ID,
					PriceID:         oldPriceRecord.ID,
					StripeID:        "sub_price123",
					Status:          "active",
					StartDate:       time.Now(),
					EndDate:         time.Now().AddDate(1, 0, 0),
					NextBillingDate: time.Now().AddDate(0, 1, 0),
				}
				testDB.Store.CreateSubscription(ctx, params)

				updateParams := UpdateSubscriptionParams{
					Status:          "active",
					EndDate:         time.Now().AddDate(1, 0, 0),
					NextBillingDate: time.Now().AddDate(0, 1, 0),
					StripeID:        "sub_price123",
					StripePriceID:   "price_new123",
				}
				return "sub_price123", updateParams
			},
			wantErr: false,
			validate: func(t *testing.T, stripeID string, params UpdateSubscriptionParams) {
				t.Helper()
				sub, err := testDB.Store.GetSubscription(ctx, stripeID)
				if err != nil {
					t.Errorf("failed to get subscription: %v", err)
					return
				}
				// Verify price was updated
				newPriceRecord, _ := testDB.Store.GetPriceByStripeID(ctx, params.StripePriceID)
				if sub.PriceID != newPriceRecord.ID {
					t.Errorf("PriceID = %v, want %v", sub.PriceID, newPriceRecord.ID)
				}
			},
		},
		{
			name: "update non-existent subscription",
			setup: func(t *testing.T) (string, UpdateSubscriptionParams) {
				t.Helper()
				// Create a price for the update params
				product, _ := testDB.Store.CreateProduct(ctx, "prod_fake123", "Fake Product", "Fake product")
				price := Price{
					ProductID:   product.ID,
					StripeID:    "price_fake123",
					Description: "Fake Price",
				}
				testDB.Store.CreatePrice(ctx, price)

				updateParams := UpdateSubscriptionParams{
					Status:          "active",
					EndDate:         time.Now().AddDate(1, 0, 0),
					NextBillingDate: time.Now().AddDate(0, 1, 0),
					StripeID:        "sub_nonexistent",
					StripePriceID:   "price_fake123",
				}
				return "sub_nonexistent", updateParams
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			stripeID, params := tt.setup(t)

			err := testDB.Store.UpdateSubscription(ctx, params)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateSubscription() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, stripeID, params)
			}
		})
	}
}

func TestStore_CancelSubscription(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) (string, time.Time)
		wantErr  bool
		validate func(t *testing.T, stripeID string, cancelAt time.Time)
	}{
		{
			name: "cancel subscription",
			setup: func(t *testing.T) (string, time.Time) {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Cancel", "User")
				product, _ := testDB.Store.CreateProduct(ctx, "prod_cancel123", "Cancel Product", "Product for cancel")
				price := Price{
					ProductID:   product.ID,
					StripeID:    "price_cancel123",
					Description: "Cancel Price",
				}
				testDB.Store.CreatePrice(ctx, price)
				priceRecord, _ := testDB.Store.GetPriceByStripeID(ctx, "price_cancel123")

				params := CreateSubscriptionsParams{
					UserID:          user.ID,
					PriceID:         priceRecord.ID,
					StripeID:        "sub_cancel123",
					Status:          "active",
					StartDate:       time.Now(),
					EndDate:         time.Now().AddDate(1, 0, 0),
					NextBillingDate: time.Now().AddDate(0, 1, 0),
				}
				testDB.Store.CreateSubscription(ctx, params)

				cancelAt := time.Now().AddDate(0, 0, 7) // Cancel in 7 days
				return "sub_cancel123", cancelAt
			},
			wantErr: false,
			validate: func(t *testing.T, stripeID string, cancelAt time.Time) {
				t.Helper()
				sub, err := testDB.Store.GetSubscription(ctx, stripeID)
				if err != nil {
					t.Errorf("failed to get subscription: %v", err)
					return
				}
				if sub.Status != "canceled" {
					t.Errorf("Status = %v, want canceled", sub.Status)
				}
				// Verify end date was updated (within 1 second tolerance)
				if sub.EndDate.Unix() < cancelAt.Unix()-1 || sub.EndDate.Unix() > cancelAt.Unix()+1 {
					t.Errorf("EndDate = %v, want around %v", sub.EndDate, cancelAt)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			stripeID, cancelAt := tt.setup(t)

			err := testDB.Store.CancelSubscription(ctx, stripeID, cancelAt)
			if (err != nil) != tt.wantErr {
				t.Errorf("CancelSubscription() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, stripeID, cancelAt)
			}
		})
	}
}

func TestStore_GetSubscriptionByUserID_NotFound(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	user, _ := createTestUser(t, testDB, "No", "Subscription")

	_, err := testDB.Store.GetSubscriptionByUserID(ctx, user.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
