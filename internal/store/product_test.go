package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestStore_CreateProduct(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("create product successfully", func(t *testing.T) {
		t.Parallel()
		stripeID := "prod_" + uuid.New().String()
		product, err := testDB.Store.CreateProduct(ctx, stripeID, "Premium Plan", "Premium subscription plan")
		if err != nil {
			t.Errorf("CreateProduct() error = %v", err)
			return
		}
		if product.StripeID != stripeID {
			t.Errorf("StripeID = %v, want %v", product.StripeID, stripeID)
		}
		if product.Name != "Premium Plan" {
			t.Errorf("Name = %v, want Premium Plan", product.Name)
		}
		if product.Description != "Premium subscription plan" {
			t.Errorf("Description = %v, want Premium subscription plan", product.Description)
		}
	})

	t.Run("create another product", func(t *testing.T) {
		t.Parallel()
		stripeID := "prod_" + uuid.New().String()
		product, err := testDB.Store.CreateProduct(ctx, stripeID, "Enterprise Plan", "Enterprise subscription plan")
		if err != nil {
			t.Errorf("CreateProduct() error = %v", err)
			return
		}
		if product.Name != "Enterprise Plan" {
			t.Errorf("Name = %v, want Enterprise Plan", product.Name)
		}
	})
}

func TestStore_GetProductByStripeID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("get existing product", func(t *testing.T) {
		t.Parallel()
		stripeID := "prod_" + uuid.New().String()
		_, err := testDB.Store.CreateProduct(ctx, stripeID, "Premium Plan", "Premium subscription")
		if err != nil {
			t.Fatalf("failed to create product: %v", err)
		}

		product, err := testDB.Store.GetProductByStripeID(ctx, stripeID)
		if err != nil {
			t.Errorf("GetProductByStripeID() error = %v", err)
			return
		}
		if product.Name != "Premium Plan" {
			t.Errorf("Name = %v, want Premium Plan", product.Name)
		}
	})

	t.Run("product does not exist", func(t *testing.T) {
		t.Parallel()
		_, err := testDB.Store.GetProductByStripeID(ctx, "prod_nonexistent_"+uuid.New().String())
		if err == nil {
			t.Error("GetProductByStripeID() expected error for non-existent product")
		}
	})
}

func TestStore_CreatePrice(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("create price successfully", func(t *testing.T) {
		t.Parallel()
		productStripeID := "prod_" + uuid.New().String()
		product, err := testDB.Store.CreateProduct(ctx, productStripeID, "Premium Plan", "Premium subscription")
		if err != nil {
			t.Fatalf("failed to create product: %v", err)
		}

		price := Price{
			ProductID:   product.ID,
			StripeID:    "price_" + uuid.New().String(),
			Description: "$29.99/month",
		}
		err = testDB.Store.CreatePrice(ctx, price)
		if err != nil {
			t.Errorf("CreatePrice() error = %v", err)
		}
	})

	t.Run("create another price", func(t *testing.T) {
		t.Parallel()
		productStripeID := "prod_" + uuid.New().String()
		product, err := testDB.Store.CreateProduct(ctx, productStripeID, "Enterprise Plan", "Enterprise subscription")
		if err != nil {
			t.Fatalf("failed to create product: %v", err)
		}

		price := Price{
			ProductID:   product.ID,
			StripeID:    "price_" + uuid.New().String(),
			Description: "$99.99/month",
		}
		err = testDB.Store.CreatePrice(ctx, price)
		if err != nil {
			t.Errorf("CreatePrice() error = %v", err)
		}
	})
}

func TestStore_GetPriceByStripeID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("get existing price", func(t *testing.T) {
		t.Parallel()
		productStripeID := "prod_" + uuid.New().String()
		product, err := testDB.Store.CreateProduct(ctx, productStripeID, "Premium Plan", "Premium subscription")
		if err != nil {
			t.Fatalf("failed to create product: %v", err)
		}

		priceStripeID := "price_" + uuid.New().String()
		price := Price{
			ProductID:   product.ID,
			StripeID:    priceStripeID,
			Description: "$29.99/month",
		}
		err = testDB.Store.CreatePrice(ctx, price)
		if err != nil {
			t.Fatalf("failed to create price: %v", err)
		}

		gotPrice, err := testDB.Store.GetPriceByStripeID(ctx, priceStripeID)
		if err != nil {
			t.Errorf("GetPriceByStripeID() error = %v", err)
			return
		}
		if gotPrice.Description != "$29.99/month" {
			t.Errorf("Description = %v, want $29.99/month", gotPrice.Description)
		}
	})

	t.Run("price does not exist", func(t *testing.T) {
		t.Parallel()
		_, err := testDB.Store.GetPriceByStripeID(ctx, "price_nonexistent_"+uuid.New().String())
		if err == nil {
			t.Error("GetPriceByStripeID() expected error for non-existent price")
		}
	})
}

func TestStore_UpdatePriceByStripeID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("update existing price", func(t *testing.T) {
		t.Parallel()
		productStripeID := "prod_" + uuid.New().String()
		product, err := testDB.Store.CreateProduct(ctx, productStripeID, "Premium Plan", "Premium subscription")
		if err != nil {
			t.Fatalf("failed to create product: %v", err)
		}

		priceStripeID := "price_" + uuid.New().String()
		price := Price{
			ProductID:   product.ID,
			StripeID:    priceStripeID,
			Description: "$29.99/month",
		}
		err = testDB.Store.CreatePrice(ctx, price)
		if err != nil {
			t.Fatalf("failed to create price: %v", err)
		}

		newDescription := "$39.99/month"
		err = testDB.Store.UpdatePriceByStripeID(ctx, product.ID, newDescription, priceStripeID)
		if err != nil {
			t.Errorf("UpdatePriceByStripeID() error = %v", err)
			return
		}

		// Verify the update
		updatedPrice, err := testDB.Store.GetPriceByStripeID(ctx, priceStripeID)
		if err != nil {
			t.Errorf("failed to get updated price: %v", err)
			return
		}
		if updatedPrice.Description != newDescription {
			t.Errorf("Description = %v, want %v", updatedPrice.Description, newDescription)
		}
	})

	t.Run("update non-existent price", func(t *testing.T) {
		t.Parallel()
		err := testDB.Store.UpdatePriceByStripeID(ctx, uuid.New(), "$49.99/month", "price_nonexistent_"+uuid.New().String())
		if err == nil {
			t.Error("UpdatePriceByStripeID() expected error for non-existent price")
		}
	})
}

func TestStore_DeletePriceByStripeID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("delete existing price", func(t *testing.T) {
		t.Parallel()
		productStripeID := "prod_" + uuid.New().String()
		product, err := testDB.Store.CreateProduct(ctx, productStripeID, "Premium Plan", "Premium subscription")
		if err != nil {
			t.Fatalf("failed to create product: %v", err)
		}

		priceStripeID := "price_" + uuid.New().String()
		price := Price{
			ProductID:   product.ID,
			StripeID:    priceStripeID,
			Description: "$29.99/month",
		}
		err = testDB.Store.CreatePrice(ctx, price)
		if err != nil {
			t.Fatalf("failed to create price: %v", err)
		}

		err = testDB.Store.DeletePriceByStripeID(ctx, priceStripeID)
		if err != nil {
			t.Errorf("DeletePriceByStripeID() error = %v", err)
			return
		}

		// Verify the deletion
		_, err = testDB.Store.GetPriceByStripeID(ctx, priceStripeID)
		if err == nil {
			t.Error("expected price to be deleted, but it still exists")
		}
	})
}

func TestStore_ListPrices(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	t.Run("list prices returns created prices", func(t *testing.T) {
		t.Parallel()
		// Create products and prices with unique IDs
		product1StripeID := "prod_" + uuid.New().String()
		product2StripeID := "prod_" + uuid.New().String()
		product1, err := testDB.Store.CreateProduct(ctx, product1StripeID, "Plan 1", "Description 1")
		if err != nil {
			t.Fatalf("failed to create product1: %v", err)
		}
		product2, err := testDB.Store.CreateProduct(ctx, product2StripeID, "Plan 2", "Description 2")
		if err != nil {
			t.Fatalf("failed to create product2: %v", err)
		}

		price1StripeID := "price_" + uuid.New().String()
		price2StripeID := "price_" + uuid.New().String()
		err = testDB.Store.CreatePrice(ctx, Price{
			ProductID:   product1.ID,
			StripeID:    price1StripeID,
			Description: "$29.99/month",
		})
		if err != nil {
			t.Fatalf("failed to create price1: %v", err)
		}
		err = testDB.Store.CreatePrice(ctx, Price{
			ProductID:   product2.ID,
			StripeID:    price2StripeID,
			Description: "$49.99/month",
		})
		if err != nil {
			t.Fatalf("failed to create price2: %v", err)
		}

		prices, err := testDB.Store.ListPrices(ctx)
		if err != nil {
			t.Errorf("ListPrices() error = %v", err)
			return
		}

		// Verify our created prices are in the list
		foundPrice1 := false
		foundPrice2 := false
		for _, p := range prices {
			if p.StripeID == price1StripeID {
				foundPrice1 = true
			}
			if p.StripeID == price2StripeID {
				foundPrice2 = true
			}
		}
		if !foundPrice1 || !foundPrice2 {
			t.Errorf("ListPrices() didn't return created prices, found price1: %v, found price2: %v", foundPrice1, foundPrice2)
		}
	})
}
