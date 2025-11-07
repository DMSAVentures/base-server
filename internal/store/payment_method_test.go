package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestStore_CreatePaymentMethod(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) CreatePaymentMethodParams
		wantErr  bool
		validate func(t *testing.T, pm *PaymentMethod, params CreatePaymentMethodParams)
	}{
		{
			name: "create payment method successfully",
			setup: func(t *testing.T) CreatePaymentMethodParams {
				t.Helper()
				user, _ := createTestUser(t, testDB, "John", "Doe")
				return CreatePaymentMethodParams{
					UserID:       user.ID,
					StripeID:     "pm_test123",
					CardBrand:    "visa",
					CardLast4:    "4242",
					CardExpMonth: 12,
					CardExpYear:  2025,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, pm *PaymentMethod, params CreatePaymentMethodParams) {
				t.Helper()
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
				if pm.CardExpMonth != params.CardExpMonth {
					t.Errorf("CardExpMonth = %v, want %v", pm.CardExpMonth, params.CardExpMonth)
				}
				if pm.CardExpYear != params.CardExpYear {
					t.Errorf("CardExpYear = %v, want %v", pm.CardExpYear, params.CardExpYear)
				}
			},
		},
		{
			name: "create payment method with different card brand",
			setup: func(t *testing.T) CreatePaymentMethodParams {
				t.Helper()
				user, _ := createTestUser(t, testDB, "Jane", "Smith")
				return CreatePaymentMethodParams{
					UserID:       user.ID,
					StripeID:     "pm_test456",
					CardBrand:    "mastercard",
					CardLast4:    "5555",
					CardExpMonth: 6,
					CardExpYear:  2026,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, pm *PaymentMethod, params CreatePaymentMethodParams) {
				t.Helper()
				if pm.CardBrand != "mastercard" {
					t.Errorf("CardBrand = %v, want mastercard", pm.CardBrand)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			params := tt.setup(t)

			pm, err := testDB.Store.CreatePaymentMethod(ctx, params)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePaymentMethod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, pm, params)
			}
		})
	}
}

func TestStore_GetPaymentMethodByUserID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) uuid.UUID
		wantErr  bool
		validate func(t *testing.T, pm *PaymentMethod)
	}{
		{
			name: "get existing payment method",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "John", "Doe")
				testDB.Store.CreatePaymentMethod(ctx, CreatePaymentMethodParams{
					UserID:       user.ID,
					StripeID:     "pm_test123",
					CardBrand:    "visa",
					CardLast4:    "4242",
					CardExpMonth: 12,
					CardExpYear:  2025,
				})
				return user.ID
			},
			wantErr: false,
			validate: func(t *testing.T, pm *PaymentMethod) {
				t.Helper()
				if pm.StripeID != "pm_test123" {
					t.Errorf("StripeID = %v, want pm_test123", pm.StripeID)
				}
			},
		},
		{
			name: "payment method does not exist",
			setup: func(t *testing.T) uuid.UUID {
				return uuid.New()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			userID := tt.setup(t)

			pm, err := testDB.Store.GetPaymentMethodByUserID(ctx, userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPaymentMethodByUserID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, pm)
			}
		})
	}
}

func TestStore_UpdatePaymentMethodByUserID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name            string
		setup           func(t *testing.T) uuid.UUID
		newStripeID     string
		newCardBrand    string
		newCardLast4    string
		newCardExpMonth int64
		newCardExpYear  int64
		wantErr         bool
	}{
		{
			name: "update existing payment method",
			setup: func(t *testing.T) uuid.UUID {
				t.Helper()
				user, _ := createTestUser(t, testDB, "John", "Doe")
				testDB.Store.CreatePaymentMethod(ctx, CreatePaymentMethodParams{
					UserID:       user.ID,
					StripeID:     "pm_old123",
					CardBrand:    "visa",
					CardLast4:    "1111",
					CardExpMonth: 1,
					CardExpYear:  2024,
				})
				return user.ID
			},
			newStripeID:     "pm_new456",
			newCardBrand:    "mastercard",
			newCardLast4:    "2222",
			newCardExpMonth: 12,
			newCardExpYear:  2026,
			wantErr:         false,
		},
		{
			name: "update non-existent payment method",
			setup: func(t *testing.T) uuid.UUID {
				return uuid.New()
			},
			newStripeID:     "pm_test",
			newCardBrand:    "visa",
			newCardLast4:    "4242",
			newCardExpMonth: 12,
			newCardExpYear:  2025,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			userID := tt.setup(t)

			err := testDB.Store.UpdatePaymentMethodByUserID(ctx, userID, tt.newStripeID, tt.newCardBrand, tt.newCardLast4, tt.newCardExpMonth, tt.newCardExpYear)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdatePaymentMethodByUserID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the update
				pm, err := testDB.Store.GetPaymentMethodByUserID(ctx, userID)
				if err != nil {
					t.Errorf("failed to get updated payment method: %v", err)
					return
				}
				if pm.StripeID != tt.newStripeID {
					t.Errorf("StripeID = %v, want %v", pm.StripeID, tt.newStripeID)
				}
				if pm.CardBrand != tt.newCardBrand {
					t.Errorf("CardBrand = %v, want %v", pm.CardBrand, tt.newCardBrand)
				}
				if pm.CardLast4 != tt.newCardLast4 {
					t.Errorf("CardLast4 = %v, want %v", pm.CardLast4, tt.newCardLast4)
				}
			}
		})
	}
}
