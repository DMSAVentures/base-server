//go:build integration
// +build integration

package tests

import (
	"fmt"
	"net/http"
	"testing"
)

func TestAPI_WaitlistUser_PublicSignup(t *testing.T) {
	// First create an authenticated user and a campaign
	token := createAuthenticatedUser(t)

	// Create a test campaign
	createCampaignReq := map[string]interface{}{
		"name":            "Public Waitlist Test Campaign",
		"slug":            generateTestCampaignSlug(),
		"type":            "waitlist",
		"form_config":     map[string]interface{}{},
		"referral_config": map[string]interface{}{},
		"email_config":    map[string]interface{}{},
		"branding_config": map[string]interface{}{},
	}
	createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", createCampaignReq, token)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test campaign: %s", string(createBody))
	}

	var createdCampaign map[string]interface{}
	parseJSONResponse(t, createBody, &createdCampaign)
	campaignID := createdCampaign["id"].(string)

	tests := []struct {
		name           string
		campaignID     string
		request        map[string]interface{}
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name:       "public signup with minimal fields (email and terms only)",
			campaignID: campaignID,
			request: map[string]interface{}{
				"email":          "minimal@example.com",
				"terms_accepted": true,
			},
			expectedStatus: http.StatusCreated,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				user, ok := response["user"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected 'user' object in response")
				}

				if user["email"] != "minimal@example.com" {
					t.Error("Expected email to match request")
				}

				if response["position"] == nil {
					t.Error("Expected position in response")
				}

				if response["referral_link"] == nil {
					t.Error("Expected referral_link in response")
				}

				if response["message"] == nil {
					t.Error("Expected message in response")
				}
			},
		},
		{
			name:       "public signup with custom fields",
			campaignID: campaignID,
			request: map[string]interface{}{
				"email":          "custom@example.com",
				"terms_accepted": true,
				"custom_fields": map[string]string{
					"first_name": "John",
					"last_name":  "Doe",
					"company":    "Acme Inc",
				},
			},
			expectedStatus: http.StatusCreated,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				user, ok := response["user"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected 'user' object in response")
				}

				if user["email"] != "custom@example.com" {
					t.Error("Expected email to match request")
				}

				metadata, ok := user["metadata"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected 'metadata' object in user")
				}

				if metadata["first_name"] != "John" {
					t.Error("Expected first_name in metadata")
				}

				if metadata["company"] != "Acme Inc" {
					t.Error("Expected company in metadata")
				}
			},
		},
		{
			name:       "public signup with UTM parameters",
			campaignID: campaignID,
			request: map[string]interface{}{
				"email":          "utm-test@example.com",
				"terms_accepted": true,
				"utm_source":     "facebook",
				"utm_medium":     "social",
				"utm_campaign":   "spring-launch",
				"custom_fields": map[string]string{
					"first_name": "Jane",
					"last_name":  "Smith",
				},
			},
			expectedStatus: http.StatusCreated,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				user, ok := response["user"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected 'user' object in response")
				}

				if user["utm_source"] != "facebook" {
					t.Error("Expected utm_source to be saved")
				}
			},
		},
		{
			name:       "public signup fails with duplicate email",
			campaignID: campaignID,
			request: map[string]interface{}{
				"email":          "minimal@example.com", // Same as first test
				"terms_accepted": true,
			},
			expectedStatus: http.StatusConflict,
			validateFunc: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)
				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
		{
			name:       "public signup fails without email",
			campaignID: campaignID,
			request: map[string]interface{}{
				"terms_accepted": true,
				// Missing email
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)
				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
		{
			name:       "public signup fails without terms_accepted",
			campaignID: campaignID,
			request: map[string]interface{}{
				"email": "noterms@example.com",
				// Missing terms_accepted
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)
				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
		{
			name:       "public signup fails with invalid email",
			campaignID: campaignID,
			request: map[string]interface{}{
				"email":          "not-an-email",
				"terms_accepted": true,
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)
				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
		{
			name:       "public signup fails with invalid campaign ID",
			campaignID: "invalid-uuid",
			request: map[string]interface{}{
				"email":          "valid@example.com",
				"terms_accepted": true,
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)
				if errResp["error"] == nil {
					t.Error("Expected error message in response")
				}
			},
		},
		{
			name:       "public signup fails with non-existent campaign",
			campaignID: "00000000-0000-0000-0000-000000000000",
			request: map[string]interface{}{
				"email":          "nonexistent@example.com",
				"terms_accepted": true,
			},
			expectedStatus: http.StatusNotFound,
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
			path := fmt.Sprintf("/api/v1/campaigns/%s/users", tt.campaignID)
			// Note: Using makeRequest instead of makeAuthenticatedRequest for public endpoint
			resp, body := makeRequest(t, http.MethodPost, path, tt.request, nil)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}
