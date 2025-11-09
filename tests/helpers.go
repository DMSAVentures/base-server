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
	"testing"
	"time"

	"base-server/internal/observability"
	"base-server/internal/store"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	baseURL string
	logger  *observability.Logger
)

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
