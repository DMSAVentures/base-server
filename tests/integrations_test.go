//go:build integration
// +build integration

package tests

import (
	"net/http"
	"testing"
)

func TestAPI_Integrations_ZapierStatus(t *testing.T) {
	t.Parallel()

	// Create authenticated test user
	token := createAuthenticatedTestUser(t)

	tests := []struct {
		name           string
		token          string
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name:           "get status with valid token - not connected",
			token:          token,
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var status map[string]interface{}
				parseJSONResponse(t, body, &status)

				// A new user should not be connected to Zapier
				if status["connected"] == nil {
					t.Error("Expected connected field in response")
				}
				if status["connected"] != false {
					t.Errorf("Expected connected to be false for new user, got %v", status["connected"])
				}
				if status["active_subscriptions"] == nil {
					t.Error("Expected active_subscriptions field in response")
				}
				if status["active_subscriptions"].(float64) != 0 {
					t.Errorf("Expected active_subscriptions to be 0 for new user, got %v", status["active_subscriptions"])
				}
			},
		},
		{
			name:           "get status fails without token",
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
			name:           "get status fails with invalid token",
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
			resp, body := makeAuthenticatedRequest(t, http.MethodGet, "/api/protected/integrations/zapier/status", nil, tt.token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

func TestAPI_Integrations_ZapierSubscriptions(t *testing.T) {
	t.Parallel()

	// Create authenticated test user
	token := createAuthenticatedTestUser(t)

	tests := []struct {
		name           string
		token          string
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name:           "get subscriptions with valid token - empty list",
			token:          token,
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var subscriptions []map[string]interface{}
				parseJSONResponse(t, body, &subscriptions)

				// A new user should have no subscriptions
				if len(subscriptions) != 0 {
					t.Errorf("Expected empty subscriptions list for new user, got %d", len(subscriptions))
				}
			},
		},
		{
			name:           "get subscriptions fails without token",
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
			name:           "get subscriptions fails with invalid token",
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
			resp, body := makeAuthenticatedRequest(t, http.MethodGet, "/api/protected/integrations/zapier/subscriptions", nil, tt.token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

func TestAPI_Integrations_ZapierDisconnect(t *testing.T) {
	t.Parallel()

	// Create authenticated test user
	token := createAuthenticatedTestUser(t)

	tests := []struct {
		name           string
		token          string
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name:           "disconnect with valid token - succeeds even if not connected",
			token:          token,
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				parseJSONResponse(t, body, &resp)

				if resp["success"] == nil {
					t.Error("Expected success field in response")
				}
				if resp["success"] != true {
					t.Errorf("Expected success to be true, got %v", resp["success"])
				}
			},
		},
		{
			name:           "disconnect fails without token",
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
			name:           "disconnect fails with invalid token",
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
			resp, body := makeAuthenticatedRequest(t, http.MethodPost, "/api/protected/integrations/zapier/disconnect", nil, tt.token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

func TestAPI_Integrations_ZapierStatusWithFluentAPI(t *testing.T) {
	t.Parallel()

	token := createAuthenticatedTestUser(t)

	t.Run("get status returns connected status", func(t *testing.T) {
		t.Parallel()

		GET(t, "/api/protected/integrations/zapier/status").
			WithToken(token).
			Do().
			RequireStatus(http.StatusOK).
			AssertJSONField("connected", false).
			AssertJSONField("active_subscriptions", float64(0))
	})

	t.Run("get status without auth returns unauthorized", func(t *testing.T) {
		t.Parallel()

		GET(t, "/api/protected/integrations/zapier/status").
			Do().
			RequireStatus(http.StatusUnauthorized).
			AssertError()
	})
}

func TestAPI_Integrations_ZapierSubscriptionsWithFluentAPI(t *testing.T) {
	t.Parallel()

	token := createAuthenticatedTestUser(t)

	t.Run("get subscriptions returns empty list for new user", func(t *testing.T) {
		t.Parallel()

		resp := GET(t, "/api/protected/integrations/zapier/subscriptions").
			WithToken(token).
			Do().
			RequireStatus(http.StatusOK)

		// Check it's an empty array
		var subscriptions []map[string]interface{}
		parseJSONResponse(t, resp.Body, &subscriptions)
		if len(subscriptions) != 0 {
			t.Errorf("Expected empty subscriptions, got %d", len(subscriptions))
		}
	})

	t.Run("get subscriptions without auth returns unauthorized", func(t *testing.T) {
		t.Parallel()

		GET(t, "/api/protected/integrations/zapier/subscriptions").
			Do().
			RequireStatus(http.StatusUnauthorized).
			AssertError()
	})
}

func TestAPI_Integrations_ZapierDisconnectWithFluentAPI(t *testing.T) {
	t.Parallel()

	token := createAuthenticatedTestUser(t)

	t.Run("disconnect succeeds for user not connected", func(t *testing.T) {
		t.Parallel()

		POST(t, "/api/protected/integrations/zapier/disconnect").
			WithToken(token).
			Do().
			RequireStatus(http.StatusOK).
			AssertJSONField("success", true)
	})

	t.Run("disconnect without auth returns unauthorized", func(t *testing.T) {
		t.Parallel()

		POST(t, "/api/protected/integrations/zapier/disconnect").
			Do().
			RequireStatus(http.StatusUnauthorized).
			AssertError()
	})
}

func TestAPI_Integrations_ZapierWorkflow(t *testing.T) {
	t.Parallel()

	// This test verifies the full workflow:
	// 1. User starts disconnected from Zapier
	// 2. Status shows not connected
	// 3. Subscriptions list is empty
	// 4. Disconnect works even when not connected

	token := createAuthenticatedTestUser(t)

	t.Run("complete workflow for new user", func(t *testing.T) {
		// Step 1: Check initial status
		GET(t, "/api/protected/integrations/zapier/status").
			WithToken(token).
			Do().
			RequireStatus(http.StatusOK).
			AssertJSONField("connected", false).
			AssertJSONField("active_subscriptions", float64(0))

		// Step 2: Check subscriptions is empty
		subsResp := GET(t, "/api/protected/integrations/zapier/subscriptions").
			WithToken(token).
			Do().
			RequireStatus(http.StatusOK)

		var subs []map[string]interface{}
		parseJSONResponse(t, subsResp.Body, &subs)
		if len(subs) != 0 {
			t.Errorf("Expected empty subscriptions, got %d", len(subs))
		}

		// Step 3: Disconnect (should succeed even if not connected)
		POST(t, "/api/protected/integrations/zapier/disconnect").
			WithToken(token).
			Do().
			RequireStatus(http.StatusOK).
			AssertJSONField("success", true)

		// Step 4: Verify still shows not connected after disconnect
		GET(t, "/api/protected/integrations/zapier/status").
			WithToken(token).
			Do().
			RequireStatus(http.StatusOK).
			AssertJSONField("connected", false)
	})
}
