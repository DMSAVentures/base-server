//go:build integration
// +build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"base-server/internal/observability"
	"base-server/internal/store"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// TierConfig defines the configuration for a subscription tier
type TierConfig struct {
	Name            string
	PriceStripeID   string
	ProductStripeID string
	Features        map[string]bool // true = enabled, false = disabled
	Limits          map[string]*int // nil = unlimited
}

var (
	// Tier mutex and price IDs for thread-safe initialization
	tierMutex             sync.Mutex
	freeTierPriceID       uuid.UUID
	proTierPriceID        uuid.UUID
	proAnnualTierPriceID  uuid.UUID
	teamTierPriceID       uuid.UUID
	teamAnnualTierPriceID uuid.UUID
)

var (
	baseURL string
	logger  *observability.Logger
)

// Shared test store - initialized once and reused across all tests
var (
	sharedTestStore     store.Store
	sharedTestStoreOnce sync.Once
	sharedTestStoreErr  error
)

// getTierConfigs returns the configuration for all tiers
func getTierConfigs() map[string]TierConfig {
	// Free tier limits
	freeCampaigns := 1
	freeLeads := 200
	freeTeamMembers := 1

	// Pro tier limits
	proCampaigns := 5
	proLeads := 5000
	proTeamMembers := 1

	// Team tier limits
	teamLeads := 100000
	teamMembers := 5

	return map[string]TierConfig{
		"free": {
			Name:            "free",
			PriceStripeID:   "price_test_free",
			ProductStripeID: "prod_test_free",
			Features: map[string]bool{
				"email_verification":   false,
				"referral_system":      false,
				"visual_form_builder":  true,
				"visual_email_builder": false,
				"all_widget_types":     false,
				"remove_branding":      false,
				"anti_spam_protection": false,
				"enhanced_lead_data":   false,
				"tracking_pixels":      false,
				"webhooks_zapier":      false,
				"email_blasts":         false,
				"json_export":          false,
			},
			Limits: map[string]*int{
				"campaigns":    &freeCampaigns,
				"leads":        &freeLeads,
				"team_members": &freeTeamMembers,
			},
		},
		"pro": {
			Name:            "lc_pro_monthly",
			PriceStripeID:   "price_test_pro",
			ProductStripeID: "prod_test_pro",
			Features: map[string]bool{
				"email_verification":   true,
				"referral_system":      true,
				"visual_form_builder":  true,
				"visual_email_builder": true,
				"all_widget_types":     true,
				"remove_branding":      true,
				"anti_spam_protection": true,
				"enhanced_lead_data":   true,
				"tracking_pixels":      true,
				"webhooks_zapier":      false, // Not in Pro
				"email_blasts":         true,
				"json_export":          true,
			},
			Limits: map[string]*int{
				"campaigns":    &proCampaigns,
				"leads":        &proLeads,
				"team_members": &proTeamMembers,
			},
		},
		"pro_annual": {
			Name:            "lc_pro_annual",
			PriceStripeID:   "price_test_pro_annual",
			ProductStripeID: "prod_test_pro",
			Features: map[string]bool{
				"email_verification":   true,
				"referral_system":      true,
				"visual_form_builder":  true,
				"visual_email_builder": true,
				"all_widget_types":     true,
				"remove_branding":      true,
				"anti_spam_protection": true,
				"enhanced_lead_data":   true,
				"tracking_pixels":      true,
				"webhooks_zapier":      false, // Not in Pro
				"email_blasts":         true,
				"json_export":          true,
			},
			Limits: map[string]*int{
				"campaigns":    &proCampaigns,
				"leads":        &proLeads,
				"team_members": &proTeamMembers,
			},
		},
		"team": {
			Name:            "lc_team_monthly",
			PriceStripeID:   "price_test_team",
			ProductStripeID: "prod_test_team",
			Features: map[string]bool{
				"email_verification":   true,
				"referral_system":      true,
				"visual_form_builder":  true,
				"visual_email_builder": true,
				"all_widget_types":     true,
				"remove_branding":      true,
				"anti_spam_protection": true,
				"enhanced_lead_data":   true,
				"tracking_pixels":      true,
				"webhooks_zapier":      true, // Team tier has webhooks
				"email_blasts":         true,
				"json_export":          true,
			},
			Limits: map[string]*int{
				"campaigns":    nil, // Unlimited
				"leads":        &teamLeads,
				"team_members": &teamMembers,
			},
		},
		"team_annual": {
			Name:            "lc_team_annual",
			PriceStripeID:   "price_test_team_annual",
			ProductStripeID: "prod_test_team",
			Features: map[string]bool{
				"email_verification":   true,
				"referral_system":      true,
				"visual_form_builder":  true,
				"visual_email_builder": true,
				"all_widget_types":     true,
				"remove_branding":      true,
				"anti_spam_protection": true,
				"enhanced_lead_data":   true,
				"tracking_pixels":      true,
				"webhooks_zapier":      true, // Team tier has webhooks
				"email_blasts":         true,
				"json_export":          true,
			},
			Limits: map[string]*int{
				"campaigns":    nil, // Unlimited
				"leads":        &teamLeads,
				"team_members": &teamMembers,
			},
		},
	}
}

func init() {
	logger = observability.NewLogger()
	host := getEnv("TEST_API_HOST", "localhost")
	port := getEnv("TEST_API_PORT", "8080")
	baseURL = fmt.Sprintf("http://%s:%s", host, port)
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getSharedTestStore returns a shared database connection for all tests.
// This prevents "too many connections" errors by reusing a single connection pool.
func getSharedTestStore() (store.Store, error) {
	sharedTestStoreOnce.Do(func() {
		dbHost := getEnv("TEST_DB_HOST", "localhost")
		dbPort := getEnv("TEST_DB_PORT", "5432")
		dbUser := getEnv("TEST_DB_USER", "postgres")
		dbPass := getEnv("TEST_DB_PASS", "password123")
		dbName := getEnv("TEST_DB_NAME", "base_server_test")

		connectionString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			dbUser, dbPass, dbHost, dbPort, dbName)

		sharedTestStore, sharedTestStoreErr = store.New(connectionString, logger)
	})
	return sharedTestStore, sharedTestStoreErr
}

// setupTestStore returns the shared test database connection.
// Uses a single connection pool across all tests to prevent connection exhaustion.
func setupTestStore(t *testing.T) store.Store {
	testStore, err := getSharedTestStore()
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	return testStore
}

// makeRequest performs an HTTP request and returns the response and body
func makeRequest(t *testing.T, method, path string, body interface{}, headers map[string]string) (*http.Response, []byte) {
	client := &http.Client{Timeout: 10 * time.Second}

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, baseURL+path, reqBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")

	// Add custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	return resp, respBody
}

// makeAuthenticatedRequest performs an HTTP request with JWT token from cookie
func makeAuthenticatedRequest(t *testing.T, method, path string, body interface{}, token string) (*http.Response, []byte) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, baseURL+path, reqBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add JWT token as cookie
	if token != "" {
		req.AddCookie(&http.Cookie{
			Name:  "token",
			Value: token,
		})
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	return resp, respBody
}

// parseJSONResponse unmarshals JSON response into the provided interface
func parseJSONResponse(t *testing.T, body []byte, v interface{}) {
	err := json.Unmarshal(body, v)
	if err != nil {
		t.Fatalf("Failed to parse JSON response: %v\nBody: %s", err, string(body))
	}
}

// assertStatusCode checks if the response status code matches expected
func assertStatusCode(t *testing.T, resp *http.Response, expected int) {
	if resp.StatusCode != expected {
		t.Errorf("Expected status code %d, got %d", expected, resp.StatusCode)
	}
}

// assertResponseHeader checks if a response header has the expected value
func assertResponseHeader(t *testing.T, resp *http.Response, key, expected string) {
	actual := resp.Header.Get(key)
	if actual != expected {
		t.Errorf("Expected header %s to be %s, got %s", key, expected, actual)
	}
}

// createTestUserDirectly creates a test user by directly inserting into the database
// This bypasses the Stripe integration required by the signup endpoint
func createTestUserDirectly(t *testing.T, firstName, lastName, email, password string) (userID string, accountID string) {
	testStore := setupTestStore(t)
	ctx := context.Background()

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	// Start transaction
	tx, err := testStore.GetDB().BeginTxx(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Insert user
	var user store.User
	err = tx.GetContext(ctx, &user, `
		INSERT INTO users (first_name, last_name)
		VALUES ($1, $2)
		RETURNING id, first_name, last_name
	`, firstName, lastName)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Insert user_auth
	var userAuth store.UserAuth
	err = tx.GetContext(ctx, &userAuth, `
		INSERT INTO user_auth (user_id, auth_type)
		VALUES ($1, $2)
		RETURNING id, user_id, auth_type
	`, user.ID, "email")
	if err != nil {
		t.Fatalf("Failed to create user auth: %v", err)
	}

	// Insert email_auth
	_, err = tx.ExecContext(ctx, `
		INSERT INTO email_auth (auth_id, email, hashed_password)
		VALUES ($1, $2, $3)
	`, userAuth.ID, email, string(hashedPassword))
	if err != nil {
		t.Fatalf("Failed to create email auth: %v", err)
	}

	// Create account for user
	accountName := fmt.Sprintf("%s %s's Account", firstName, lastName)
	accountSlug := fmt.Sprintf("user-%s", user.ID.String()[:8])
	var accID uuid.UUID
	err = tx.GetContext(ctx, &accID, `
		INSERT INTO accounts (name, slug, owner_user_id, plan, settings)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, accountName, accountSlug, user.ID, "free", "{}")
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	return user.ID.String(), accID.String()
}

// createAuthenticatedTestUser creates a test user with TeamTier subscription (default).
// TeamTier is used by default because it has all features enabled, which is appropriate
// for most tests. Use createAuthenticatedTestUserWithFreeTier or createAuthenticatedTestUserWithProTier
// when testing tier-specific feature restrictions.
func createAuthenticatedTestUser(t *testing.T) string {
	return createAuthenticatedTestUserWithTier(t, "team")
}

// createAuthenticatedTestUserWithTier creates a test user with a specific tier subscription
// tierName can be "free", "pro", or "team"
func createAuthenticatedTestUserWithTier(t *testing.T, tierName string) string {
	email := generateTestEmail()
	password := "testpassword123"

	// Create user directly in database and get IDs
	userID, _ := createTestUserDirectly(t, "Test", "User", email, password)

	// Create subscription for the specified tier
	createTierSubscription(t, userID, tierName)

	// Login via API to get token
	loginReq := map[string]interface{}{
		"email":    email,
		"password": password,
	}
	loginResp, loginBody := makeRequest(t, http.MethodPost, "/api/auth/login/email", loginReq, nil)
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to login test user: %s", string(loginBody))
	}

	var loginRespData map[string]interface{}
	parseJSONResponse(t, loginBody, &loginRespData)
	return loginRespData["token"].(string)
}

// createAuthenticatedTestUserWithFreeTier creates a test user with a free tier subscription
func createAuthenticatedTestUserWithFreeTier(t *testing.T) string {
	return createAuthenticatedTestUserWithTier(t, "free")
}

// createAuthenticatedTestUserWithProTier creates a test user with a pro tier subscription
func createAuthenticatedTestUserWithProTier(t *testing.T) string {
	return createAuthenticatedTestUserWithTier(t, "pro")
}

// createAuthenticatedTestUserWithTeamTier creates a test user with a team tier subscription
// that has access to features like webhooks, email blasts, and JSON export
func createAuthenticatedTestUserWithTeamTier(t *testing.T) string {
	return createAuthenticatedTestUserWithTier(t, "team")
}

// ensureTierExists finds or creates a tier's price in the database (thread-safe)
// Creates the product, price, and plan_feature_limits if they don't exist
func ensureTierExists(t *testing.T, tierName string) uuid.UUID {
	configs := getTierConfigs()
	config, ok := configs[tierName]
	if !ok {
		t.Fatalf("Unknown tier: %s", tierName)
	}

	tierMutex.Lock()
	defer tierMutex.Unlock()

	var priceIDPtr *uuid.UUID
	switch tierName {
	case "free":
		priceIDPtr = &freeTierPriceID
	case "pro":
		priceIDPtr = &proTierPriceID
	case "pro_annual":
		priceIDPtr = &proAnnualTierPriceID
	case "team":
		priceIDPtr = &teamTierPriceID
	case "team_annual":
		priceIDPtr = &teamAnnualTierPriceID
	default:
		t.Fatalf("Unknown tier: %s", tierName)
	}

	// If already looked up, return the cached price ID
	if *priceIDPtr != uuid.Nil {
		return *priceIDPtr
	}

	testStore := setupTestStore(t)
	ctx := context.Background()
	db := testStore.GetDB()

	// Try to find existing price
	var priceID uuid.UUID
	err := db.GetContext(ctx, &priceID, `
		SELECT id FROM prices WHERE description = $1 AND deleted_at IS NULL LIMIT 1
	`, config.Name)

	if err == nil {
		// Price exists, cache and return
		*priceIDPtr = priceID
		return priceID
	}

	// Price doesn't exist, need to create it
	// First, ensure all tiers are created to avoid partial state
	ensureAllTiersCreated(t, db, ctx)

	// Now look up the price again
	err = db.GetContext(ctx, &priceID, `
		SELECT id FROM prices WHERE description = $1 AND deleted_at IS NULL LIMIT 1
	`, config.Name)
	if err != nil {
		t.Fatalf("Failed to find price for tier %s (description=%s) after creation: %v", tierName, config.Name, err)
	}

	// Cache and return the price ID
	*priceIDPtr = priceID
	return priceID
}

// ensureAllTiersCreated creates all products, prices, features, limits, and plan_feature_limits
// This is called once when any tier is first needed
func ensureAllTiersCreated(t *testing.T, db *sqlx.DB, ctx context.Context) {
	// Check if we've already created the tiers by looking for the free price
	var count int
	err := db.GetContext(ctx, &count, `SELECT COUNT(*) FROM prices WHERE description = 'free' AND deleted_at IS NULL`)
	if err == nil && count > 0 {
		return // Already created
	}

	// Create products
	createTestProducts(t, db, ctx)

	// Create prices
	createTestPrices(t, db, ctx)

	// Ensure features exist
	ensureFeaturesExist(t, db, ctx)

	// Ensure limits exist
	ensureLimitsExist(t, db, ctx)

	// Create plan_feature_limits
	createPlanFeatureLimits(t, db, ctx)
}

// createTestProducts creates the test products if they don't exist
func createTestProducts(t *testing.T, db *sqlx.DB, ctx context.Context) {
	products := []struct {
		name        string
		description string
		stripeID    string
	}{
		{"Free Plan", "Free tier product", "prod_test_free"},
		{"Pro Plan", "Pro tier product", "prod_test_pro"},
		{"Team Plan", "Team tier product", "prod_test_team"},
	}

	for _, p := range products {
		_, err := db.ExecContext(ctx, `
			INSERT INTO products (name, description, stripe_id)
			VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING
		`, p.name, p.description, p.stripeID)
		if err != nil {
			t.Fatalf("Failed to create product %s: %v", p.name, err)
		}
	}
}

// createTestPrices creates the test prices if they don't exist
func createTestPrices(t *testing.T, db *sqlx.DB, ctx context.Context) {
	prices := []struct {
		productStripeID string
		description     string
		stripeID        string
	}{
		{"prod_test_free", "free", "price_test_free"},
		{"prod_test_pro", "lc_pro_monthly", "price_test_pro"},
		{"prod_test_pro", "lc_pro_annual", "price_test_pro_annual"},
		{"prod_test_team", "lc_team_monthly", "price_test_team"},
		{"prod_test_team", "lc_team_annual", "price_test_team_annual"},
	}

	for _, p := range prices {
		_, err := db.ExecContext(ctx, `
			INSERT INTO prices (product_id, description, stripe_id)
			SELECT id, $2, $3 FROM products WHERE stripe_id = $1 AND deleted_at IS NULL
			ON CONFLICT DO NOTHING
		`, p.productStripeID, p.description, p.stripeID)
		if err != nil {
			t.Fatalf("Failed to create price %s: %v", p.description, err)
		}
	}
}

// ensureFeaturesExist ensures all features exist in the database
func ensureFeaturesExist(t *testing.T, db *sqlx.DB, ctx context.Context) {
	features := []struct {
		name        string
		description string
	}{
		{"email_verification", "Verify user emails before adding to waitlist"},
		{"referral_system", "Enable referral tracking and rewards"},
		{"visual_form_builder", "Drag-and-drop form customization"},
		{"visual_email_builder", "Design emails with visual editor"},
		{"all_widget_types", "Access to all widget types"},
		{"remove_branding", "Remove platform branding from forms"},
		{"anti_spam_protection", "Advanced spam and bot protection"},
		{"enhanced_lead_data", "Collect additional lead information"},
		{"tracking_pixels", "Add Facebook, Google, and other tracking pixels"},
		{"webhooks_zapier", "Integrate with external services"},
		{"email_blasts", "Send bulk emails to waitlist"},
		{"json_export", "Export data in JSON format"},
		{"campaigns", "Number of waitlist campaigns"},
		{"leads", "Maximum leads per account"},
		{"team_members", "Number of team members"},
	}

	for _, f := range features {
		_, err := db.ExecContext(ctx, `
			INSERT INTO features (name, description)
			VALUES ($1, $2)
			ON CONFLICT (name) DO NOTHING
		`, f.name, f.description)
		if err != nil {
			t.Fatalf("Failed to create feature %s: %v", f.name, err)
		}
	}
}

// ensureLimitsExist ensures all limits exist in the database
func ensureLimitsExist(t *testing.T, db *sqlx.DB, ctx context.Context) {
	limits := []struct {
		featureName string
		limitName   string
		limitValue  int
	}{
		{"campaigns", "campaigns_free", 1},
		{"leads", "leads_free", 200},
		{"leads", "leads_pro", 5000},
		{"leads", "leads_team", 100000},
		{"team_members", "team_members_free", 1},
		{"team_members", "team_members_pro", 1},
		{"team_members", "team_members_team", 5},
	}

	for _, l := range limits {
		_, err := db.ExecContext(ctx, `
			INSERT INTO limits (feature_id, limit_name, limit_value)
			SELECT id, $2, $3 FROM features WHERE name = $1
			ON CONFLICT (feature_id, limit_name) DO NOTHING
		`, l.featureName, l.limitName, l.limitValue)
		if err != nil {
			t.Fatalf("Failed to create limit %s: %v", l.limitName, err)
		}
	}
}

// createPlanFeatureLimits creates the plan_feature_limits mappings
func createPlanFeatureLimits(t *testing.T, db *sqlx.DB, ctx context.Context) {
	configs := getTierConfigs()

	// Boolean features (not resource limits)
	boolFeatures := []string{
		"email_verification", "referral_system", "visual_form_builder", "visual_email_builder",
		"all_widget_types", "remove_branding", "anti_spam_protection", "enhanced_lead_data",
		"tracking_pixels", "webhooks_zapier", "email_blasts", "json_export",
	}

	for tierName, config := range configs {
		// Get price ID
		var priceID uuid.UUID
		err := db.GetContext(ctx, &priceID, `
			SELECT id FROM prices WHERE description = $1 AND deleted_at IS NULL LIMIT 1
		`, config.Name)
		if err != nil {
			t.Fatalf("Failed to get price ID for tier %s: %v", tierName, err)
		}

		// Insert boolean features
		for _, featureName := range boolFeatures {
			enabled := config.Features[featureName]
			_, err := db.ExecContext(ctx, `
				INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
				SELECT $1, id, NULL, $3 FROM features WHERE name = $2
				ON CONFLICT (plan_id, feature_id) DO NOTHING
			`, priceID, featureName, enabled)
			if err != nil {
				t.Fatalf("Failed to create plan_feature_limit for tier %s, feature %s: %v", tierName, featureName, err)
			}
		}

		// Insert resource limits (campaigns, leads, team_members)
		insertResourceLimit(t, db, ctx, priceID, "campaigns", config.Limits["campaigns"], tierName)
		insertResourceLimit(t, db, ctx, priceID, "leads", config.Limits["leads"], tierName)
		insertResourceLimit(t, db, ctx, priceID, "team_members", config.Limits["team_members"], tierName)
	}
}

// insertResourceLimit inserts a resource limit for a tier
func insertResourceLimit(t *testing.T, db *sqlx.DB, ctx context.Context, priceID uuid.UUID, featureName string, limitValue *int, tierName string) {
	if limitValue == nil {
		// Unlimited - insert with NULL limit_id
		_, err := db.ExecContext(ctx, `
			INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
			SELECT $1, id, NULL, true FROM features WHERE name = $2
			ON CONFLICT (plan_id, feature_id) DO NOTHING
		`, priceID, featureName)
		if err != nil {
			t.Fatalf("Failed to create unlimited plan_feature_limit for tier %s, feature %s: %v", tierName, featureName, err)
		}
	} else {
		// Limited - find the appropriate limit and link it
		limitName := getLimitName(featureName, tierName)
		_, err := db.ExecContext(ctx, `
			INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
			SELECT $1, f.id, l.id, true
			FROM features f
			JOIN limits l ON l.feature_id = f.id
			WHERE f.name = $2 AND l.limit_name = $3
			ON CONFLICT (plan_id, feature_id) DO NOTHING
		`, priceID, featureName, limitName)
		if err != nil {
			t.Fatalf("Failed to create plan_feature_limit for tier %s, feature %s, limit %s: %v", tierName, featureName, limitName, err)
		}
	}
}

// getLimitName returns the limit name for a feature and tier
func getLimitName(featureName, tierName string) string {
	tierBase := tierName
	// Map annual tiers to their base tier for limit names
	switch tierName {
	case "pro_annual":
		tierBase = "pro"
	case "team_annual":
		tierBase = "team"
	}
	return featureName + "_" + tierBase
}

// createTierSubscription creates a subscription for a user with the specified tier
func createTierSubscription(t *testing.T, userID string, tierName string) {
	priceID := ensureTierExists(t, tierName)

	testStore := setupTestStore(t)
	ctx := context.Background()
	db := testStore.GetDB()

	parsedUserID, _ := uuid.Parse(userID)
	// Set end_date and next_billing_date to 30 days from now (required by Subscription struct)
	_, err := db.ExecContext(ctx, `
		INSERT INTO subscriptions (user_id, price_id, stripe_id, status, start_date, end_date, next_billing_date)
		VALUES ($1, $2, $3, 'active', NOW(), NOW() + INTERVAL '30 days', NOW() + INTERVAL '30 days')
	`, parsedUserID, priceID, "sub_test_"+uuid.New().String()[:8])
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}
}

// Legacy aliases for backward compatibility
func ensureTeamTierExists(t *testing.T) uuid.UUID {
	return ensureTierExists(t, "team")
}

func createTeamTierSubscription(t *testing.T, userID string) {
	createTierSubscription(t, userID, "team")
}

// extractCookie extracts a cookie value from the response by name
func extractCookie(resp *http.Response, name string) string {
	for _, cookie := range resp.Cookies() {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

// generateTestEmail generates a unique test email address
func generateTestEmail() string {
	return fmt.Sprintf("test-%s@example.com", uuid.New().String()[:8])
}

// generateTestCampaignSlug generates a unique campaign slug
func generateTestCampaignSlug() string {
	return fmt.Sprintf("test-campaign-%s", uuid.New().String()[:8])
}

// waitForCondition polls a condition function until it returns true or timeout
func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, errorMsg string) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("Timeout waiting for condition: %s", errorMsg)
}

// getKeys returns the keys from a map for debugging
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// --- Testify-based Assertion Helpers ---

// APIResponse wraps an HTTP response for fluent assertions.
type APIResponse struct {
	t          *testing.T
	Response   *http.Response
	Body       []byte
	parsedJSON map[string]interface{}
}

// NewAPIResponse creates a new APIResponse wrapper.
func NewAPIResponse(t *testing.T, resp *http.Response, body []byte) *APIResponse {
	t.Helper()
	return &APIResponse{t: t, Response: resp, Body: body}
}

// RequireStatus asserts the response has the expected status code (fails test immediately if not).
func (r *APIResponse) RequireStatus(expected int) *APIResponse {
	r.t.Helper()
	require.Equal(r.t, expected, r.Response.StatusCode,
		"unexpected status code, body: %s", string(r.Body))
	return r
}

// AssertStatus asserts the response has the expected status code.
func (r *APIResponse) AssertStatus(expected int) *APIResponse {
	r.t.Helper()
	assert.Equal(r.t, expected, r.Response.StatusCode,
		"unexpected status code, body: %s", string(r.Body))
	return r
}

// JSON parses the response body as JSON and returns the parsed map.
func (r *APIResponse) JSON() map[string]interface{} {
	r.t.Helper()
	if r.parsedJSON == nil {
		r.parsedJSON = make(map[string]interface{})
		require.NoError(r.t, json.Unmarshal(r.Body, &r.parsedJSON),
			"failed to parse JSON response: %s", string(r.Body))
	}
	return r.parsedJSON
}

// AssertJSONField asserts a field exists and has the expected value.
func (r *APIResponse) AssertJSONField(field string, expected interface{}) *APIResponse {
	r.t.Helper()
	data := r.JSON()
	assert.Contains(r.t, data, field, "field %s not found in response", field)
	if expected != nil {
		assert.Equal(r.t, expected, data[field], "field %s has unexpected value", field)
	}
	return r
}

// AssertJSONFieldExists asserts a field exists (value can be anything).
func (r *APIResponse) AssertJSONFieldExists(field string) *APIResponse {
	r.t.Helper()
	data := r.JSON()
	assert.Contains(r.t, data, field, "field %s not found in response", field)
	return r
}

// AssertJSONFieldNotNil asserts a field exists and is not nil.
func (r *APIResponse) AssertJSONFieldNotNil(field string) *APIResponse {
	r.t.Helper()
	data := r.JSON()
	assert.Contains(r.t, data, field, "field %s not found in response", field)
	assert.NotNil(r.t, data[field], "field %s is nil", field)
	return r
}

// AssertError asserts the response contains an error field.
func (r *APIResponse) AssertError() *APIResponse {
	r.t.Helper()
	data := r.JSON()
	assert.Contains(r.t, data, "error", "expected error field in response")
	return r
}

// --- Request Builder ---

// APIRequest helps build and execute API requests.
type APIRequest struct {
	t       *testing.T
	method  string
	path    string
	body    interface{}
	token   string
	headers map[string]string
}

// NewRequest creates a new API request builder.
func NewRequest(t *testing.T, method, path string) *APIRequest {
	t.Helper()
	return &APIRequest{
		t:       t,
		method:  method,
		path:    path,
		headers: make(map[string]string),
	}
}

// WithBody sets the request body.
func (r *APIRequest) WithBody(body interface{}) *APIRequest {
	r.body = body
	return r
}

// WithToken sets the authentication token.
func (r *APIRequest) WithToken(token string) *APIRequest {
	r.token = token
	return r
}

// WithHeader adds a header to the request.
func (r *APIRequest) WithHeader(key, value string) *APIRequest {
	r.headers[key] = value
	return r
}

// Do executes the request and returns an APIResponse.
func (r *APIRequest) Do() *APIResponse {
	r.t.Helper()
	var resp *http.Response
	var body []byte

	if r.token != "" {
		resp, body = makeAuthenticatedRequest(r.t, r.method, r.path, r.body, r.token)
	} else {
		resp, body = makeRequest(r.t, r.method, r.path, r.body, r.headers)
	}

	return NewAPIResponse(r.t, resp, body)
}

// --- Convenience Functions ---

// GET creates a GET request.
func GET(t *testing.T, path string) *APIRequest {
	return NewRequest(t, http.MethodGet, path)
}

// POST creates a POST request.
func POST(t *testing.T, path string) *APIRequest {
	return NewRequest(t, http.MethodPost, path)
}

// PUT creates a PUT request.
func PUT(t *testing.T, path string) *APIRequest {
	return NewRequest(t, http.MethodPut, path)
}

// DELETE creates a DELETE request.
func DELETE(t *testing.T, path string) *APIRequest {
	return NewRequest(t, http.MethodDelete, path)
}

// PATCH creates a PATCH request.
func PATCH(t *testing.T, path string) *APIRequest {
	return NewRequest(t, http.MethodPatch, path)
}
