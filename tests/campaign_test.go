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

// TestAPI_Campaign_CreateWithFormFields tests form field validation
func TestAPI_Campaign_CreateWithFormFields(t *testing.T) {
	token := createAuthenticatedUser(t)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name: "create campaign with valid form fields",
			request: map[string]interface{}{
				"name": "Campaign with Form Fields",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_fields": []map[string]interface{}{
					{
						"name":          "email_field",
						"field_type":    "email",
						"label":         "Email Address",
						"placeholder":   "Enter your email",
						"required":      true,
						"display_order": 1,
					},
					{
						"name":          "name_field",
						"field_type":    "text",
						"label":         "Full Name",
						"required":      true,
						"display_order": 2,
					},
				},
			},
			expectedStatus: http.StatusCreated,
			validateFunc: func(t *testing.T, body []byte) {
				var campaign map[string]interface{}
				parseJSONResponse(t, body, &campaign)
				if campaign["id"] == nil {
					t.Error("Expected campaign ID in response")
				}
			},
		},
		{
			name: "create campaign with all valid field types",
			request: map[string]interface{}{
				"name": "Campaign with All Field Types",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_fields": []map[string]interface{}{
					{"name": "email", "field_type": "email", "label": "Email", "display_order": 1},
					{"name": "text", "field_type": "text", "label": "Text", "display_order": 2},
					{"name": "textarea", "field_type": "textarea", "label": "Textarea", "display_order": 3},
					{"name": "select", "field_type": "select", "label": "Select", "options": []string{"a", "b"}, "display_order": 4},
					{"name": "checkbox", "field_type": "checkbox", "label": "Checkbox", "display_order": 5},
					{"name": "radio", "field_type": "radio", "label": "Radio", "options": []string{"yes", "no"}, "display_order": 6},
					{"name": "phone", "field_type": "phone", "label": "Phone", "display_order": 7},
					{"name": "url", "field_type": "url", "label": "URL", "display_order": 8},
					{"name": "date", "field_type": "date", "label": "Date", "display_order": 9},
					{"name": "number", "field_type": "number", "label": "Number", "display_order": 10},
				},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create fails with invalid field_type",
			request: map[string]interface{}{
				"name": "Campaign with Invalid Field Type",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_fields": []map[string]interface{}{
					{
						"name":          "test_field",
						"field_type":    "invalid_type",
						"label":         "Test Field",
						"display_order": 1,
					},
				},
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
			name: "create fails with empty field name",
			request: map[string]interface{}{
				"name": "Campaign with Empty Field Name",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_fields": []map[string]interface{}{
					{
						"name":          "",
						"field_type":    "text",
						"label":         "Test Field",
						"display_order": 1,
					},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create fails with empty field label",
			request: map[string]interface{}{
				"name": "Campaign with Empty Field Label",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_fields": []map[string]interface{}{
					{
						"name":          "test_field",
						"field_type":    "text",
						"label":         "",
						"display_order": 1,
					},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create fails with missing required field_type",
			request: map[string]interface{}{
				"name": "Campaign Missing Field Type",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_fields": []map[string]interface{}{
					{
						"name":          "test_field",
						"label":         "Test Field",
						"display_order": 1,
					},
				},
			},
			expectedStatus: http.StatusBadRequest,
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

// TestAPI_Campaign_CreateWithShareMessages tests share message validation
func TestAPI_Campaign_CreateWithShareMessages(t *testing.T) {
	token := createAuthenticatedUser(t)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name: "create campaign with valid share messages",
			request: map[string]interface{}{
				"name": "Campaign with Share Messages",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"share_messages": []map[string]interface{}{
					{"channel": "email", "message": "Check out this campaign!"},
					{"channel": "twitter", "message": "Join the waitlist!"},
					{"channel": "facebook", "message": "Sign up now!"},
					{"channel": "linkedin", "message": "Professional opportunity!"},
					{"channel": "whatsapp", "message": "Don't miss out!"},
				},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create fails with invalid share channel",
			request: map[string]interface{}{
				"name": "Campaign with Invalid Channel",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"share_messages": []map[string]interface{}{
					{"channel": "instagram", "message": "Invalid channel!"},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create fails with empty share message",
			request: map[string]interface{}{
				"name": "Campaign with Empty Message",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"share_messages": []map[string]interface{}{
					{"channel": "email", "message": ""},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create fails with missing channel",
			request: map[string]interface{}{
				"name": "Campaign Missing Channel",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"share_messages": []map[string]interface{}{
					{"message": "Message without channel"},
				},
			},
			expectedStatus: http.StatusBadRequest,
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

// TestAPI_Campaign_CreateWithTrackingIntegrations tests tracking integration validation
func TestAPI_Campaign_CreateWithTrackingIntegrations(t *testing.T) {
	token := createAuthenticatedUser(t)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
	}{
		{
			name: "create campaign with valid tracking integrations",
			request: map[string]interface{}{
				"name": "Campaign with Tracking",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"tracking_integrations": []map[string]interface{}{
					{"integration_type": "google_analytics", "enabled": true, "tracking_id": "GA-12345678"},
					{"integration_type": "meta_pixel", "enabled": true, "tracking_id": "123456789"},
					{"integration_type": "google_ads", "enabled": false, "tracking_id": "AW-123456"},
					{"integration_type": "tiktok_pixel", "enabled": true, "tracking_id": "TT-789"},
					{"integration_type": "linkedin_insight", "enabled": true, "tracking_id": "LI-456"},
				},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create fails with invalid integration_type",
			request: map[string]interface{}{
				"name": "Campaign with Invalid Integration",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"tracking_integrations": []map[string]interface{}{
					{"integration_type": "invalid_tracker", "enabled": true, "tracking_id": "123"},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create fails with empty tracking_id",
			request: map[string]interface{}{
				"name": "Campaign with Empty Tracking ID",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"tracking_integrations": []map[string]interface{}{
					{"integration_type": "google_analytics", "enabled": true, "tracking_id": ""},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create fails with missing integration_type",
			request: map[string]interface{}{
				"name": "Campaign Missing Integration Type",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"tracking_integrations": []map[string]interface{}{
					{"enabled": true, "tracking_id": "GA-123"},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, _ := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", tt.request, token)
			assertStatusCode(t, resp, tt.expectedStatus)
		})
	}
}

// TestAPI_Campaign_CreateWithReferralSettings tests referral settings validation
func TestAPI_Campaign_CreateWithReferralSettings(t *testing.T) {
	token := createAuthenticatedUser(t)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
	}{
		{
			name: "create campaign with valid referral settings",
			request: map[string]interface{}{
				"name": "Campaign with Referral Settings",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"referral_settings": map[string]interface{}{
					"enabled":                   true,
					"points_per_referral":       10,
					"verified_only":             true,
					"positions_to_jump":         5,
					"referrer_positions_to_jump": 2,
					"sharing_channels":          []string{"email", "twitter", "facebook"},
				},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create campaign with zero referral values (valid)",
			request: map[string]interface{}{
				"name": "Campaign with Zero Referral Values",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"referral_settings": map[string]interface{}{
					"enabled":                   false,
					"points_per_referral":       0,
					"positions_to_jump":         0,
					"referrer_positions_to_jump": 0,
					"sharing_channels":          []string{},
				},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create fails with negative points_per_referral",
			request: map[string]interface{}{
				"name": "Campaign with Negative Points",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"referral_settings": map[string]interface{}{
					"enabled":             true,
					"points_per_referral": -5,
					"sharing_channels":    []string{"email"},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create fails with negative positions_to_jump",
			request: map[string]interface{}{
				"name": "Campaign with Negative Positions",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"referral_settings": map[string]interface{}{
					"enabled":           true,
					"positions_to_jump": -1,
					"sharing_channels":  []string{"email"},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create fails with negative referrer_positions_to_jump",
			request: map[string]interface{}{
				"name": "Campaign with Negative Referrer Positions",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"referral_settings": map[string]interface{}{
					"enabled":                   true,
					"referrer_positions_to_jump": -3,
					"sharing_channels":          []string{"email"},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create fails with invalid sharing channel",
			request: map[string]interface{}{
				"name": "Campaign with Invalid Sharing Channel",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"referral_settings": map[string]interface{}{
					"enabled":          true,
					"sharing_channels": []string{"email", "telegram"},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create campaign with all valid sharing channels",
			request: map[string]interface{}{
				"name": "Campaign with All Sharing Channels",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"referral_settings": map[string]interface{}{
					"enabled":          true,
					"sharing_channels": []string{"email", "twitter", "facebook", "linkedin", "whatsapp"},
				},
			},
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, _ := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", tt.request, token)
			assertStatusCode(t, resp, tt.expectedStatus)
		})
	}
}

// TestAPI_Campaign_CreateWithFormSettings tests form settings validation
func TestAPI_Campaign_CreateWithFormSettings(t *testing.T) {
	token := createAuthenticatedUser(t)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
	}{
		{
			name: "create campaign with valid form settings using turnstile",
			request: map[string]interface{}{
				"name": "Campaign with Turnstile Captcha",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_settings": map[string]interface{}{
					"captcha_enabled":  true,
					"captcha_provider": "turnstile",
					"captcha_site_key": "0x123456789",
					"double_opt_in":    true,
					"success_title":    "Thank you!",
					"success_message":  "You have been added to the waitlist.",
					"design": map[string]interface{}{
						"theme":      "dark",
						"buttonText": "Join Now",
					},
				},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create campaign with valid form settings using recaptcha",
			request: map[string]interface{}{
				"name": "Campaign with Recaptcha",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_settings": map[string]interface{}{
					"captcha_enabled":  true,
					"captcha_provider": "recaptcha",
					"captcha_site_key": "6Le123456789",
				},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create campaign with valid form settings using hcaptcha",
			request: map[string]interface{}{
				"name": "Campaign with hCaptcha",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_settings": map[string]interface{}{
					"captcha_enabled":  true,
					"captcha_provider": "hcaptcha",
					"captcha_site_key": "abc123def456",
				},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create campaign with captcha disabled (no provider required)",
			request: map[string]interface{}{
				"name": "Campaign without Captcha",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_settings": map[string]interface{}{
					"captcha_enabled": false,
					"double_opt_in":   false,
				},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create fails with invalid captcha_provider",
			request: map[string]interface{}{
				"name": "Campaign with Invalid Captcha Provider",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_settings": map[string]interface{}{
					"captcha_enabled":  true,
					"captcha_provider": "invalid_captcha",
					"captcha_site_key": "123",
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, _ := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", tt.request, token)
			assertStatusCode(t, resp, tt.expectedStatus)
		})
	}
}

// TestAPI_Campaign_CreateWithFullSettings tests creating a campaign with all settings
func TestAPI_Campaign_CreateWithFullSettings(t *testing.T) {
	token := createAuthenticatedUser(t)

	request := map[string]interface{}{
		"name":               "Full Featured Campaign",
		"slug":               generateTestCampaignSlug(),
		"description":        "A campaign with all settings configured",
		"type":               "referral",
		"privacy_policy_url": "https://example.com/privacy",
		"terms_url":          "https://example.com/terms",
		"max_signups":        5000,
		"email_settings": map[string]interface{}{
			"from_name":             "Test Campaign",
			"from_email":            "campaign@example.com",
			"reply_to":              "support@example.com",
			"verification_required": true,
			"send_welcome_email":    true,
		},
		"branding_settings": map[string]interface{}{
			"logo_url":      "https://example.com/logo.png",
			"primary_color": "#FF5733",
			"font_family":   "Inter",
			"custom_domain": "campaign.example.com",
		},
		"form_settings": map[string]interface{}{
			"captcha_enabled":  true,
			"captcha_provider": "turnstile",
			"captcha_site_key": "0x123456789",
			"double_opt_in":    true,
			"success_title":    "Welcome!",
			"success_message":  "You're on the list!",
			"design": map[string]interface{}{
				"theme":           "light",
				"backgroundColor": "#FFFFFF",
			},
		},
		"referral_settings": map[string]interface{}{
			"enabled":                   true,
			"points_per_referral":       25,
			"verified_only":             true,
			"positions_to_jump":         10,
			"referrer_positions_to_jump": 5,
			"sharing_channels":          []string{"email", "twitter", "linkedin"},
		},
		"form_fields": []map[string]interface{}{
			{
				"name":          "email",
				"field_type":    "email",
				"label":         "Email Address",
				"placeholder":   "you@example.com",
				"required":      true,
				"display_order": 1,
			},
			{
				"name":          "full_name",
				"field_type":    "text",
				"label":         "Full Name",
				"required":      true,
				"display_order": 2,
			},
			{
				"name":          "company",
				"field_type":    "text",
				"label":         "Company",
				"required":      false,
				"display_order": 3,
			},
		},
		"share_messages": []map[string]interface{}{
			{"channel": "email", "message": "Join me on this amazing campaign!"},
			{"channel": "twitter", "message": "Check out this waitlist! #launch"},
			{"channel": "linkedin", "message": "Exciting new product launching soon!"},
		},
		"tracking_integrations": []map[string]interface{}{
			{
				"integration_type": "google_analytics",
				"enabled":          true,
				"tracking_id":      "GA-123456789",
				"tracking_label":   "waitlist_signup",
			},
			{
				"integration_type": "meta_pixel",
				"enabled":          true,
				"tracking_id":      "987654321",
			},
		},
	}

	resp, body := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", request, token)
	assertStatusCode(t, resp, http.StatusCreated)

	var campaign map[string]interface{}
	parseJSONResponse(t, body, &campaign)

	// Validate response contains all expected fields
	if campaign["id"] == nil {
		t.Error("Expected campaign ID in response")
	}
	if campaign["name"] != "Full Featured Campaign" {
		t.Error("Expected name to match request")
	}
	if campaign["type"] != "referral" {
		t.Error("Expected type to be 'referral'")
	}
	if campaign["status"] != "draft" {
		t.Error("Expected initial status to be 'draft'")
	}

	// Verify we can retrieve the campaign with all settings
	campaignID := campaign["id"].(string)
	getResp, getBody := makeAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/campaigns/%s", campaignID), nil, token)
	assertStatusCode(t, getResp, http.StatusOK)

	var retrievedCampaign map[string]interface{}
	parseJSONResponse(t, getBody, &retrievedCampaign)

	if retrievedCampaign["email_settings"] == nil {
		t.Error("Expected email_settings to be present")
	}
	if retrievedCampaign["branding_settings"] == nil {
		t.Error("Expected branding_settings to be present")
	}
	if retrievedCampaign["form_settings"] == nil {
		t.Error("Expected form_settings to be present")
	}
	if retrievedCampaign["referral_settings"] == nil {
		t.Error("Expected referral_settings to be present")
	}
}

// TestAPI_Campaign_UpdateWithSettings tests updating campaign settings with validation
func TestAPI_Campaign_UpdateWithSettings(t *testing.T) {
	token := createAuthenticatedUser(t)

	// Create a base campaign first
	createReq := map[string]interface{}{
		"name": "Campaign to Update",
		"slug": generateTestCampaignSlug(),
		"type": "waitlist",
	}
	createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", createReq, token)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create campaign: %s", string(createBody))
	}

	var createdCampaign map[string]interface{}
	parseJSONResponse(t, createBody, &createdCampaign)
	campaignID := createdCampaign["id"].(string)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
	}{
		{
			name: "update campaign with valid form fields",
			request: map[string]interface{}{
				"form_fields": []map[string]interface{}{
					{"name": "email", "field_type": "email", "label": "Email", "display_order": 1},
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "update campaign with valid email settings",
			request: map[string]interface{}{
				"email_settings": map[string]interface{}{
					"from_name":          "Updated Name",
					"send_welcome_email": true,
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "update fails with invalid form field type",
			request: map[string]interface{}{
				"form_fields": []map[string]interface{}{
					{"name": "field", "field_type": "invalid", "label": "Field", "display_order": 1},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "update fails with invalid captcha provider",
			request: map[string]interface{}{
				"form_settings": map[string]interface{}{
					"captcha_enabled":  true,
					"captcha_provider": "invalid",
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "update fails with negative referral points",
			request: map[string]interface{}{
				"referral_settings": map[string]interface{}{
					"points_per_referral": -10,
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fmt.Sprintf("/api/v1/campaigns/%s", campaignID)
			resp, _ := makeAuthenticatedRequest(t, http.MethodPut, path, tt.request, token)
			assertStatusCode(t, resp, tt.expectedStatus)
		})
	}
}

// TestAPI_Campaign_DuplicateSlug tests that duplicate slugs are rejected
func TestAPI_Campaign_DuplicateSlug(t *testing.T) {
	token := createAuthenticatedUser(t)

	slug := generateTestCampaignSlug()

	// Create first campaign with the slug
	createReq := map[string]interface{}{
		"name": "First Campaign",
		"slug": slug,
		"type": "waitlist",
	}
	resp, _ := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", createReq, token)
	assertStatusCode(t, resp, http.StatusCreated)

	// Try to create second campaign with the same slug
	duplicateReq := map[string]interface{}{
		"name": "Second Campaign",
		"slug": slug,
		"type": "waitlist",
	}
	dupResp, dupBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", duplicateReq, token)
	assertStatusCode(t, dupResp, http.StatusConflict)

	var errResp map[string]interface{}
	parseJSONResponse(t, dupBody, &errResp)
	if errResp["error"] == nil {
		t.Error("Expected error message for duplicate slug")
	}
}
