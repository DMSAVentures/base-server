package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestStore_CreateSubscription(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

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
				prodStripeID := "prod_" + uuid.New().String()
				priceStripeID := "price_" + uuid.New().String()
				subStripeID := "sub_" + uuid.New().String()

				product, _ := testDB.Store.CreateProduct(ctx, prodStripeID, "Test Product", "A test product")
				price := Price{
					ProductID:   product.ID,
					StripeID:    priceStripeID,
					Description: "Test Price",
				}
				testDB.Store.CreatePrice(ctx, price)
				priceRecord, _ := testDB.Store.GetPriceByStripeID(ctx, priceStripeID)

				return CreateSubscriptionsParams{
					UserID:          user.ID,
					PriceID:         priceRecord.ID,
					StripeID:        subStripeID,
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
				prodStripeID := "prod_" + uuid.New().String()
				priceStripeID := "price_" + uuid.New().String()
				subStripeID := "sub_" + uuid.New().String()

				product, _ := testDB.Store.CreateProduct(ctx, prodStripeID, "Trial Product", "A trial product")
				price := Price{
					ProductID:   product.ID,
					StripeID:    priceStripeID,
					Description: "Trial Price",
				}
				testDB.Store.CreatePrice(ctx, price)
				priceRecord, _ := testDB.Store.GetPriceByStripeID(ctx, priceStripeID)

				return CreateSubscriptionsParams{
					UserID:          user.ID,
					PriceID:         priceRecord.ID,
					StripeID:        subStripeID,
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
			t.Parallel()
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
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) string
		wantErr  bool
		validate func(t *testing.T, sub Subscription, expectedStripeID string)
	}{
		{
			name: "get existing subscription",
			setup: func(t *testing.T) string {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Sub", "User")
				prodStripeID := "prod_" + uuid.New().String()
				priceStripeID := "price_" + uuid.New().String()
				subStripeID := "sub_" + uuid.New().String()

				product, _ := testDB.Store.CreateProduct(ctx, prodStripeID, "Get Product", "Product for get test")
				price := Price{
					ProductID:   product.ID,
					StripeID:    priceStripeID,
					Description: "Get Price",
				}
				testDB.Store.CreatePrice(ctx, price)
				priceRecord, _ := testDB.Store.GetPriceByStripeID(ctx, priceStripeID)

				params := CreateSubscriptionsParams{
					UserID:          user.ID,
					PriceID:         priceRecord.ID,
					StripeID:        subStripeID,
					Status:          "active",
					StartDate:       time.Now(),
					EndDate:         time.Now().AddDate(1, 0, 0),
					NextBillingDate: time.Now().AddDate(0, 1, 0),
				}
				testDB.Store.CreateSubscription(ctx, params)
				return subStripeID
			},
			wantErr: false,
			validate: func(t *testing.T, sub Subscription, expectedStripeID string) {
				t.Helper()
				if sub.StripeID != expectedStripeID {
					t.Errorf("StripeID = %v, want %v", sub.StripeID, expectedStripeID)
				}
				if sub.Status != "active" {
					t.Errorf("Status = %v, want active", sub.Status)
				}
			},
		},
		{
			name: "subscription does not exist",
			setup: func(t *testing.T) string {
				return "sub_" + uuid.New().String()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stripeID := tt.setup(t)

			sub, err := testDB.Store.GetSubscription(ctx, stripeID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSubscription() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, sub, stripeID)
			}
		})
	}
}

func TestStore_GetSubscriptionByUserID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

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
				prodStripeID := "prod_" + uuid.New().String()
				priceStripeID := "price_" + uuid.New().String()
				subStripeID := "sub_" + uuid.New().String()

				product, _ := testDB.Store.CreateProduct(ctx, prodStripeID, "User Product", "Product for user")
				price := Price{
					ProductID:   product.ID,
					StripeID:    priceStripeID,
					Description: "User Price",
				}
				testDB.Store.CreatePrice(ctx, price)
				priceRecord, _ := testDB.Store.GetPriceByStripeID(ctx, priceStripeID)

				params := CreateSubscriptionsParams{
					UserID:          user.ID,
					PriceID:         priceRecord.ID,
					StripeID:        subStripeID,
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
			t.Parallel()
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
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

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
				prodStripeID := "prod_" + uuid.New().String()
				priceStripeID := "price_" + uuid.New().String()
				subStripeID := "sub_" + uuid.New().String()

				product, _ := testDB.Store.CreateProduct(ctx, prodStripeID, "Update Product", "Product for update")
				price := Price{
					ProductID:   product.ID,
					StripeID:    priceStripeID,
					Description: "Update Price",
				}
				testDB.Store.CreatePrice(ctx, price)
				priceRecord, _ := testDB.Store.GetPriceByStripeID(ctx, priceStripeID)

				params := CreateSubscriptionsParams{
					UserID:          user.ID,
					PriceID:         priceRecord.ID,
					StripeID:        subStripeID,
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
					StripeID:        subStripeID,
					StripePriceID:   priceStripeID,
				}
				return subStripeID, updateParams
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
				prodStripeID := "prod_" + uuid.New().String()
				oldPriceStripeID := "price_" + uuid.New().String()
				newPriceStripeID := "price_" + uuid.New().String()
				subStripeID := "sub_" + uuid.New().String()

				product, _ := testDB.Store.CreateProduct(ctx, prodStripeID, "Price Product", "Product for price test")

				// Old price
				oldPrice := Price{
					ProductID:   product.ID,
					StripeID:    oldPriceStripeID,
					Description: "Old Price",
				}
				testDB.Store.CreatePrice(ctx, oldPrice)
				oldPriceRecord, _ := testDB.Store.GetPriceByStripeID(ctx, oldPriceStripeID)

				// New price
				newPrice := Price{
					ProductID:   product.ID,
					StripeID:    newPriceStripeID,
					Description: "New Price",
				}
				testDB.Store.CreatePrice(ctx, newPrice)

				params := CreateSubscriptionsParams{
					UserID:          user.ID,
					PriceID:         oldPriceRecord.ID,
					StripeID:        subStripeID,
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
					StripeID:        subStripeID,
					StripePriceID:   newPriceStripeID,
				}
				return subStripeID, updateParams
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
				prodStripeID := "prod_" + uuid.New().String()
				priceStripeID := "price_" + uuid.New().String()
				subStripeID := "sub_" + uuid.New().String()

				// Create a price for the update params
				product, _ := testDB.Store.CreateProduct(ctx, prodStripeID, "Fake Product", "Fake product")
				price := Price{
					ProductID:   product.ID,
					StripeID:    priceStripeID,
					Description: "Fake Price",
				}
				testDB.Store.CreatePrice(ctx, price)

				updateParams := UpdateSubscriptionParams{
					Status:          "active",
					EndDate:         time.Now().AddDate(1, 0, 0),
					NextBillingDate: time.Now().AddDate(0, 1, 0),
					StripeID:        subStripeID,
					StripePriceID:   priceStripeID,
				}
				return subStripeID, updateParams
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

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
				prodStripeID := "prod_" + uuid.New().String()
				priceStripeID := "price_" + uuid.New().String()
				subStripeID := "sub_" + uuid.New().String()

				product, _ := testDB.Store.CreateProduct(ctx, prodStripeID, "Cancel Product", "Product for cancel")
				price := Price{
					ProductID:   product.ID,
					StripeID:    priceStripeID,
					Description: "Cancel Price",
				}
				testDB.Store.CreatePrice(ctx, price)
				priceRecord, _ := testDB.Store.GetPriceByStripeID(ctx, priceStripeID)

				params := CreateSubscriptionsParams{
					UserID:          user.ID,
					PriceID:         priceRecord.ID,
					StripeID:        subStripeID,
					Status:          "active",
					StartDate:       time.Now(),
					EndDate:         time.Now().AddDate(1, 0, 0),
					NextBillingDate: time.Now().AddDate(0, 1, 0),
				}
				testDB.Store.CreateSubscription(ctx, params)

				cancelAt := time.Now().AddDate(0, 0, 7) // Cancel in 7 days
				return subStripeID, cancelAt
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
			t.Parallel()
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
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	user, _ := createTestUser(t, testDB, "No", "Subscription")

	_, err := testDB.Store.GetSubscriptionByUserID(ctx, user.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
