//go:build integration
// +build integration

package tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAPI_Analytics_SignupsOverTime(t *testing.T) {
	// Create authenticated user and get token
	token := createAuthenticatedTestUser(t)

	// Create a test campaign
	campaignSlug := generateTestCampaignSlug()
	createCampaignReq := map[string]interface{}{
		"name":            "Analytics Test Campaign",
		"slug":            campaignSlug,
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

	// Add some test signups to the campaign
	for i := 0; i < 5; i++ {
		signupReq := map[string]interface{}{
			"email":          fmt.Sprintf("analytics-test-%d-%s@example.com", i, uuid.New().String()[:8]),
			"terms_accepted": true,
		}
		signupResp, signupBody := makeRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID), signupReq, nil)
		if signupResp.StatusCode != http.StatusCreated {
			t.Fatalf("Failed to create test signup: %s", string(signupBody))
		}
	}

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name:           "get signups over time with default params",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				// Check required fields
				if response["data"] == nil {
					t.Error("Expected 'data' field in response")
				}
				if response["total"] == nil {
					t.Error("Expected 'total' field in response")
				}
				if response["period"] == nil {
					t.Error("Expected 'period' field in response")
				}

				// Default period should be "day"
				if response["period"] != "day" {
					t.Errorf("Expected period 'day', got %v", response["period"])
				}

				// Total should be at least 5 (we created 5 signups)
				total := int(response["total"].(float64))
				if total < 5 {
					t.Errorf("Expected total >= 5, got %d", total)
				}

				// Data should be an array
				data, ok := response["data"].([]interface{})
				if !ok {
					t.Fatal("Expected 'data' to be an array")
				}

				// Each data point should have date and count
				if len(data) > 0 {
					firstPoint := data[0].(map[string]interface{})
					if firstPoint["date"] == nil {
						t.Error("Expected 'date' in data point")
					}
					if firstPoint["count"] == nil {
						t.Error("Expected 'count' in data point")
					}
				}
			},
		},
		{
			name:           "get signups over time with hourly period",
			queryParams:    "?period=hour",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				if response["period"] != "hour" {
					t.Errorf("Expected period 'hour', got %v", response["period"])
				}
			},
		},
		{
			name:           "get signups over time with weekly period",
			queryParams:    "?period=week",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				if response["period"] != "week" {
					t.Errorf("Expected period 'week', got %v", response["period"])
				}
			},
		},
		{
			name:           "get signups over time with monthly period",
			queryParams:    "?period=month",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				if response["period"] != "month" {
					t.Errorf("Expected period 'month', got %v", response["period"])
				}

				// Should have current month data
				total := int(response["total"].(float64))
				if total < 5 {
					t.Errorf("Expected total >= 5, got %d", total)
				}
			},
		},
		{
			name:           "get signups over time with date range",
			queryParams:    fmt.Sprintf("?from=%s&to=%s", time.Now().AddDate(0, 0, -7).Format(time.RFC3339), time.Now().Format(time.RFC3339)),
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				// Should have data within the date range
				if response["data"] == nil {
					t.Error("Expected 'data' field in response")
				}
			},
		},
		{
			name:           "get signups over time with invalid period defaults to day",
			queryParams:    "?period=invalid",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				// Invalid period should default to "day"
				if response["period"] != "day" {
					t.Errorf("Expected period 'day' for invalid input, got %v", response["period"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fmt.Sprintf("/api/v1/campaigns/%s/analytics/signups-over-time%s", campaignID, tt.queryParams)
			resp, body := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.StatusCode, string(body))
				return
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

func TestAPI_Analytics_SignupsOverTime_Unauthorized(t *testing.T) {
	// Try to access without authentication
	path := fmt.Sprintf("/api/v1/campaigns/%s/analytics/signups-over-time", uuid.New().String())
	resp, _ := makeRequest(t, http.MethodGet, path, nil, nil)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status %d for unauthenticated request, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestAPI_Analytics_SignupsOverTime_InvalidCampaign(t *testing.T) {
	token := createAuthenticatedTestUser(t)

	// Try to access with non-existent campaign ID
	path := fmt.Sprintf("/api/v1/campaigns/%s/analytics/signups-over-time", uuid.New().String())
	resp, _ := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)

	// Should return 404 Not Found
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status %d for non-existent campaign, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestAPI_Analytics_SignupsOverTime_InvalidDateFormat(t *testing.T) {
	token := createAuthenticatedTestUser(t)

	// Create a campaign first
	campaignSlug := generateTestCampaignSlug()
	createCampaignReq := map[string]interface{}{
		"name":            "Date Format Test Campaign",
		"slug":            campaignSlug,
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

	// Try with invalid date format
	path := fmt.Sprintf("/api/v1/campaigns/%s/analytics/signups-over-time?from=invalid-date", campaignID)
	resp, _ := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)

	// Should return 400 Bad Request
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for invalid date format, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestAPI_Analytics_SignupsBySource(t *testing.T) {
	// Create authenticated user and get token
	token := createAuthenticatedTestUser(t)

	// Create a test campaign
	campaignSlug := generateTestCampaignSlug()
	createCampaignReq := map[string]interface{}{
		"name":            "Signups By Source Test Campaign",
		"slug":            campaignSlug,
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

	// Add signups with different UTM sources
	for i := 0; i < 3; i++ {
		signupReq := map[string]interface{}{
			"email":          fmt.Sprintf("source-google-%d-%s@example.com", i, uuid.New().String()[:8]),
			"terms_accepted": true,
			"utm_source":     "google",
		}
		signupResp, signupBody := makeRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID), signupReq, nil)
		if signupResp.StatusCode != http.StatusCreated {
			t.Fatalf("Failed to create test signup: %s", string(signupBody))
		}
	}

	for i := 0; i < 2; i++ {
		signupReq := map[string]interface{}{
			"email":          fmt.Sprintf("source-facebook-%d-%s@example.com", i, uuid.New().String()[:8]),
			"terms_accepted": true,
			"utm_source":     "facebook",
		}
		signupResp, signupBody := makeRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID), signupReq, nil)
		if signupResp.StatusCode != http.StatusCreated {
			t.Fatalf("Failed to create test signup: %s", string(signupBody))
		}
	}

	// Add one signup without UTM source (null)
	signupReq := map[string]interface{}{
		"email":          fmt.Sprintf("source-null-%s@example.com", uuid.New().String()[:8]),
		"terms_accepted": true,
	}
	signupResp, signupBody := makeRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID), signupReq, nil)
	if signupResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test signup: %s", string(signupBody))
	}

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name:           "get signups by source with default params",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				// Check required fields
				if response["data"] == nil {
					t.Error("Expected 'data' field in response")
				}
				if response["sources"] == nil {
					t.Error("Expected 'sources' field in response")
				}
				if response["total"] == nil {
					t.Error("Expected 'total' field in response")
				}
				if response["period"] == nil {
					t.Error("Expected 'period' field in response")
				}

				// Default period should be "day"
				if response["period"] != "day" {
					t.Errorf("Expected period 'day', got %v", response["period"])
				}

				// Total should be at least 6 (we created 6 signups)
				total := int(response["total"].(float64))
				if total < 6 {
					t.Errorf("Expected total >= 6, got %d", total)
				}

				// Sources should include google, facebook
				sources, ok := response["sources"].([]interface{})
				if !ok {
					t.Fatal("Expected 'sources' to be an array")
				}

				sourceSet := make(map[string]bool)
				for _, s := range sources {
					if s != nil {
						sourceSet[s.(string)] = true
					} else {
						sourceSet[""] = true
					}
				}

				if !sourceSet["google"] {
					t.Error("Expected 'google' in sources")
				}
				if !sourceSet["facebook"] {
					t.Error("Expected 'facebook' in sources")
				}

				// Data should be an array
				data, ok := response["data"].([]interface{})
				if !ok {
					t.Fatal("Expected 'data' to be an array")
				}

				// Each data point should have date, utm_source, and count
				if len(data) > 0 {
					firstPoint := data[0].(map[string]interface{})
					if firstPoint["date"] == nil {
						t.Error("Expected 'date' in data point")
					}
					// utm_source can be null, so just check count
					if firstPoint["count"] == nil {
						t.Error("Expected 'count' in data point")
					}
				}
			},
		},
		{
			name:           "get signups by source with hourly period",
			queryParams:    "?period=hour",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				if response["period"] != "hour" {
					t.Errorf("Expected period 'hour', got %v", response["period"])
				}
			},
		},
		{
			name:           "get signups by source with weekly period",
			queryParams:    "?period=week",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				if response["period"] != "week" {
					t.Errorf("Expected period 'week', got %v", response["period"])
				}
			},
		},
		{
			name:           "get signups by source with monthly period",
			queryParams:    "?period=month",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				if response["period"] != "month" {
					t.Errorf("Expected period 'month', got %v", response["period"])
				}
			},
		},
		{
			name:           "get signups by source with date range",
			queryParams:    fmt.Sprintf("?from=%s&to=%s", time.Now().AddDate(0, 0, -7).Format(time.RFC3339), time.Now().Format(time.RFC3339)),
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				if response["data"] == nil {
					t.Error("Expected 'data' field in response")
				}
			},
		},
		{
			name:           "get signups by source with invalid period defaults to day",
			queryParams:    "?period=invalid",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				// Invalid period should default to "day"
				if response["period"] != "day" {
					t.Errorf("Expected period 'day' for invalid input, got %v", response["period"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fmt.Sprintf("/api/v1/campaigns/%s/analytics/signups-by-source%s", campaignID, tt.queryParams)
			resp, body := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.StatusCode, string(body))
				return
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

func TestAPI_Analytics_SignupsBySource_Unauthorized(t *testing.T) {
	// Try to access without authentication
	path := fmt.Sprintf("/api/v1/campaigns/%s/analytics/signups-by-source", uuid.New().String())
	resp, _ := makeRequest(t, http.MethodGet, path, nil, nil)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status %d for unauthenticated request, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestAPI_Analytics_SignupsBySource_InvalidCampaign(t *testing.T) {
	token := createAuthenticatedTestUser(t)

	// Try to access with non-existent campaign ID
	path := fmt.Sprintf("/api/v1/campaigns/%s/analytics/signups-by-source", uuid.New().String())
	resp, _ := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)

	// Should return 404 Not Found
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status %d for non-existent campaign, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestAPI_Analytics_SignupsBySource_InvalidDateFormat(t *testing.T) {
	token := createAuthenticatedTestUser(t)

	// Create a campaign first
	campaignSlug := generateTestCampaignSlug()
	createCampaignReq := map[string]interface{}{
		"name":            "Date Format Test Campaign 2",
		"slug":            campaignSlug,
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

	// Try with invalid date format
	path := fmt.Sprintf("/api/v1/campaigns/%s/analytics/signups-by-source?from=invalid-date", campaignID)
	resp, _ := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)

	// Should return 400 Bad Request
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for invalid date format, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestAPI_Analytics_SignupsBySource_GapFilling(t *testing.T) {
	token := createAuthenticatedTestUser(t)

	// Create a campaign
	campaignSlug := generateTestCampaignSlug()
	createCampaignReq := map[string]interface{}{
		"name":            "Gap Filling By Source Test Campaign",
		"slug":            campaignSlug,
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

	// Add one signup with a source
	signupReq := map[string]interface{}{
		"email":          fmt.Sprintf("gap-source-test-%s@example.com", uuid.New().String()[:8]),
		"terms_accepted": true,
		"utm_source":     "google",
	}
	signupResp, signupBody := makeRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID), signupReq, nil)
	if signupResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test signup: %s", string(signupBody))
	}

	// Small delay to ensure data is committed
	time.Sleep(100 * time.Millisecond)

	// Request 7 days of data using UTC to ensure consistent day boundaries
	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	from := todayStart.AddDate(0, 0, -6).Format(time.RFC3339)
	to := todayStart.Add(24*time.Hour - time.Second).Format(time.RFC3339)
	path := fmt.Sprintf("/api/v1/campaigns/%s/analytics/signups-by-source?period=day&from=%s&to=%s", campaignID, from, to)

	resp, body := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	var response map[string]interface{}
	parseJSONResponse(t, body, &response)

	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatal("Expected 'data' to be an array")
	}

	// With gap filling, we should have 7 days of data for each source
	// Since we only have 1 source, we should have 7 data points
	sources, ok := response["sources"].([]interface{})
	if !ok {
		t.Fatal("Expected 'sources' to be an array")
	}

	expectedDataPoints := 7 * len(sources)
	if len(data) != expectedDataPoints {
		t.Errorf("Expected %d data points (7 days * %d sources), got %d", expectedDataPoints, len(sources), len(data))
	}

	// Verify we have data points with gap filling
	// With 1 source and 7 days, we should have 7 data points total
	// Most days should have count 0 (gap filled), but at least verify we have multiple data points
	if len(sources) == 0 {
		t.Error("Expected at least one source")
	}

	// Verify the total is correct (should be 1 since we added 1 signup)
	total := int(response["total"].(float64))
	if total != 1 {
		t.Errorf("Expected total of 1, got %d", total)
	}

	// Count zero-count data points
	zeroCount := 0
	for _, point := range data {
		p := point.(map[string]interface{})
		count := int(p["count"].(float64))
		if count == 0 {
			zeroCount++
		}
	}

	// With 7 days and 1 signup, at least some should be zero (gap filled)
	// Allow for edge cases where timezone might affect day boundaries
	if len(data) > 1 && zeroCount < 1 {
		t.Errorf("Expected at least 1 zero-count data point (gap filling), got %d out of %d data points", zeroCount, len(data))
	}
}

func TestAPI_Analytics_SignupsOverTime_GapFilling(t *testing.T) {
	token := createAuthenticatedTestUser(t)

	// Create a campaign
	campaignSlug := generateTestCampaignSlug()
	createCampaignReq := map[string]interface{}{
		"name":            "Gap Filling Test Campaign",
		"slug":            campaignSlug,
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

	// Add one signup
	signupReq := map[string]interface{}{
		"email":          fmt.Sprintf("gap-test-%s@example.com", uuid.New().String()[:8]),
		"terms_accepted": true,
	}
	signupResp, signupBody := makeRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID), signupReq, nil)
	if signupResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test signup: %s", string(signupBody))
	}

	// Request 7 days of data
	from := time.Now().AddDate(0, 0, -6).Format(time.RFC3339)
	to := time.Now().Format(time.RFC3339)
	path := fmt.Sprintf("/api/v1/campaigns/%s/analytics/signups-over-time?period=day&from=%s&to=%s", campaignID, from, to)

	resp, body := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	var response map[string]interface{}
	parseJSONResponse(t, body, &response)

	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatal("Expected 'data' to be an array")
	}

	// Should have 7 days of data (with gap filling)
	if len(data) != 7 {
		t.Errorf("Expected 7 data points (gap filling), got %d", len(data))
	}

	// Most days should have count 0 (gap filled)
	zeroCount := 0
	for _, point := range data {
		p := point.(map[string]interface{})
		if int(p["count"].(float64)) == 0 {
			zeroCount++
		}
	}

	// At least some days should be zero (gap filled)
	if zeroCount < 5 {
		t.Errorf("Expected at least 5 zero-count days (gap filling), got %d", zeroCount)
	}
}
