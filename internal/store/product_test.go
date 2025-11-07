package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestStore_CreateProduct(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name        string
		stripeID    string
		productName string
		description string
		wantErr     bool
		validate    func(t *testing.T, product Product)
	}{
		{
			name:        "create product successfully",
			stripeID:    "prod_test123",
			productName: "Premium Plan",
			description: "Premium subscription plan",
			wantErr:     false,
			validate: func(t *testing.T, product Product) {
				t.Helper()
				if product.StripeID != "prod_test123" {
					t.Errorf("StripeID = %v, want prod_test123", product.StripeID)
				}
				if product.Name != "Premium Plan" {
					t.Errorf("Name = %v, want Premium Plan", product.Name)
				}
				if product.Description != "Premium subscription plan" {
					t.Errorf("Description = %v, want Premium subscription plan", product.Description)
				}
			},
		},
		{
			name:        "create another product",
			stripeID:    "prod_test456",
			productName: "Enterprise Plan",
			description: "Enterprise subscription plan",
			wantErr:     false,
			validate: func(t *testing.T, product Product) {
				t.Helper()
				if product.Name != "Enterprise Plan" {
					t.Errorf("Name = %v, want Enterprise Plan", product.Name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)

			product, err := testDB.Store.CreateProduct(ctx, tt.stripeID, tt.productName, tt.description)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateProduct() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, product)
			}
		})
	}
}

func TestStore_GetProductByStripeID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) string
		wantErr  bool
		validate func(t *testing.T, product Product)
	}{
		{
			name: "get existing product",
			setup: func(t *testing.T) string {
				t.Helper()
				product, _ := testDB.Store.CreateProduct(ctx, "prod_test123", "Premium Plan", "Premium subscription")
				return product.StripeID
			},
			wantErr: false,
			validate: func(t *testing.T, product Product) {
				t.Helper()
				if product.Name != "Premium Plan" {
					t.Errorf("Name = %v, want Premium Plan", product.Name)
				}
			},
		},
		{
			name: "product does not exist",
			setup: func(t *testing.T) string {
				return "prod_nonexistent"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			stripeID := tt.setup(t)

			product, err := testDB.Store.GetProductByStripeID(ctx, stripeID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProductByStripeID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, product)
			}
		})
	}
}

func TestStore_CreatePrice(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func(t *testing.T) Price
		wantErr bool
	}{
		{
			name: "create price successfully",
			setup: func(t *testing.T) Price {
				t.Helper()
				product, _ := testDB.Store.CreateProduct(ctx, "prod_test123", "Premium Plan", "Premium subscription")
				return Price{
					ProductID:   product.ID,
					StripeID:    "price_test123",
					Description: "$29.99/month",
				}
			},
			wantErr: false,
		},
		{
			name: "create another price",
			setup: func(t *testing.T) Price {
				t.Helper()
				product, _ := testDB.Store.CreateProduct(ctx, "prod_test456", "Enterprise Plan", "Enterprise subscription")
				return Price{
					ProductID:   product.ID,
					StripeID:    "price_test456",
					Description: "$99.99/month",
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			price := tt.setup(t)

			err := testDB.Store.CreatePrice(ctx, price)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePrice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStore_GetPriceByStripeID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(t *testing.T) string
		wantErr  bool
		validate func(t *testing.T, price Price)
	}{
		{
			name: "get existing price",
			setup: func(t *testing.T) string {
				t.Helper()
				product, _ := testDB.Store.CreateProduct(ctx, "prod_test123", "Premium Plan", "Premium subscription")
				price := Price{
					ProductID:   product.ID,
					StripeID:    "price_test123",
					Description: "$29.99/month",
				}
				testDB.Store.CreatePrice(ctx, price)
				return price.StripeID
			},
			wantErr: false,
			validate: func(t *testing.T, price Price) {
				t.Helper()
				if price.Description != "$29.99/month" {
					t.Errorf("Description = %v, want $29.99/month", price.Description)
				}
			},
		},
		{
			name: "price does not exist",
			setup: func(t *testing.T) string {
				return "price_nonexistent"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			stripeID := tt.setup(t)

			price, err := testDB.Store.GetPriceByStripeID(ctx, stripeID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPriceByStripeID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, price)
			}
		})
	}
}

func TestStore_UpdatePriceByStripeID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name           string
		setup          func(t *testing.T) (string, uuid.UUID)
		newDescription string
		wantErr        bool
	}{
		{
			name: "update existing price",
			setup: func(t *testing.T) (string, uuid.UUID) {
				t.Helper()
				product, _ := testDB.Store.CreateProduct(ctx, "prod_test123", "Premium Plan", "Premium subscription")
				price := Price{
					ProductID:   product.ID,
					StripeID:    "price_test123",
					Description: "$29.99/month",
				}
				testDB.Store.CreatePrice(ctx, price)
				return price.StripeID, product.ID
			},
			newDescription: "$39.99/month",
			wantErr:        false,
		},
		{
			name: "update non-existent price",
			setup: func(t *testing.T) (string, uuid.UUID) {
				return "price_nonexistent", uuid.New()
			},
			newDescription: "$49.99/month",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			stripeID, productID := tt.setup(t)

			err := testDB.Store.UpdatePriceByStripeID(ctx, productID, tt.newDescription, stripeID)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdatePriceByStripeID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the update
				price, err := testDB.Store.GetPriceByStripeID(ctx, stripeID)
				if err != nil {
					t.Errorf("failed to get updated price: %v", err)
					return
				}
				if price.Description != tt.newDescription {
					t.Errorf("Description = %v, want %v", price.Description, tt.newDescription)
				}
			}
		})
	}
}

func TestStore_DeletePriceByStripeID(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr bool
	}{
		{
			name: "delete existing price",
			setup: func(t *testing.T) string {
				t.Helper()
				product, _ := testDB.Store.CreateProduct(ctx, "prod_test123", "Premium Plan", "Premium subscription")
				price := Price{
					ProductID:   product.ID,
					StripeID:    "price_test123",
					Description: "$29.99/month",
				}
				testDB.Store.CreatePrice(ctx, price)
				return price.StripeID
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			stripeID := tt.setup(t)

			err := testDB.Store.DeletePriceByStripeID(ctx, stripeID)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeletePriceByStripeID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the deletion
				_, err := testDB.Store.GetPriceByStripeID(ctx, stripeID)
				if err == nil {
					t.Error("expected price to be deleted, but it still exists")
				}
			}
		})
	}
}

func TestStore_ListPrices(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()

	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(t *testing.T)
		wantCount int
		wantErr   bool
	}{
		{
			name: "list multiple prices",
			setup: func(t *testing.T) {
				t.Helper()
				product1, _ := testDB.Store.CreateProduct(ctx, "prod_test1", "Plan 1", "Description 1")
				product2, _ := testDB.Store.CreateProduct(ctx, "prod_test2", "Plan 2", "Description 2")

				testDB.Store.CreatePrice(ctx, Price{
					ProductID:   product1.ID,
					StripeID:    "price_test1",
					Description: "$29.99/month",
				})
				testDB.Store.CreatePrice(ctx, Price{
					ProductID:   product2.ID,
					StripeID:    "price_test2",
					Description: "$49.99/month",
				})
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "list empty prices",
			setup: func(t *testing.T) {
				// No prices created
			},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB.Truncate(t)
			tt.setup(t)

			prices, err := testDB.Store.ListPrices(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListPrices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(prices) != tt.wantCount {
				t.Errorf("ListPrices() count = %v, want %v", len(prices), tt.wantCount)
			}
		})
	}
}
