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
	tierMutex       sync.Mutex
	freeTierPriceID uuid.UUID
	proTierPriceID  uuid.UUID
	teamTierPriceID uuid.UUID
)

var (
	baseURL string
	logger  *observability.Logger
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
				"leads":        nil, // Unlimited
				"team_members": nil, // Unlimited
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

// setupTestStore creates a connection to the test database
func setupTestStore(t *testing.T) store.Store {
	dbHost := getEnv("TEST_DB_HOST", "localhost")
	dbPort := getEnv("TEST_DB_PORT", "5432")
	dbUser := getEnv("TEST_DB_USER", "postgres")
	dbPass := getEnv("TEST_DB_PASS", "password123")
	dbName := getEnv("TEST_DB_NAME", "base_server_test")

	connectionString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, dbHost, dbPort, dbName)

	testStore, err := store.New(connectionString, logger)
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

// createAuthenticatedTestUser creates a test user in the database and returns a valid JWT token
func createAuthenticatedTestUser(t *testing.T) string {
	email := generateTestEmail()
	password := "testpassword123"

	// Create user directly in database
	createTestUserDirectly(t, "Test", "User", email, password)

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

// ensureTierExists finds an existing tier's price from the database (thread-safe)
// The tiers are seeded via migrations, so we just need to look them up
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
	case "team":
		priceIDPtr = &teamTierPriceID
	default:
		t.Fatalf("Unknown tier: %s", tierName)
	}

	// If already looked up, return the cached price ID
	if *priceIDPtr != uuid.Nil {
		return *priceIDPtr
	}

	// Look up the existing price from the database (seeded via migrations)
	testStore := setupTestStore(t)
	ctx := context.Background()
	db := testStore.GetDB()

	var priceID uuid.UUID
	err := db.GetContext(ctx, &priceID, `
		SELECT id FROM prices WHERE description = $1 AND deleted_at IS NULL LIMIT 1
	`, config.Name)
	if err != nil {
		t.Fatalf("Failed to find price for tier %s (description=%s): %v. Make sure migrations have been run.", tierName, config.Name, err)
	}

	// Cache and return the price ID
	*priceIDPtr = priceID
	return priceID
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
