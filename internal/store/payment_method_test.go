package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestStore_CreatePaymentMethod(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("create payment method successfully", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "John", "Doe")
		params := CreatePaymentMethodParams{
			UserID:       user.ID,
			StripeID:     "pm_" + uuid.New().String(),
			CardBrand:    "visa",
			CardLast4:    "4242",
			CardExpMonth: 12,
			CardExpYear:  2025,
		}

		pm, err := testDB.Store.CreatePaymentMethod(ctx, params)
		if err != nil {
			t.Errorf("CreatePaymentMethod() error = %v", err)
			return
		}
		if pm.UserID != params.UserID {
			t.Errorf("UserID = %v, want %v", pm.UserID, params.UserID)
		}
		if pm.StripeID != params.StripeID {
			t.Errorf("StripeID = %v, want %v", pm.StripeID, params.StripeID)
		}
		if pm.CardBrand != params.CardBrand {
			t.Errorf("CardBrand = %v, want %v", pm.CardBrand, params.CardBrand)
		}
		if pm.CardLast4 != params.CardLast4 {
			t.Errorf("CardLast4 = %v, want %v", pm.CardLast4, params.CardLast4)
		}
	})

	t.Run("create payment method with different card brand", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "Jane", "Smith")
		params := CreatePaymentMethodParams{
			UserID:       user.ID,
			StripeID:     "pm_" + uuid.New().String(),
			CardBrand:    "mastercard",
			CardLast4:    "5555",
			CardExpMonth: 6,
			CardExpYear:  2026,
		}

		pm, err := testDB.Store.CreatePaymentMethod(ctx, params)
		if err != nil {
			t.Errorf("CreatePaymentMethod() error = %v", err)
			return
		}
		if pm.CardBrand != "mastercard" {
			t.Errorf("CardBrand = %v, want mastercard", pm.CardBrand)
		}
	})
}

func TestStore_GetPaymentMethodByUserID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("get existing payment method", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "John", "Doe")
		stripeID := "pm_" + uuid.New().String()
		_, err := testDB.Store.CreatePaymentMethod(ctx, CreatePaymentMethodParams{
			UserID:       user.ID,
			StripeID:     stripeID,
			CardBrand:    "visa",
			CardLast4:    "4242",
			CardExpMonth: 12,
			CardExpYear:  2025,
		})
		if err != nil {
			t.Fatalf("failed to create payment method: %v", err)
		}

		pm, err := testDB.Store.GetPaymentMethodByUserID(ctx, user.ID)
		if err != nil {
			t.Errorf("GetPaymentMethodByUserID() error = %v", err)
			return
		}
		if pm.StripeID != stripeID {
			t.Errorf("StripeID = %v, want %v", pm.StripeID, stripeID)
		}
	})

	t.Run("payment method does not exist", func(t *testing.T) {
		t.Parallel()
		_, err := testDB.Store.GetPaymentMethodByUserID(ctx, uuid.New())
		if err == nil {
			t.Error("GetPaymentMethodByUserID() expected error for non-existent user")
		}
	})
}

func TestStore_UpdatePaymentMethodByUserID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("update existing payment method", func(t *testing.T) {
		t.Parallel()
		user, _ := createTestUser(t, testDB, "John", "Doe")
		_, err := testDB.Store.CreatePaymentMethod(ctx, CreatePaymentMethodParams{
			UserID:       user.ID,
			StripeID:     "pm_" + uuid.New().String(),
			CardBrand:    "visa",
			CardLast4:    "1111",
			CardExpMonth: 1,
			CardExpYear:  2024,
		})
		if err != nil {
			t.Fatalf("failed to create payment method: %v", err)
		}

		newStripeID := "pm_" + uuid.New().String()
		err = testDB.Store.UpdatePaymentMethodByUserID(ctx, user.ID, newStripeID, "mastercard", "2222", 12, 2026)
		if err != nil {
			t.Errorf("UpdatePaymentMethodByUserID() error = %v", err)
			return
		}

		// Verify the update
		pm, err := testDB.Store.GetPaymentMethodByUserID(ctx, user.ID)
		if err != nil {
			t.Errorf("failed to get updated payment method: %v", err)
			return
		}
		if pm.StripeID != newStripeID {
			t.Errorf("StripeID = %v, want %v", pm.StripeID, newStripeID)
		}
		if pm.CardBrand != "mastercard" {
			t.Errorf("CardBrand = %v, want mastercard", pm.CardBrand)
		}
		if pm.CardLast4 != "2222" {
			t.Errorf("CardLast4 = %v, want 2222", pm.CardLast4)
		}
	})

	t.Run("update non-existent payment method", func(t *testing.T) {
		t.Parallel()
		err := testDB.Store.UpdatePaymentMethodByUserID(ctx, uuid.New(), "pm_"+uuid.New().String(), "visa", "4242", 12, 2025)
		if err == nil {
			t.Error("UpdatePaymentMethodByUserID() expected error for non-existent user")
		}
	})
}
