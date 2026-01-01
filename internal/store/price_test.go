package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_GetFreePriceStripeID(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)
	ctx := context.Background()

	t.Run("returns stripe_id when free price exists", func(t *testing.T) {
		t.Parallel()
		// Ensure at least one free price exists
		ensureFreePriceExists(t, testDB)

		stripeID, err := testDB.Store.GetFreePriceStripeID(ctx)

		require.NoError(t, err)
		assert.NotEmpty(t, stripeID, "stripe_id should not be empty")
	})
}

// Helper functions for creating test data

func createTestProduct(t *testing.T, testDB *TestDB, name string) Product {
	t.Helper()
	stripeID := "prod_" + uuid.New().String()[:8]
	description := "Test product description"

	var product Product
	query := `INSERT INTO products (stripe_id, name, description)
	          VALUES ($1, $2, $3)
	          RETURNING id, stripe_id, name, description`
	err := testDB.GetDB().Get(&product, query, stripeID, name, description)
	require.NoError(t, err, "failed to create test product")
	return product
}

func createTestPrice(t *testing.T, testDB *TestDB, productID uuid.UUID, description string) uuid.UUID {
	t.Helper()
	stripeID := "price_" + uuid.New().String()[:8]

	var priceID uuid.UUID
	query := `INSERT INTO prices (product_id, stripe_id, description)
	          VALUES ($1, $2, $3)
	          RETURNING id`
	err := testDB.GetDB().Get(&priceID, query, productID, stripeID, description)
	require.NoError(t, err, "failed to create test price")
	return priceID
}

func ensureFreePriceExists(t *testing.T, testDB *TestDB) {
	t.Helper()
	// Check if free price already exists
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM prices WHERE description = 'free' AND deleted_at IS NULL)`
	err := testDB.GetDB().Get(&exists, query)
	require.NoError(t, err, "failed to check if free price exists")

	if !exists {
		// Create a product for the free price
		product := createTestProduct(t, testDB, "Free Product")
		createTestPrice(t, testDB, product.ID, "free")
	}
}
