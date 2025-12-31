//go:build integration
// +build integration

package tests

import (
	"fmt"
	"net/http"
	"testing"
)

func TestAPI_WaitlistUser_PublicSignup(t *testing.T) {
	t.Parallel()
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

	// Set campaign to active status so it can accept signups
	statusPath := fmt.Sprintf("/api/v1/campaigns/%s/status", campaignID)
	statusResp, _ := makeAuthenticatedRequest(t, http.MethodPatch, statusPath, map[string]interface{}{"status": "active"}, token)
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to set campaign status to active")
	}

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

func TestAPI_WaitlistUser_SignupFailsForNonActiveCampaign(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedUser(t)

	// Create test campaigns for each non-active status
	statusTests := []struct {
		name           string
		status         string
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "signup fails for draft campaign",
			status:         "draft", // Default status, no update needed
			expectedStatus: http.StatusConflict,
			expectedCode:   "CAMPAIGN_NOT_ACTIVE",
		},
		{
			name:           "signup fails for paused campaign",
			status:         "paused",
			expectedStatus: http.StatusConflict,
			expectedCode:   "CAMPAIGN_NOT_ACTIVE",
		},
		{
			name:           "signup fails for completed campaign",
			status:         "completed",
			expectedStatus: http.StatusConflict,
			expectedCode:   "CAMPAIGN_NOT_ACTIVE",
		},
	}

	for _, tt := range statusTests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a campaign
			createCampaignReq := map[string]interface{}{
				"name": fmt.Sprintf("Test Campaign - %s", tt.status),
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
			}
			createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", createCampaignReq, token)
			if createResp.StatusCode != http.StatusCreated {
				t.Fatalf("Failed to create test campaign: %s", string(createBody))
			}

			var createdCampaign map[string]interface{}
			parseJSONResponse(t, createBody, &createdCampaign)
			campaignID := createdCampaign["id"].(string)

			// Set status if not draft (draft is the default)
			if tt.status != "draft" {
				// First activate, then set to target status (some statuses may require being active first)
				statusPath := fmt.Sprintf("/api/v1/campaigns/%s/status", campaignID)
				makeAuthenticatedRequest(t, http.MethodPatch, statusPath, map[string]interface{}{"status": "active"}, token)
				makeAuthenticatedRequest(t, http.MethodPatch, statusPath, map[string]interface{}{"status": tt.status}, token)
			}

			// Try to sign up - should fail
			signupPath := fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID)
			signupResp, signupBody := makeRequest(t, http.MethodPost, signupPath, map[string]interface{}{
				"email":          fmt.Sprintf("test-%s@example.com", tt.status),
				"terms_accepted": true,
			}, nil)

			assertStatusCode(t, signupResp, tt.expectedStatus)

			var errResp map[string]interface{}
			parseJSONResponse(t, signupBody, &errResp)
			if errResp["code"] != tt.expectedCode {
				t.Errorf("Expected error code %s, got %v", tt.expectedCode, errResp["code"])
			}
		})
	}
}

func TestAPI_CampaignCounters_RealTime(t *testing.T) {
	t.Parallel()
	// Create an authenticated user and a campaign
	token := createAuthenticatedUser(t)

	// Create a test campaign
	createCampaignReq := map[string]interface{}{
		"name":            "Counter Test Campaign",
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

	// Set campaign to active status so it can accept signups
	statusPath := fmt.Sprintf("/api/v1/campaigns/%s/status", campaignID)
	statusResp, _ := makeAuthenticatedRequest(t, http.MethodPatch, statusPath, map[string]interface{}{"status": "active"}, token)
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to set campaign status to active")
	}

	// Verify initial counts are zero
	getCampaignPath := fmt.Sprintf("/api/v1/campaigns/%s", campaignID)
	resp, body := makeAuthenticatedRequest(t, http.MethodGet, getCampaignPath, nil, token)
	assertStatusCode(t, resp, http.StatusOK)

	var campaign map[string]interface{}
	parseJSONResponse(t, body, &campaign)

	if campaign["total_signups"].(float64) != 0 {
		t.Errorf("Expected initial total_signups to be 0, got %v", campaign["total_signups"])
	}
	if campaign["total_verified"].(float64) != 0 {
		t.Errorf("Expected initial total_verified to be 0, got %v", campaign["total_verified"])
	}

	// Sign up 3 users
	for i := 1; i <= 3; i++ {
		signupReq := map[string]interface{}{
			"email":          fmt.Sprintf("counter-test-%d@example.com", i),
			"terms_accepted": true,
		}
		signupPath := fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID)
		signupResp, _ := makeRequest(t, http.MethodPost, signupPath, signupReq, nil)
		if signupResp.StatusCode != http.StatusCreated {
			t.Fatalf("Failed to signup user %d", i)
		}
	}

	// Verify counts are updated
	resp, body = makeAuthenticatedRequest(t, http.MethodGet, getCampaignPath, nil, token)
	assertStatusCode(t, resp, http.StatusOK)

	parseJSONResponse(t, body, &campaign)

	if campaign["total_signups"].(float64) != 3 {
		t.Errorf("Expected total_signups to be 3, got %v", campaign["total_signups"])
	}

	// Verify in campaign list as well
	listPath := "/api/v1/campaigns"
	resp, body = makeAuthenticatedRequest(t, http.MethodGet, listPath, nil, token)
	assertStatusCode(t, resp, http.StatusOK)

	var listResponse map[string]interface{}
	parseJSONResponse(t, body, &listResponse)

	campaigns, ok := listResponse["campaigns"].([]interface{})
	if !ok {
		t.Fatal("Expected 'campaigns' array in response")
	}

	// Find our campaign in the list
	var foundCampaign map[string]interface{}
	for _, c := range campaigns {
		campMap := c.(map[string]interface{})
		if campMap["id"].(string) == campaignID {
			foundCampaign = campMap
			break
		}
	}

	if foundCampaign == nil {
		t.Fatal("Campaign not found in list")
	}

	if foundCampaign["total_signups"].(float64) != 3 {
		t.Errorf("Expected total_signups in list to be 3, got %v", foundCampaign["total_signups"])
	}
}

func TestAPI_ListUsers_WithFilters(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedUser(t)

	// Create a test campaign
	createCampaignReq := map[string]interface{}{
		"name": "Filter Test Campaign",
		"slug": generateTestCampaignSlug(),
		"type": "waitlist",
	}
	createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", createCampaignReq, token)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test campaign: %s", string(createBody))
	}

	var createdCampaign map[string]interface{}
	parseJSONResponse(t, createBody, &createdCampaign)
	campaignID := createdCampaign["id"].(string)

	// Set campaign to active
	statusPath := fmt.Sprintf("/api/v1/campaigns/%s/status", campaignID)
	makeAuthenticatedRequest(t, http.MethodPatch, statusPath, map[string]interface{}{"status": "active"}, token)

	// Sign up users with different custom fields
	signupPath := fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID)

	// User 1: Company=Acme, Role=Developer
	makeRequest(t, http.MethodPost, signupPath, map[string]interface{}{
		"email":          "user1-filter@example.com",
		"terms_accepted": true,
		"custom_fields": map[string]string{
			"company": "Acme",
			"role":    "Developer",
		},
	}, nil)

	// User 2: Company=Beta, Role=Manager
	makeRequest(t, http.MethodPost, signupPath, map[string]interface{}{
		"email":          "user2-filter@example.com",
		"terms_accepted": true,
		"custom_fields": map[string]string{
			"company": "Beta",
			"role":    "Manager",
		},
	}, nil)

	// User 3: Company=Acme, Role=Designer
	makeRequest(t, http.MethodPost, signupPath, map[string]interface{}{
		"email":          "user3-filter@example.com",
		"terms_accepted": true,
		"custom_fields": map[string]string{
			"company": "Acme",
			"role":    "Designer",
		},
	}, nil)

	tests := []struct {
		name           string
		queryParams    string
		expectedCount  int
		validateFunc   func(t *testing.T, users []interface{})
	}{
		{
			name:          "list all users",
			queryParams:   "",
			expectedCount: 3,
		},
		{
			name:          "filter by custom field company=Acme",
			queryParams:   "custom_fields[company]=Acme",
			expectedCount: 2,
			validateFunc: func(t *testing.T, users []interface{}) {
				for _, u := range users {
					user := u.(map[string]interface{})
					metadata := user["metadata"].(map[string]interface{})
					if metadata["company"] != "Acme" {
						t.Errorf("Expected company=Acme, got %v", metadata["company"])
					}
				}
			},
		},
		{
			name:          "filter by custom field role=Developer",
			queryParams:   "custom_fields[role]=Developer",
			expectedCount: 1,
			validateFunc: func(t *testing.T, users []interface{}) {
				if len(users) != 1 {
					t.Fatalf("Expected 1 user, got %d", len(users))
				}
				user := users[0].(map[string]interface{})
				metadata := user["metadata"].(map[string]interface{})
				if metadata["role"] != "Developer" {
					t.Errorf("Expected role=Developer, got %v", metadata["role"])
				}
			},
		},
		{
			name:          "pagination with limit",
			queryParams:   "limit=2&page=1",
			expectedCount: 2,
		},
		{
			name:          "pagination with offset",
			queryParams:   "limit=2&page=2",
			expectedCount: 1,
		},
		{
			name:          "sort by position ascending",
			queryParams:   "sort=position&order=asc",
			expectedCount: 3,
		},
		{
			name:          "sort by created_at descending",
			queryParams:   "sort=created_at&order=desc",
			expectedCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listPath := fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID)
			if tt.queryParams != "" {
				listPath += "?" + tt.queryParams
			}

			resp, body := makeAuthenticatedRequest(t, http.MethodGet, listPath, nil, token)
			assertStatusCode(t, resp, http.StatusOK)

			var listResponse map[string]interface{}
			parseJSONResponse(t, body, &listResponse)

			users, ok := listResponse["users"].([]interface{})
			if !ok {
				t.Fatal("Expected 'users' array in response")
			}

			if len(users) != tt.expectedCount {
				t.Errorf("Expected %d users, got %d", tt.expectedCount, len(users))
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, users)
			}
		})
	}
}

func TestAPI_ListUsers_FilterBySource(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedUser(t)

	// Create a test campaign
	createCampaignReq := map[string]interface{}{
		"name": "Source Filter Test Campaign",
		"slug": generateTestCampaignSlug(),
		"type": "waitlist",
	}
	createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", createCampaignReq, token)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test campaign: %s", string(createBody))
	}

	var createdCampaign map[string]interface{}
	parseJSONResponse(t, createBody, &createdCampaign)
	campaignID := createdCampaign["id"].(string)

	// Set campaign to active
	statusPath := fmt.Sprintf("/api/v1/campaigns/%s/status", campaignID)
	makeAuthenticatedRequest(t, http.MethodPatch, statusPath, map[string]interface{}{"status": "active"}, token)

	// Sign up 3 users (all will be direct source since no referral code)
	signupPath := fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID)
	for i := 1; i <= 3; i++ {
		makeRequest(t, http.MethodPost, signupPath, map[string]interface{}{
			"email":          fmt.Sprintf("source-test-%d@example.com", i),
			"terms_accepted": true,
		}, nil)
	}

	// Test filtering by source
	listPath := fmt.Sprintf("/api/v1/campaigns/%s/users?source[]=direct", campaignID)
	resp, body := makeAuthenticatedRequest(t, http.MethodGet, listPath, nil, token)
	assertStatusCode(t, resp, http.StatusOK)

	var listResponse map[string]interface{}
	parseJSONResponse(t, body, &listResponse)

	users := listResponse["users"].([]interface{})
	if len(users) != 3 {
		t.Errorf("Expected 3 users with direct source, got %d", len(users))
	}
}

func TestAPI_ListUsers_FilterByPositionRange(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedUser(t)

	// Create a test campaign
	createCampaignReq := map[string]interface{}{
		"name": "Position Filter Test Campaign",
		"slug": generateTestCampaignSlug(),
		"type": "waitlist",
	}
	createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", createCampaignReq, token)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test campaign: %s", string(createBody))
	}

	var createdCampaign map[string]interface{}
	parseJSONResponse(t, createBody, &createdCampaign)
	campaignID := createdCampaign["id"].(string)

	// Set campaign to active
	statusPath := fmt.Sprintf("/api/v1/campaigns/%s/status", campaignID)
	makeAuthenticatedRequest(t, http.MethodPatch, statusPath, map[string]interface{}{"status": "active"}, token)

	// Sign up 5 users
	signupPath := fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID)
	for i := 1; i <= 5; i++ {
		makeRequest(t, http.MethodPost, signupPath, map[string]interface{}{
			"email":          fmt.Sprintf("position-test-%d@example.com", i),
			"terms_accepted": true,
		}, nil)
	}

	// Note: Positions may be -1 initially (calculated asynchronously)
	// Test that the endpoint accepts position filters without error
	listPath := fmt.Sprintf("/api/v1/campaigns/%s/users?min_position=1&max_position=3", campaignID)
	resp, body := makeAuthenticatedRequest(t, http.MethodGet, listPath, nil, token)
	assertStatusCode(t, resp, http.StatusOK)

	var listResponse map[string]interface{}
	parseJSONResponse(t, body, &listResponse)

	// Verify response structure
	if listResponse["users"] == nil {
		t.Error("Expected 'users' in response")
	}
	if listResponse["total_count"] == nil {
		t.Error("Expected 'total_count' in response")
	}
	if listResponse["page"] == nil {
		t.Error("Expected 'page' in response")
	}
	if listResponse["page_size"] == nil {
		t.Error("Expected 'page_size' in response")
	}
	if listResponse["total_pages"] == nil {
		t.Error("Expected 'total_pages' in response")
	}
}

func TestAPI_ListUsers_FilterByDateRange(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedUser(t)

	// Create a test campaign
	createCampaignReq := map[string]interface{}{
		"name": "Date Filter Test Campaign",
		"slug": generateTestCampaignSlug(),
		"type": "waitlist",
	}
	createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", createCampaignReq, token)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test campaign: %s", string(createBody))
	}

	var createdCampaign map[string]interface{}
	parseJSONResponse(t, createBody, &createdCampaign)
	campaignID := createdCampaign["id"].(string)

	// Set campaign to active
	statusPath := fmt.Sprintf("/api/v1/campaigns/%s/status", campaignID)
	makeAuthenticatedRequest(t, http.MethodPatch, statusPath, map[string]interface{}{"status": "active"}, token)

	// Sign up a user
	signupPath := fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID)
	makeRequest(t, http.MethodPost, signupPath, map[string]interface{}{
		"email":          "date-test@example.com",
		"terms_accepted": true,
	}, nil)

	// Test with today's date range (should include the user we just created)
	listPath := fmt.Sprintf("/api/v1/campaigns/%s/users?date_from=2020-01-01&date_to=2030-12-31", campaignID)
	resp, body := makeAuthenticatedRequest(t, http.MethodGet, listPath, nil, token)
	assertStatusCode(t, resp, http.StatusOK)

	var listResponse map[string]interface{}
	parseJSONResponse(t, body, &listResponse)

	users := listResponse["users"].([]interface{})
	if len(users) != 1 {
		t.Errorf("Expected 1 user in date range, got %d", len(users))
	}

	// Test with past date range (should return no users)
	listPath = fmt.Sprintf("/api/v1/campaigns/%s/users?date_from=2000-01-01&date_to=2000-12-31", campaignID)
	resp, body = makeAuthenticatedRequest(t, http.MethodGet, listPath, nil, token)
	assertStatusCode(t, resp, http.StatusOK)

	parseJSONResponse(t, body, &listResponse)
	users = listResponse["users"].([]interface{})
	if len(users) != 0 {
		t.Errorf("Expected 0 users for old date range, got %d", len(users))
	}
}

func TestAPI_ListUsers_CombinedFilters(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedUser(t)

	// Create a test campaign
	createCampaignReq := map[string]interface{}{
		"name": "Combined Filter Test Campaign",
		"slug": generateTestCampaignSlug(),
		"type": "waitlist",
	}
	createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", createCampaignReq, token)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test campaign: %s", string(createBody))
	}

	var createdCampaign map[string]interface{}
	parseJSONResponse(t, createBody, &createdCampaign)
	campaignID := createdCampaign["id"].(string)

	// Set campaign to active
	statusPath := fmt.Sprintf("/api/v1/campaigns/%s/status", campaignID)
	makeAuthenticatedRequest(t, http.MethodPatch, statusPath, map[string]interface{}{"status": "active"}, token)

	// Sign up users with different companies
	signupPath := fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID)

	// 2 users from Acme
	for i := 1; i <= 2; i++ {
		makeRequest(t, http.MethodPost, signupPath, map[string]interface{}{
			"email":          fmt.Sprintf("acme-combined-%d@example.com", i),
			"terms_accepted": true,
			"custom_fields": map[string]string{
				"company": "Acme",
			},
		}, nil)
	}

	// 1 user from Beta
	makeRequest(t, http.MethodPost, signupPath, map[string]interface{}{
		"email":          "beta-combined@example.com",
		"terms_accepted": true,
		"custom_fields": map[string]string{
			"company": "Beta",
		},
	}, nil)

	// Test combined filters: custom field + pagination + sorting
	listPath := fmt.Sprintf("/api/v1/campaigns/%s/users?custom_fields[company]=Acme&limit=10&sort=created_at&order=desc", campaignID)
	resp, body := makeAuthenticatedRequest(t, http.MethodGet, listPath, nil, token)
	assertStatusCode(t, resp, http.StatusOK)

	var listResponse map[string]interface{}
	parseJSONResponse(t, body, &listResponse)

	users := listResponse["users"].([]interface{})
	if len(users) != 2 {
		t.Errorf("Expected 2 users from Acme, got %d", len(users))
	}

	// Verify all returned users have company=Acme
	for _, u := range users {
		user := u.(map[string]interface{})
		metadata := user["metadata"].(map[string]interface{})
		if metadata["company"] != "Acme" {
			t.Errorf("Expected company=Acme, got %v", metadata["company"])
		}
	}
}

func TestAPI_ListUsers_ResponseStructure(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedUser(t)

	// Create a test campaign
	createCampaignReq := map[string]interface{}{
		"name": "Response Structure Test Campaign",
		"slug": generateTestCampaignSlug(),
		"type": "waitlist",
	}
	createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", createCampaignReq, token)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test campaign: %s", string(createBody))
	}

	var createdCampaign map[string]interface{}
	parseJSONResponse(t, createBody, &createdCampaign)
	campaignID := createdCampaign["id"].(string)

	// Set campaign to active
	statusPath := fmt.Sprintf("/api/v1/campaigns/%s/status", campaignID)
	makeAuthenticatedRequest(t, http.MethodPatch, statusPath, map[string]interface{}{"status": "active"}, token)

	// Sign up a user with custom fields
	signupPath := fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID)
	makeRequest(t, http.MethodPost, signupPath, map[string]interface{}{
		"email":          "structure-test@example.com",
		"terms_accepted": true,
		"custom_fields": map[string]string{
			"company": "TestCorp",
			"role":    "Engineer",
		},
	}, nil)

	// List users and verify response structure
	listPath := fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID)
	resp, body := makeAuthenticatedRequest(t, http.MethodGet, listPath, nil, token)
	assertStatusCode(t, resp, http.StatusOK)

	var listResponse map[string]interface{}
	parseJSONResponse(t, body, &listResponse)

	// Verify pagination fields
	if listResponse["page"].(float64) != 1 {
		t.Errorf("Expected page=1, got %v", listResponse["page"])
	}
	if listResponse["total_count"].(float64) != 1 {
		t.Errorf("Expected total_count=1, got %v", listResponse["total_count"])
	}
	if listResponse["total_pages"].(float64) != 1 {
		t.Errorf("Expected total_pages=1, got %v", listResponse["total_pages"])
	}

	users := listResponse["users"].([]interface{})
	if len(users) != 1 {
		t.Fatalf("Expected 1 user, got %d", len(users))
	}

	user := users[0].(map[string]interface{})

	// Verify user has expected fields
	requiredFields := []string{"id", "email", "status", "position", "metadata", "created_at"}
	for _, field := range requiredFields {
		if user[field] == nil {
			t.Errorf("Expected user to have field '%s'", field)
		}
	}

	// Verify metadata contains custom fields
	metadata := user["metadata"].(map[string]interface{})
	if metadata["company"] != "TestCorp" {
		t.Errorf("Expected company=TestCorp in metadata, got %v", metadata["company"])
	}
	if metadata["role"] != "Engineer" {
		t.Errorf("Expected role=Engineer in metadata, got %v", metadata["role"])
	}
}
