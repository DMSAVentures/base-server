package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestStore_GetUserByExternalID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("get existing user", func(t *testing.T) {
		t.Parallel()
		created, _ := createTestUser(t, testDB, "John", "Doe")

		user, err := testDB.Store.GetUserByExternalID(ctx, created.ID)
		if err != nil {
			t.Errorf("GetUserByExternalID() error = %v", err)
			return
		}
		if user.ID != created.ID {
			t.Errorf("ID = %v, want %v", user.ID, created.ID)
		}
		if user.FirstName != "John" {
			t.Errorf("FirstName = %v, want John", user.FirstName)
		}
		if user.LastName != "Doe" {
			t.Errorf("LastName = %v, want Doe", user.LastName)
		}
	})

	t.Run("user does not exist", func(t *testing.T) {
		t.Parallel()
		_, err := testDB.Store.GetUserByExternalID(ctx, uuid.New())
		if err == nil {
			t.Error("GetUserByExternalID() expected error for non-existent user")
		}
	})
}

func TestStore_UpdateStripeCustomerIDByUserID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("update stripe customer ID for existing user", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "Test", "User")
		stripeCustomerID := "cus_" + uuid.New().String()

		err := testDB.Store.UpdateStripeCustomerIDByUserID(ctx, user.ID, stripeCustomerID)
		if err != nil {
			t.Errorf("UpdateStripeCustomerIDByUserID() error = %v", err)
			return
		}

		// Verify the update
		stripeID, err := testDB.Store.GetStripeCustomerIDByUserExternalID(ctx, user.ID)
		if err != nil {
			t.Errorf("failed to get stripe customer ID: %v", err)
			return
		}
		if stripeID != stripeCustomerID {
			t.Errorf("StripeCustomerID = %v, want %v", stripeID, stripeCustomerID)
		}
	})

	t.Run("update stripe customer ID to different value", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "Another", "User")
		// Set initial stripe customer ID
		oldStripeID := "cus_" + uuid.New().String()
		_ = testDB.Store.UpdateStripeCustomerIDByUserID(ctx, user.ID, oldStripeID)

		newStripeID := "cus_" + uuid.New().String()
		err := testDB.Store.UpdateStripeCustomerIDByUserID(ctx, user.ID, newStripeID)
		if err != nil {
			t.Errorf("UpdateStripeCustomerIDByUserID() error = %v", err)
			return
		}

		// Verify the update
		stripeID, err := testDB.Store.GetStripeCustomerIDByUserExternalID(ctx, user.ID)
		if err != nil {
			t.Errorf("failed to get stripe customer ID: %v", err)
			return
		}
		if stripeID != newStripeID {
			t.Errorf("StripeCustomerID = %v, want %v", stripeID, newStripeID)
		}
	})
}

func TestStore_GetStripeCustomerIDByUserExternalID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("get stripe customer ID for user", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "Stripe", "User")
		stripeID := "cus_" + uuid.New().String()
		_ = testDB.Store.UpdateStripeCustomerIDByUserID(ctx, user.ID, stripeID)

		gotStripeID, err := testDB.Store.GetStripeCustomerIDByUserExternalID(ctx, user.ID)
		if err != nil {
			t.Errorf("GetStripeCustomerIDByUserExternalID() error = %v", err)
			return
		}
		if gotStripeID != stripeID {
			t.Errorf("StripeCustomerID = %v, want %v", gotStripeID, stripeID)
		}
	})

	t.Run("user without stripe customer ID", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "NoStripe", "User")

		_, err := testDB.Store.GetStripeCustomerIDByUserExternalID(ctx, user.ID)
		if err == nil {
			t.Error("GetStripeCustomerIDByUserExternalID() expected error for user without stripe ID")
		}
	})
}

func TestStore_GetUserByStripeCustomerID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("get user by stripe customer ID", func(t *testing.T) {
		t.Parallel()
		created, _ := createTestUser(t, testDB, "Stripe", "Customer")
		stripeID := "cus_" + uuid.New().String()
		_ = testDB.Store.UpdateStripeCustomerIDByUserID(ctx, created.ID, stripeID)

		user, err := testDB.Store.GetUserByStripeCustomerID(ctx, stripeID)
		if err != nil {
			t.Errorf("GetUserByStripeCustomerID() error = %v", err)
			return
		}
		if user.ID != created.ID {
			t.Errorf("ID = %v, want %v", user.ID, created.ID)
		}
		if user.FirstName != "Stripe" {
			t.Errorf("FirstName = %v, want Stripe", user.FirstName)
		}
		if user.LastName != "Customer" {
			t.Errorf("LastName = %v, want Customer", user.LastName)
		}
	})

	t.Run("stripe customer ID does not exist", func(t *testing.T) {
		t.Parallel()
		_, err := testDB.Store.GetUserByStripeCustomerID(ctx, "cus_nonexistent_"+uuid.New().String())
		if err == nil {
			t.Error("GetUserByStripeCustomerID() expected error for non-existent stripe ID")
		}
	})
}
