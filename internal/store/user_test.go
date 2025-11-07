package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestStore_GetUserByExternalID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) uuid.UUID
		wantErr  bool
		validate func(t *testing.T, user User, expectedID uuid.UUID)
	}{
		{
			name: "get existing user",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "John", "Doe")
				return user.ID
			},
			wantErr: false,
			validate: func(t *testing.T, user User, expectedID uuid.UUID) {
				t.Helper()
				if user.ID != expectedID {
					t.Errorf("ID = %v, want %v", user.ID, expectedID)
				}
				if user.FirstName != "John" {
					t.Errorf("FirstName = %v, want John", user.FirstName)
				}
				if user.LastName != "Doe" {
					t.Errorf("LastName = %v, want Doe", user.LastName)
				}
			},
		},
		{
			name: "user does not exist",
			setup: func(t *testing.T) uuid.UUID {
				return uuid.New() // Random UUID that doesn't exist
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			expectedID := tt.setup(t)

			user, err := testDB.Store.GetUserByExternalID(ctx, expectedID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserByExternalID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, user, expectedID)
			}
		})
	}
}

func TestStore_UpdateStripeCustomerIDByUserID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name             string
		setup            func(t *testing.T) uuid.UUID
		stripeCustomerID string
		wantErr          bool
	}{
		{
			name: "update stripe customer ID for existing user",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Test", "User")
				return user.ID
			},
			stripeCustomerID: "cus_test123",
			wantErr:          false,
		},
		{
			name: "update stripe customer ID to different value",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Another", "User")
				// Set initial stripe customer ID
				testDB.Store.UpdateStripeCustomerIDByUserID(ctx, user.ID, "cus_old123")
				return user.ID
			},
			stripeCustomerID: "cus_new456",
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			userID := tt.setup(t)

			err := testDB.Store.UpdateStripeCustomerIDByUserID(ctx, userID, tt.stripeCustomerID)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateStripeCustomerIDByUserID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the update
				stripeID, err := testDB.Store.GetStripeCustomerIDByUserExternalID(ctx, userID)
				if err != nil {
					t.Errorf("failed to get stripe customer ID: %v", err)
					return
				}
				if stripeID != tt.stripeCustomerID {
					t.Errorf("StripeCustomerID = %v, want %v", stripeID, tt.stripeCustomerID)
				}
			}
		})
	}
}

func TestStore_GetStripeCustomerIDByUserExternalID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name         string
		setup        func(t *testing.T) uuid.UUID
		wantStripeID string
		wantErr      bool
	}{
		{
			name: "get stripe customer ID for user",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Stripe", "User")
				testDB.Store.UpdateStripeCustomerIDByUserID(ctx, user.ID, "cus_stripe123")
				return user.ID
			},
			wantStripeID: "cus_stripe123",
			wantErr:      false,
		},
		{
			name: "user without stripe customer ID",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "NoStripe", "User")
				return user.ID
			},
			wantStripeID: "",
			wantErr:      true, // NULL value will cause error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			userID := tt.setup(t)

			stripeID, err := testDB.Store.GetStripeCustomerIDByUserExternalID(ctx, userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetStripeCustomerIDByUserExternalID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && stripeID != tt.wantStripeID {
				t.Errorf("StripeCustomerID = %v, want %v", stripeID, tt.wantStripeID)
			}
		})
	}
}

func TestStore_GetUserByStripeCustomerID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) (uuid.UUID, string)
		stripeID string
		wantErr  bool
		validate func(t *testing.T, user User, expectedID uuid.UUID)
	}{
		{
			name: "get user by stripe customer ID",
			setup: func(t *testing.T) (uuid.UUID, string) {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Stripe", "Customer")
				stripeID := "cus_findme123"
				testDB.Store.UpdateStripeCustomerIDByUserID(ctx, user.ID, stripeID)
				return user.ID, stripeID
			},
			wantErr: false,
			validate: func(t *testing.T, user User, expectedID uuid.UUID) {
				t.Helper()
				if user.ID != expectedID {
					t.Errorf("ID = %v, want %v", user.ID, expectedID)
				}
				if user.FirstName != "Stripe" {
					t.Errorf("FirstName = %v, want Stripe", user.FirstName)
				}
				if user.LastName != "Customer" {
					t.Errorf("LastName = %v, want Customer", user.LastName)
				}
			},
		},
		{
			name: "stripe customer ID does not exist",
			setup: func(t *testing.T) (uuid.UUID, string) {
				return uuid.Nil, "cus_nonexistent"
			},
			stripeID: "cus_nonexistent",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			expectedID, stripeID := tt.setup(t)
			if tt.stripeID != "" {
				stripeID = tt.stripeID
			}

			user, err := testDB.Store.GetUserByStripeCustomerID(ctx, stripeID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserByStripeCustomerID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, user, expectedID)
			}
		})
	}
}
