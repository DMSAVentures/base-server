//go:build integration
// +build integration

package tests

import (
	"fmt"
	"net/http"
	"testing"
)

// Helper to create authenticated user and return token
func createAuthenticatedUser(t *testing.T) string {
	signupReq := map[string]interface{}{
		"first_name": "Campaign",
		"last_name":  "Tester",
		"email":      generateTestEmail(),
		"password":   "testpassword123",
	}
	signupResp, signupBody := makeRequest(t, http.MethodPost, "/api/auth/signup/email", signupReq, nil)
	if signupResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to create test user: %s", string(signupBody))
	}

	loginReq := map[string]interface{}{
		"email":    signupReq["email"],
		"password": "testpassword123",
	}
	loginResp, loginBody := makeRequest(t, http.MethodPost, "/api/auth/login/email", loginReq, nil)
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to login test user: %s", string(loginBody))
	}

	var loginRespData map[string]interface{}
	parseJSONResponse(t, loginBody, &loginRespData)
	return loginRespData["token"].(string)
}

func TestAPI_Campaign_Create(t *testing.T) {
	token := createAuthenticatedUser(t)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name: "create waitlist campaign successfully",
			request: map[string]interface{}{
				"name":             "Test Waitlist Campaign",
				"slug":             generateTestCampaignSlug(),
				"description":      "A test waitlist campaign",
				"type":             "waitlist",
				"form_config":      map[string]interface{}{},
				"referral_config":  map[string]interface{}{},
				"email_config":     map[string]interface{}{},
				"branding_config":  map[string]interface{}{},
			},
			expectedStatus: http.StatusCreated,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)

				if campaign["id"] == nil {
					t.Error("Expected campaign ID in response")
				}
				if campaign["name"] != "Test Waitlist Campaign" {
					t.Error("Expected name to match request")
				}
				if campaign["type"] != "waitlist" {
					t.Error("Expected type to be 'waitlist'")
				}
				if campaign["status"] != "draft" {
					t.Error("Expected initial status to be 'draft'")
				}
			},
		},
		{
			name: "create referral campaign successfully",
			request: map[string]interface{}{
				"name":            "Test Referral Campaign",
				"slug":            generateTestCampaignSlug(),
				"type":            "referral",
				"form_config":     map[string]interface{}{},
				"referral_config": map[string]interface{}{},
				"email_config":    map[string]interface{}{},
				"branding_config": map[string]interface{}{},
			},
			expectedStatus: http.StatusCreated,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)

				if campaign["type"] != "referral" {
					t.Error("Expected type to be 'referral'")
				}
			},
		},
		{
			name: "create contest campaign successfully",
			request: map[string]interface{}{
				"name":            "Test Contest Campaign",
				"slug":            generateTestCampaignSlug(),
				"type":            "contest",
				"form_config":     map[string]interface{}{},
				"referral_config": map[string]interface{}{},
				"email_config":    map[string]interface{}{},
				"branding_config": map[string]interface{}{},
			},
			expectedStatus: http.StatusCreated,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)

				if campaign["type"] != "contest" {
					t.Error("Expected type to be 'contest'")
				}
			},
		},
		{
			name: "create campaign with optional fields",
			request: map[string]interface{}{
				"name":               "Full Featured Campaign",
				"slug":               generateTestCampaignSlug(),
				"type":               "waitlist",
				"description":        "Campaign with all fields",
				"privacy_policy_url": "https://example.com/privacy",
				"terms_url":          "https://example.com/terms",
				"max_signups":        1000,
				"form_config":        map[string]interface{}{},
				"referral_config":    map[string]interface{}{},
				"email_config":       map[string]interface{}{},
				"branding_config":    map[string]interface{}{},
			},
			expectedStatus: http.StatusCreated,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)

				if campaign["description"] != "Campaign with all fields" {
					t.Error("Expected description to match request")
				}
				if campaign["privacy_policy_url"] != "https://example.com/privacy" {
					t.Error("Expected privacy_policy_url to match request")
				}
				if campaign["terms_url"] != "https://example.com/terms" {
					t.Error("Expected terms_url to match request")
				}
			},
		},
		{
			name: "create fails without name",
			request: map[string]interface{}{
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
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
			name: "create fails without slug",
			request: map[string]interface{}{
				"name": "Test Campaign",
				"type": "waitlist",
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
			name: "create fails without type",
			request: map[string]interface{}{
				"name": "Test Campaign",
				"slug": generateTestCampaignSlug(),
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
			name: "create fails with invalid type",
			request: map[string]interface{}{
				"name": "Test Campaign",
				"slug": generateTestCampaignSlug(),
				"type": "invalid_type",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", tt.request, token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

func TestAPI_Campaign_List(t *testing.T) {
	token := createAuthenticatedUser(t)

	// Create a few test campaigns
	for i := 0; i < 3; i++ {
		req := map[string]interface{}{
			"name":            fmt.Sprintf("List Test Campaign %d", i),
			"slug":            generateTestCampaignSlug(),
			"type":            "waitlist",
			"form_config":     map[string]interface{}{},
			"referral_config": map[string]interface{}{},
			"email_config":    map[string]interface{}{},
			"branding_config": map[string]interface{}{},
		}
		makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", req, token)
	}

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name:           "list all campaigns without filters",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				campaigns, ok := response["campaigns"].([]interface{})
				if !ok {
					t.Fatal("Expected 'campaigns' array in response")
				}

				if len(campaigns) < 3 {
					t.Errorf("Expected at least 3 campaigns, got %d", len(campaigns))
				}

				pagination, ok := response["pagination"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected 'pagination' object in response")
				}

				if pagination["page"] == nil || pagination["page_size"] == nil {
					t.Error("Expected pagination details")
				}
			},
		},
		{
			name:           "list campaigns with pagination",
			queryParams:    "?page=1&limit=2",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				campaigns, ok := response["campaigns"].([]interface{})
				if !ok {
					t.Fatal("Expected 'campaigns' array in response")
				}

				if len(campaigns) > 2 {
					t.Errorf("Expected max 2 campaigns with limit=2, got %d", len(campaigns))
				}
			},
		},
		{
			name:           "list campaigns filtered by status",
			queryParams:    "?status=draft",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				campaigns, ok := response["campaigns"].([]interface{})
				if !ok {
					t.Fatal("Expected 'campaigns' array in response")
				}

				for _, c := range campaigns {
					campaign := c.(map[string]interface{})
					if campaign["status"] != "draft" {
						t.Error("Expected all campaigns to have status 'draft'")
					}
				}
			},
		},
		{
			name:           "list campaigns filtered by type",
			queryParams:    "?type=waitlist",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				campaigns, ok := response["campaigns"].([]interface{})
				if !ok {
					t.Fatal("Expected 'campaigns' array in response")
				}

				for _, c := range campaigns {
					campaign := c.(map[string]interface{})
					if campaign["type"] != "waitlist" {
						t.Error("Expected all campaigns to have type 'waitlist'")
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/v1/campaigns" + tt.queryParams
			resp, body := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

func TestAPI_Campaign_GetByID(t *testing.T) {
	token := createAuthenticatedUser(t)

	// Create a test campaign
	createReq := map[string]interface{}{
		"name":            "Get By ID Test Campaign",
		"slug":            generateTestCampaignSlug(),
		"type":            "waitlist",
		"form_config":     map[string]interface{}{},
		"referral_config": map[string]interface{}{},
		"email_config":    map[string]interface{}{},
		"branding_config": map[string]interface{}{},
	}
	createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", createReq, token)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test campaign: %s", string(createBody))
	}

	var createdCampaign map[string]interface{}
	parseJSONResponse(t, createBody, &createdCampaign)
	campaignID := createdCampaign["id"].(string)

	tests := []struct {
		name           string
		campaignID     string
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name:           "get campaign by valid ID",
			campaignID:     campaignID,
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)

				if campaign["id"] != campaignID {
					t.Errorf("Expected campaign ID %s, got %v", campaignID, campaign["id"])
				}
				if campaign["name"] != "Get By ID Test Campaign" {
					t.Error("Expected name to match created campaign")
				}
			},
		},
		{
			name:           "get campaign fails with invalid UUID",
			campaignID:     "invalid-uuid",
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
			name:           "get campaign fails with non-existent UUID",
			campaignID:     "00000000-0000-0000-0000-000000000000",
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
			path := fmt.Sprintf("/api/v1/campaigns/%s", tt.campaignID)
			resp, body := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

func TestAPI_Campaign_Update(t *testing.T) {
	token := createAuthenticatedUser(t)

	// Create a test campaign
	createReq := map[string]interface{}{
		"name":            "Update Test Campaign",
		"slug":            generateTestCampaignSlug(),
		"type":            "waitlist",
		"form_config":     map[string]interface{}{},
		"referral_config": map[string]interface{}{},
		"email_config":    map[string]interface{}{},
		"branding_config": map[string]interface{}{},
	}
	createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", createReq, token)
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
			name:       "update campaign name",
			campaignID: campaignID,
			request: map[string]interface{}{
				"name": "Updated Campaign Name",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)

				if campaign["name"] != "Updated Campaign Name" {
					t.Error("Expected name to be updated")
				}
			},
		},
		{
			name:       "update campaign description",
			campaignID: campaignID,
			request: map[string]interface{}{
				"description": "Updated description",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)

				if campaign["description"] != "Updated description" {
					t.Error("Expected description to be updated")
				}
			},
		},
		{
			name:       "update multiple fields",
			campaignID: campaignID,
			request: map[string]interface{}{
				"name":               "Multi Update Campaign",
				"description":        "Multi field update",
				"privacy_policy_url": "https://example.com/privacy",
				"terms_url":          "https://example.com/terms",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)

				if campaign["name"] != "Multi Update Campaign" {
					t.Error("Expected name to be updated")
				}
				if campaign["description"] != "Multi field update" {
					t.Error("Expected description to be updated")
				}
			},
		},
		{
			name:           "update fails with invalid campaign ID",
			campaignID:     "invalid-uuid",
			request:        map[string]interface{}{"name": "New Name"},
			expectedStatus: http.StatusBadRequest,
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
			path := fmt.Sprintf("/api/v1/campaigns/%s", tt.campaignID)
			resp, body := makeAuthenticatedRequest(t, http.MethodPut, path, tt.request, token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

func TestAPI_Campaign_UpdateStatus(t *testing.T) {
	token := createAuthenticatedUser(t)

	// Create a test campaign
	createReq := map[string]interface{}{
		"name":            "Status Update Test Campaign",
		"slug":            generateTestCampaignSlug(),
		"type":            "waitlist",
		"form_config":     map[string]interface{}{},
		"referral_config": map[string]interface{}{},
		"email_config":    map[string]interface{}{},
		"branding_config": map[string]interface{}{},
	}
	createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", createReq, token)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test campaign: %s", string(createBody))
	}

	var createdCampaign map[string]interface{}
	parseJSONResponse(t, createBody, &createdCampaign)
	campaignID := createdCampaign["id"].(string)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name:           "update status to active",
			request:        map[string]interface{}{"status": "active"},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)

				if campaign["status"] != "active" {
					t.Error("Expected status to be 'active'")
				}
			},
		},
		{
			name:           "update status to paused",
			request:        map[string]interface{}{"status": "paused"},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)

				if campaign["status"] != "paused" {
					t.Error("Expected status to be 'paused'")
				}
			},
		},
		{
			name:           "update status to completed",
			request:        map[string]interface{}{"status": "completed"},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)

				if campaign["status"] != "completed" {
					t.Error("Expected status to be 'completed'")
				}
			},
		},
		{
			name:           "update status to draft",
			request:        map[string]interface{}{"status": "draft"},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)

				if campaign["status"] != "draft" {
					t.Error("Expected status to be 'draft'")
				}
			},
		},
		{
			name:           "update fails with invalid status",
			request:        map[string]interface{}{"status": "invalid_status"},
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
			name:           "update fails without status field",
			request:        map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
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
			path := fmt.Sprintf("/api/v1/campaigns/%s/status", campaignID)
			resp, body := makeAuthenticatedRequest(t, http.MethodPatch, path, tt.request, token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

func TestAPI_Campaign_Delete(t *testing.T) {
	token := createAuthenticatedUser(t)

	tests := []struct {
		name           string
		setupFunc      func() string
		campaignID     string
		expectedStatus int
		validateFunc   func(t *testing.T, campaignID string)
	}{
		{
			name: "delete campaign successfully",
			setupFunc: func() string {
				req := map[string]interface{}{
					"name":            "Delete Test Campaign",
					"slug":            generateTestCampaignSlug(),
					"type":            "waitlist",
					"form_config":     map[string]interface{}{},
					"referral_config": map[string]interface{}{},
					"email_config":    map[string]interface{}{},
					"branding_config": map[string]interface{}{},
				}
				resp, body := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", req, token)
				if resp.StatusCode != http.StatusCreated {
					t.Fatalf("Failed to create test campaign: %s", string(body))
				}
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)
				return campaign["id"].(string)
			},
			expectedStatus: http.StatusNoContent,
			validateFunc: func(t *testing.T, campaignID string) {
				// Try to get the deleted campaign
				path := fmt.Sprintf("/api/v1/campaigns/%s", campaignID)
				resp, _ := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)
				if resp.StatusCode != http.StatusNotFound {
					t.Error("Expected campaign to be deleted")
				}
			},
		},
		{
			name:           "delete fails with invalid campaign ID",
			campaignID:     "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			validateFunc:   nil,
		},
		{
			name:           "delete fails with non-existent campaign ID",
			campaignID:     "00000000-0000-0000-0000-000000000000",
			expectedStatus: http.StatusNotFound,
			validateFunc:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			campaignID := tt.campaignID
			if tt.setupFunc != nil {
				campaignID = tt.setupFunc()
			}

			path := fmt.Sprintf("/api/v1/campaigns/%s", campaignID)
			resp, _ := makeAuthenticatedRequest(t, http.MethodDelete, path, nil, token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, campaignID)
			}
		})
	}
}
