//go:build integration
// +build integration

package tests

import (
	"net/http"
	"testing"
)

func TestAPI_Auth_EmailSignup(t *testing.T) {
	t.Skip("Skipping email signup test - requires Stripe API key for customer creation")
	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte, resp *http.Response)
	}{
		{
			name: "successful signup with valid data",
			request: map[string]interface{}{
				"first_name": "John",
				"last_name":  "Doe",
				"email":      generateTestEmail(),
				"password":   "password123",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte, resp *http.Response) {
				var user map[string]interface{}
				parseJSONResponse(t, body, &user)

				// Note: SignedUpUser struct has first_name, last_name, email (no id)
				if user["email"] == nil {
					t.Error("Expected email in response")
				}
				if user["first_name"] != "John" {
					t.Errorf("Expected first_name to be 'John', got %v", user["first_name"])
				}
				if user["last_name"] != "Doe" {
					t.Errorf("Expected last_name to be 'Doe', got %v", user["last_name"])
				}
			},
		},
		{
			name: "signup fails with missing first name",
			request: map[string]interface{}{
				"last_name": "Doe",
				"email":     generateTestEmail(),
				"password":  "password123",
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, body []byte, resp *http.Response) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)

				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
		{
			name: "signup fails with missing last name",
			request: map[string]interface{}{
				"first_name": "John",
				"email":      generateTestEmail(),
				"password":   "password123",
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, body []byte, resp *http.Response) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)

				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
		{
			name: "signup fails with invalid email format",
			request: map[string]interface{}{
				"first_name": "John",
				"last_name":  "Doe",
				"email":      "invalid-email",
				"password":   "password123",
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, body []byte, resp *http.Response) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)

				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
		{
			name: "signup fails with short password",
			request: map[string]interface{}{
				"first_name": "John",
				"last_name":  "Doe",
				"email":      generateTestEmail(),
				"password":   "short",
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, body []byte, resp *http.Response) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)

				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
		{
			name: "signup fails with missing email",
			request: map[string]interface{}{
				"first_name": "John",
				"last_name":  "Doe",
				"password":   "password123",
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, body []byte, resp *http.Response) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)

				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
		{
			name: "signup fails with missing password",
			request: map[string]interface{}{
				"first_name": "John",
				"last_name":  "Doe",
				"email":      generateTestEmail(),
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, body []byte, resp *http.Response) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)

				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(t, http.MethodPost, "/api/auth/signup/email", tt.request, nil)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body, resp)
			}
		})
	}
}

func TestAPI_Auth_EmailLogin(t *testing.T) {
	// First, create a test user for login tests (directly in database to bypass Stripe)
	testEmail := generateTestEmail()
	testPassword := "testpassword123"
	createTestUserDirectly(t, "TestLogin", "User", testEmail, testPassword)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte, resp *http.Response)
	}{
		{
			name: "successful login with valid credentials",
			request: map[string]interface{}{
				"email":    testEmail,
				"password": testPassword,
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte, resp *http.Response) {
				var loginResp map[string]interface{}
				parseJSONResponse(t, body, &loginResp)

				token, ok := loginResp["token"].(string)
				if !ok || token == "" {
					t.Error("Expected JWT token in response")
				}
			},
		},
		{
			name: "login fails with incorrect password",
			request: map[string]interface{}{
				"email":    testEmail,
				"password": "wrongpassword",
			},
			expectedStatus: http.StatusInternalServerError,
			validateFunc: func(t *testing.T, body []byte, resp *http.Response) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)

				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
		{
			name: "login fails with non-existent email",
			request: map[string]interface{}{
				"email":    "nonexistent@example.com",
				"password": "password123",
			},
			expectedStatus: http.StatusInternalServerError,
			validateFunc: func(t *testing.T, body []byte, resp *http.Response) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)

				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
		{
			name: "login fails with invalid email format",
			request: map[string]interface{}{
				"email":    "invalid-email",
				"password": "password123",
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, body []byte, resp *http.Response) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)

				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
		{
			name: "login fails with missing email",
			request: map[string]interface{}{
				"password": "password123",
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, body []byte, resp *http.Response) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)

				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
		{
			name: "login fails with missing password",
			request: map[string]interface{}{
				"email": testEmail,
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, body []byte, resp *http.Response) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)

				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(t, http.MethodPost, "/api/auth/login/email", tt.request, nil)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body, resp)
			}
		})
	}
}

func TestAPI_Auth_GetUserInfo(t *testing.T) {
	// Create test user directly in database and login to get token
	token := createAuthenticatedTestUser(t)

	tests := []struct {
		name           string
		token          string
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name:           "get user info with valid token",
			token:          token,
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var user map[string]interface{}
				parseJSONResponse(t, body, &user)

				// Note: The processor.User struct has external_id, first_name, last_name (not id)
				if user["external_id"] == nil {
					t.Error("Expected external_id in response")
				}
				if user["first_name"] != "Test" {
					t.Errorf("Expected first_name to be 'Test', got %v", user["first_name"])
				}
				if user["last_name"] != "User" {
					t.Errorf("Expected last_name to be 'User', got %v", user["last_name"])
				}
			},
		},
		{
			name:           "get user info fails without token",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
			validateFunc: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)

				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
		{
			name:           "get user info fails with invalid token",
			token:          "invalid-token",
			expectedStatus: http.StatusUnauthorized,
			validateFunc: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)

				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeAuthenticatedRequest(t, http.MethodGet, "/api/protected/user", nil, tt.token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}
