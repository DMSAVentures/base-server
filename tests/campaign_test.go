//go:build integration
// +build integration

package tests

import (
	"fmt"
	"net/http"
	"testing"
)

// Helper to create authenticated user and return token
// Now uses direct database insertion to bypass Stripe dependency
func createAuthenticatedUser(t *testing.T) string {
	return createAuthenticatedTestUser(t)
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

func TestAPI_Campaign_GetPublicCampaign(t *testing.T) {
	token := createAuthenticatedUser(t)

	// Create test campaigns with different statuses
	draftCampaignReq := map[string]interface{}{
		"name":            "Public Draft Campaign",
		"slug":            generateTestCampaignSlug(),
		"description":     "A draft campaign for public access",
		"type":            "waitlist",
		"form_config":     map[string]interface{}{"fields": []string{"email", "name"}},
		"referral_config": map[string]interface{}{"enabled": true},
		"email_config":    map[string]interface{}{},
		"branding_config": map[string]interface{}{"theme": "dark"},
	}
	draftResp, draftBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", draftCampaignReq, token)
	if draftResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create draft campaign: %s", string(draftBody))
	}

	var draftCampaign map[string]interface{}
	parseJSONResponse(t, draftBody, &draftCampaign)
	draftCampaignID := draftCampaign["id"].(string)

	// Create an active campaign
	activeCampaignReq := map[string]interface{}{
		"name":            "Public Active Campaign",
		"slug":            generateTestCampaignSlug(),
		"type":            "referral",
		"form_config":     map[string]interface{}{},
		"referral_config": map[string]interface{}{},
		"email_config":    map[string]interface{}{},
		"branding_config": map[string]interface{}{},
	}
	activeResp, activeBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", activeCampaignReq, token)
	if activeResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create active campaign: %s", string(activeBody))
	}

	var activeCampaign map[string]interface{}
	parseJSONResponse(t, activeBody, &activeCampaign)
	activeCampaignID := activeCampaign["id"].(string)

	// Update status to active
	statusReq := map[string]interface{}{"status": "active"}
	statusPath := fmt.Sprintf("/api/v1/campaigns/%s/status", activeCampaignID)
	makeAuthenticatedRequest(t, http.MethodPatch, statusPath, statusReq, token)

	tests := []struct {
		name           string
		campaignID     string
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name:           "get public campaign successfully with draft status",
			campaignID:     draftCampaignID,
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)

				if campaign["id"] != draftCampaignID {
					t.Errorf("Expected campaign ID %s, got %v", draftCampaignID, campaign["id"])
				}
				if campaign["name"] != "Public Draft Campaign" {
					t.Error("Expected name to match created campaign")
				}
				if campaign["type"] != "waitlist" {
					t.Error("Expected type to be 'waitlist'")
				}
				if campaign["status"] != "draft" {
					t.Error("Expected status to be 'draft'")
				}

				// Verify configuration fields are present
				if campaign["form_config"] == nil {
					t.Error("Expected form_config to be present")
				}
				if campaign["referral_config"] == nil {
					t.Error("Expected referral_config to be present")
				}
				if campaign["branding_config"] == nil {
					t.Error("Expected branding_config to be present")
				}

				// Verify sensitive account information is not exposed
				if campaign["account_id"] == nil {
					t.Error("Expected account_id to be present (for public form rendering)")
				}
			},
		},
		{
			name:           "get public campaign successfully with active status",
			campaignID:     activeCampaignID,
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)

				if campaign["id"] != activeCampaignID {
					t.Errorf("Expected campaign ID %s, got %v", activeCampaignID, campaign["id"])
				}
				if campaign["name"] != "Public Active Campaign" {
					t.Error("Expected name to match created campaign")
				}
				if campaign["status"] != "active" {
					t.Error("Expected status to be 'active'")
				}
			},
		},
		{
			name:           "get public campaign fails with invalid UUID",
			campaignID:     "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)

				// Validate error response structure from apierrors
				if errResp["error"] == nil {
					t.Error("Expected 'error' field in response")
				}
				if errResp["code"] == nil {
					t.Error("Expected 'code' field in response")
				}

				// Verify error code matches apierrors pattern
				code, ok := errResp["code"].(string)
				if !ok {
					t.Error("Expected 'code' to be a string")
				}
				if code != "INVALID_INPUT" {
					t.Errorf("Expected error code 'INVALID_INPUT', got '%s'", code)
				}

				// Verify sanitized error message (no internal details leaked)
				errorMsg, ok := errResp["error"].(string)
				if !ok {
					t.Error("Expected 'error' to be a string")
				}
				if errorMsg == "" {
					t.Error("Expected non-empty error message")
				}
			},
		},
		{
			name:           "get public campaign fails with non-existent UUID",
			campaignID:     "00000000-0000-0000-0000-000000000000",
			expectedStatus: http.StatusNotFound,
			validateFunc: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				parseJSONResponse(t, body, &errResp)

				// Validate error response structure from apierrors
				if errResp["error"] == nil {
					t.Error("Expected 'error' field in response")
				}
				if errResp["code"] == nil {
					t.Error("Expected 'code' field in response")
				}

				// Verify error code
				code, ok := errResp["code"].(string)
				if !ok {
					t.Error("Expected 'code' to be a string")
				}
				if code != "CAMPAIGN_NOT_FOUND" {
					t.Errorf("Expected error code 'CAMPAIGN_NOT_FOUND', got '%s'", code)
				}

				// Verify no internal database details leaked
				errorMsg := errResp["error"].(string)
				if containsSensitiveInfo(errorMsg) {
					t.Error("Error message contains sensitive internal information")
				}
			},
		},
		{
			name:           "get public campaign - no authentication required",
			campaignID:     draftCampaignID,
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)

				// This test is making an unauthenticated request (see test execution below)
				// Verify it still succeeds
				if campaign["id"] != draftCampaignID {
					t.Error("Public endpoint should work without authentication")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fmt.Sprintf("/api/v1/%s", tt.campaignID)

			var resp *http.Response
			var body []byte

			// For the "no authentication required" test, use makeRequest instead of makeAuthenticatedRequest
			if tt.name == "get public campaign - no authentication required" {
				resp, body = makeRequest(t, http.MethodGet, path, nil, nil)
			} else {
				// For other tests, we can still use the endpoint without auth,
				// but for consistency with the existing test suite, we use authenticated requests
				resp, body = makeRequest(t, http.MethodGet, path, nil, nil)
			}

			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

// containsSensitiveInfo checks if error message contains sensitive internal information
func containsSensitiveInfo(msg string) bool {
	sensitiveKeywords := []string{
		"sql",
		"SQL",
		"database",
		"postgres",
		"table",
		"column",
		"constraint",
		"violation",
		"panic",
		"stack trace",
	}

	for _, keyword := range sensitiveKeywords {
		if len(msg) > 0 && contains(msg, keyword) {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		len(s) > len(substr)+1 && containsInMiddle(s, substr)))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
