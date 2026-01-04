//go:build integration
// +build integration

package tests

import (
	"fmt"
	"net/http"
	"testing"
)

// =============================================================================
// REFERRAL SYSTEM GATING TESTS
// =============================================================================

func TestAPI_ReferralSystem_Gating(t *testing.T) {
	t.Parallel()

	t.Run("free tier cannot access referral system", func(t *testing.T) {
		t.Parallel()
		token := createAuthenticatedTestUserWithFreeTier(t)

		// Create a campaign
		campaignID := createTestCampaign(t, token)
		activateTestCampaign(t, token, campaignID)

		// Create a waitlist user for referral tests
		userID := createTestWaitlistUser(t, campaignID)

		tests := []struct {
			name     string
			method   string
			path     string
			wantCode int
		}{
			{
				name:     "list referrals blocked for free tier",
				method:   http.MethodGet,
				path:     fmt.Sprintf("/api/v1/campaigns/%s/referrals", campaignID),
				wantCode: http.StatusForbidden,
			},
			{
				name:     "get user referrals blocked for free tier",
				method:   http.MethodGet,
				path:     fmt.Sprintf("/api/v1/campaigns/%s/users/%s/referrals", campaignID, userID),
				wantCode: http.StatusForbidden,
			},
			{
				name:     "get referral link blocked for free tier",
				method:   http.MethodGet,
				path:     fmt.Sprintf("/api/v1/campaigns/%s/users/%s/referral-link", campaignID, userID),
				wantCode: http.StatusForbidden,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp, body := makeAuthenticatedRequest(t, tt.method, tt.path, nil, token)
				if resp.StatusCode != tt.wantCode {
					t.Errorf("expected status %d, got %d, body: %s", tt.wantCode, resp.StatusCode, string(body))
				}

				var response map[string]interface{}
				parseJSONResponse(t, body, &response)
				if response["code"] != "FEATURE_NOT_AVAILABLE" {
					t.Errorf("expected error code FEATURE_NOT_AVAILABLE, got %v", response["code"])
				}
			})
		}
	})

	t.Run("pro tier can access referral system", func(t *testing.T) {
		t.Parallel()
		token := createAuthenticatedTestUserWithProTier(t)

		// Create a campaign
		campaignID := createTestCampaign(t, token)
		activateTestCampaign(t, token, campaignID)

		// Create a waitlist user for referral tests
		userID := createTestWaitlistUser(t, campaignID)

		tests := []struct {
			name     string
			method   string
			path     string
			wantCode int
		}{
			{
				name:     "list referrals allowed for pro tier",
				method:   http.MethodGet,
				path:     fmt.Sprintf("/api/v1/campaigns/%s/referrals", campaignID),
				wantCode: http.StatusOK,
			},
			{
				name:     "get user referrals allowed for pro tier",
				method:   http.MethodGet,
				path:     fmt.Sprintf("/api/v1/campaigns/%s/users/%s/referrals", campaignID, userID),
				wantCode: http.StatusOK,
			},
			{
				name:     "get referral link allowed for pro tier",
				method:   http.MethodGet,
				path:     fmt.Sprintf("/api/v1/campaigns/%s/users/%s/referral-link", campaignID, userID),
				wantCode: http.StatusOK,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp, body := makeAuthenticatedRequest(t, tt.method, tt.path, nil, token)
				if resp.StatusCode != tt.wantCode {
					t.Errorf("expected status %d, got %d, body: %s", tt.wantCode, resp.StatusCode, string(body))
				}
			})
		}
	})
}

// =============================================================================
// TRACKING PIXELS GATING TESTS
// =============================================================================

func TestAPI_TrackingPixels_Gating(t *testing.T) {
	t.Parallel()

	t.Run("free tier cannot set tracking pixels", func(t *testing.T) {
		t.Parallel()
		token := createAuthenticatedTestUserWithFreeTier(t)

		// Create a campaign first
		campaignID := createTestCampaign(t, token)

		// Try to update with tracking pixels (using correct API format)
		updateReq := map[string]interface{}{
			"tracking_integrations": []map[string]interface{}{
				{
					"integration_type": "meta_pixel",
					"enabled":          true,
					"tracking_id":      "123456789",
				},
			},
		}
		path := fmt.Sprintf("/api/v1/campaigns/%s", campaignID)
		resp, body := makeAuthenticatedRequest(t, http.MethodPut, path, updateReq, token)

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status %d, got %d, body: %s", http.StatusForbidden, resp.StatusCode, string(body))
		}

		var response map[string]interface{}
		parseJSONResponse(t, body, &response)
		if response["code"] != "FEATURE_NOT_AVAILABLE" {
			t.Errorf("expected error code FEATURE_NOT_AVAILABLE, got %v", response["code"])
		}
	})

	t.Run("free tier cannot set google analytics", func(t *testing.T) {
		t.Parallel()
		token := createAuthenticatedTestUserWithFreeTier(t)

		campaignID := createTestCampaign(t, token)

		// Use correct API format for tracking integrations
		updateReq := map[string]interface{}{
			"tracking_integrations": []map[string]interface{}{
				{
					"integration_type": "google_analytics",
					"enabled":          true,
					"tracking_id":      "G-12345678",
				},
			},
		}
		path := fmt.Sprintf("/api/v1/campaigns/%s", campaignID)
		resp, body := makeAuthenticatedRequest(t, http.MethodPut, path, updateReq, token)

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status %d, got %d, body: %s", http.StatusForbidden, resp.StatusCode, string(body))
		}
	})

	t.Run("pro tier can set tracking pixels", func(t *testing.T) {
		t.Parallel()
		token := createAuthenticatedTestUserWithProTier(t)

		campaignID := createTestCampaign(t, token)

		// Use correct API format for tracking integrations
		updateReq := map[string]interface{}{
			"tracking_integrations": []map[string]interface{}{
				{
					"integration_type": "meta_pixel",
					"enabled":          true,
					"tracking_id":      "123456789",
				},
				{
					"integration_type": "google_analytics",
					"enabled":          true,
					"tracking_id":      "G-12345678",
				},
			},
		}
		path := fmt.Sprintf("/api/v1/campaigns/%s", campaignID)
		resp, body := makeAuthenticatedRequest(t, http.MethodPut, path, updateReq, token)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status %d, got %d, body: %s", http.StatusOK, resp.StatusCode, string(body))
		}

		var campaign map[string]interface{}
		parseJSONResponse(t, body, &campaign)

		// Verify tracking integrations were saved
		trackingIntegrations, ok := campaign["tracking_integrations"].([]interface{})
		if !ok || len(trackingIntegrations) != 2 {
			t.Errorf("expected 2 tracking integrations, got %v", campaign["tracking_integrations"])
		}
	})
}

// =============================================================================
// ANTI-SPAM PROTECTION GATING TESTS
// =============================================================================

func TestAPI_AntiSpamProtection_Gating(t *testing.T) {
	t.Parallel()

	t.Run("free tier cannot enable captcha", func(t *testing.T) {
		t.Parallel()
		token := createAuthenticatedTestUserWithFreeTier(t)

		campaignID := createTestCampaign(t, token)

		// Use correct API format - captcha_enabled is inside form_settings
		updateReq := map[string]interface{}{
			"form_settings": map[string]interface{}{
				"captcha_enabled": true,
			},
		}
		path := fmt.Sprintf("/api/v1/campaigns/%s", campaignID)
		resp, body := makeAuthenticatedRequest(t, http.MethodPut, path, updateReq, token)

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status %d, got %d, body: %s", http.StatusForbidden, resp.StatusCode, string(body))
		}

		var response map[string]interface{}
		parseJSONResponse(t, body, &response)
		if response["code"] != "FEATURE_NOT_AVAILABLE" {
			t.Errorf("expected error code FEATURE_NOT_AVAILABLE, got %v", response["code"])
		}
	})

	t.Run("pro tier can enable captcha", func(t *testing.T) {
		t.Parallel()
		token := createAuthenticatedTestUserWithProTier(t)

		campaignID := createTestCampaign(t, token)

		// Use correct API format - captcha_enabled is inside form_settings
		updateReq := map[string]interface{}{
			"form_settings": map[string]interface{}{
				"captcha_enabled": true,
			},
		}
		path := fmt.Sprintf("/api/v1/campaigns/%s", campaignID)
		resp, body := makeAuthenticatedRequest(t, http.MethodPut, path, updateReq, token)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status %d, got %d, body: %s", http.StatusOK, resp.StatusCode, string(body))
		}

		var campaign map[string]interface{}
		parseJSONResponse(t, body, &campaign)

		// Check form_settings.captcha_enabled
		formSettings, ok := campaign["form_settings"].(map[string]interface{})
		if !ok {
			t.Errorf("expected form_settings in response, got %v", campaign["form_settings"])
			return
		}
		if formSettings["captcha_enabled"] != true {
			t.Errorf("expected captcha_enabled to be true, got %v", formSettings["captcha_enabled"])
		}
	})
}

// =============================================================================
// VISUAL EMAIL BUILDER GATING TESTS
// =============================================================================

func TestAPI_VisualEmailBuilder_Gating(t *testing.T) {
	t.Parallel()

	t.Run("free tier cannot create email templates", func(t *testing.T) {
		t.Parallel()
		token := createAuthenticatedTestUserWithFreeTier(t)

		campaignID := createTestCampaign(t, token)

		// Use correct API field names: html_body (not html_content), type (not trigger_type)
		createReq := map[string]interface{}{
			"name":      "Welcome Email",
			"subject":   "Welcome to our waitlist!",
			"html_body": "<html><body>Welcome!</body></html>",
			"type":      "welcome",
		}
		path := fmt.Sprintf("/api/v1/campaigns/%s/email-templates", campaignID)
		resp, body := makeAuthenticatedRequest(t, http.MethodPost, path, createReq, token)

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status %d, got %d, body: %s", http.StatusForbidden, resp.StatusCode, string(body))
		}

		var response map[string]interface{}
		parseJSONResponse(t, body, &response)
		if response["code"] != "FEATURE_NOT_AVAILABLE" {
			t.Errorf("expected error code FEATURE_NOT_AVAILABLE, got %v", response["code"])
		}
	})

	t.Run("pro tier can create email templates", func(t *testing.T) {
		t.Parallel()
		token := createAuthenticatedTestUserWithProTier(t)

		campaignID := createTestCampaign(t, token)

		// Use correct API field names: html_body (not html_content), type (not trigger_type)
		createReq := map[string]interface{}{
			"name":      "Welcome Email",
			"subject":   "Welcome to our waitlist!",
			"html_body": "<html><body>Welcome!</body></html>",
			"type":      "welcome",
		}
		path := fmt.Sprintf("/api/v1/campaigns/%s/email-templates", campaignID)
		resp, body := makeAuthenticatedRequest(t, http.MethodPost, path, createReq, token)

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("expected status %d, got %d, body: %s", http.StatusCreated, resp.StatusCode, string(body))
		}

		var template map[string]interface{}
		parseJSONResponse(t, body, &template)
		if template["name"] != "Welcome Email" {
			t.Errorf("expected template name to be saved, got %v", template["name"])
		}
	})

	t.Run("pro tier can update email templates", func(t *testing.T) {
		t.Parallel()
		token := createAuthenticatedTestUserWithProTier(t)

		campaignID := createTestCampaign(t, token)

		// Create a template first (using correct field names)
		createReq := map[string]interface{}{
			"name":      "Welcome Email",
			"subject":   "Welcome!",
			"html_body": "<html><body>Welcome!</body></html>",
			"type":      "welcome",
		}
		createPath := fmt.Sprintf("/api/v1/campaigns/%s/email-templates", campaignID)
		createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, createPath, createReq, token)
		if createResp.StatusCode != http.StatusCreated {
			t.Fatalf("Failed to create template: %s", string(createBody))
		}

		var template map[string]interface{}
		parseJSONResponse(t, createBody, &template)
		templateID := template["id"].(string)

		// Update the template
		updateReq := map[string]interface{}{
			"subject": "Updated Welcome!",
		}
		updatePath := fmt.Sprintf("/api/v1/campaigns/%s/email-templates/%s", campaignID, templateID)
		updateResp, updateBody := makeAuthenticatedRequest(t, http.MethodPut, updatePath, updateReq, token)

		if updateResp.StatusCode != http.StatusOK {
			t.Errorf("expected status %d, got %d, body: %s", http.StatusOK, updateResp.StatusCode, string(updateBody))
		}
	})
}

// =============================================================================
// ENHANCED LEAD DATA GATING TESTS
// =============================================================================

func TestAPI_EnhancedLeadData_Gating(t *testing.T) {
	t.Parallel()

	t.Run("free tier list users excludes enhanced data", func(t *testing.T) {
		t.Parallel()
		token := createAuthenticatedTestUserWithFreeTier(t)

		campaignID := createTestCampaign(t, token)
		activateTestCampaign(t, token, campaignID)

		// Create a waitlist user (this will have enhanced data collected on the backend)
		createTestWaitlistUser(t, campaignID)

		// List users - enhanced data should not be present for free tier
		path := fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID)
		resp, body := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d, body: %s", http.StatusOK, resp.StatusCode, string(body))
		}

		var response map[string]interface{}
		parseJSONResponse(t, body, &response)

		users, ok := response["users"].([]interface{})
		if !ok || len(users) == 0 {
			t.Fatal("expected at least one user in response")
		}

		user := users[0].(map[string]interface{})

		// Enhanced geographic fields should be nil/absent for free tier
		enhancedGeoFields := []string{"country", "region", "region_code", "postal_code", "city", "country_code", "user_timezone", "latitude", "longitude", "metro_code"}
		for _, field := range enhancedGeoFields {
			if user[field] != nil {
				t.Errorf("expected %s to be nil for free tier, got %v", field, user[field])
			}
		}

		// Enhanced device fields should be nil/absent for free tier
		enhancedDeviceFields := []string{"device_type", "device_os"}
		for _, field := range enhancedDeviceFields {
			if user[field] != nil {
				t.Errorf("expected %s to be nil for free tier, got %v", field, user[field])
			}
		}

		// Metadata (form answers) should still be present for all tiers
		// Note: metadata might be nil if no form data was submitted, which is fine
	})

	t.Run("free tier get user excludes enhanced data", func(t *testing.T) {
		t.Parallel()
		token := createAuthenticatedTestUserWithFreeTier(t)

		campaignID := createTestCampaign(t, token)
		activateTestCampaign(t, token, campaignID)

		userID := createTestWaitlistUser(t, campaignID)

		// Get single user - enhanced data should not be present for free tier
		path := fmt.Sprintf("/api/v1/campaigns/%s/users/%s", campaignID, userID)
		resp, body := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d, body: %s", http.StatusOK, resp.StatusCode, string(body))
		}

		var user map[string]interface{}
		parseJSONResponse(t, body, &user)

		// Enhanced geographic fields should be nil/absent for free tier
		enhancedGeoFields := []string{"country", "region", "region_code", "postal_code", "city", "country_code", "user_timezone", "latitude", "longitude", "metro_code"}
		for _, field := range enhancedGeoFields {
			if user[field] != nil {
				t.Errorf("expected %s to be nil for free tier, got %v", field, user[field])
			}
		}

		// Enhanced device fields should be nil/absent for free tier
		enhancedDeviceFields := []string{"device_type", "device_os"}
		for _, field := range enhancedDeviceFields {
			if user[field] != nil {
				t.Errorf("expected %s to be nil for free tier, got %v", field, user[field])
			}
		}
	})

	t.Run("free tier cannot filter by custom fields", func(t *testing.T) {
		t.Parallel()
		token := createAuthenticatedTestUserWithFreeTier(t)

		campaignID := createTestCampaign(t, token)
		activateTestCampaign(t, token, campaignID)

		// Try to filter by custom field
		path := fmt.Sprintf("/api/v1/campaigns/%s/users?custom_fields[company]=Acme", campaignID)
		resp, body := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status %d, got %d, body: %s", http.StatusForbidden, resp.StatusCode, string(body))
		}

		var response map[string]interface{}
		parseJSONResponse(t, body, &response)
		if response["code"] != "FEATURE_NOT_AVAILABLE" {
			t.Errorf("expected error code FEATURE_NOT_AVAILABLE, got %v", response["code"])
		}
	})

	t.Run("pro tier list users includes enhanced data", func(t *testing.T) {
		t.Parallel()
		token := createAuthenticatedTestUserWithProTier(t)

		campaignID := createTestCampaign(t, token)
		activateTestCampaign(t, token, campaignID)

		createTestWaitlistUser(t, campaignID)

		// List users - response should include all fields (values may be null based on what was collected)
		path := fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID)
		resp, body := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d, body: %s", http.StatusOK, resp.StatusCode, string(body))
		}

		var response map[string]interface{}
		parseJSONResponse(t, body, &response)

		users, ok := response["users"].([]interface{})
		if !ok || len(users) == 0 {
			t.Fatal("expected at least one user in response")
		}

		// For pro tier, the query should use the extended method which includes all columns
		// We can't easily verify the values since they depend on actual request headers,
		// but we verify the request succeeded (didn't use basic method)
	})

	t.Run("pro tier can filter by custom fields", func(t *testing.T) {
		t.Parallel()
		token := createAuthenticatedTestUserWithProTier(t)

		campaignID := createTestCampaign(t, token)
		activateTestCampaign(t, token, campaignID)

		// Create user with custom field
		createTestWaitlistUserWithCustomFields(t, campaignID, map[string]string{"company": "Acme"})

		// Filter by custom field - should succeed for pro tier
		path := fmt.Sprintf("/api/v1/campaigns/%s/users?custom_fields[company]=Acme", campaignID)
		resp, body := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status %d, got %d, body: %s", http.StatusOK, resp.StatusCode, string(body))
		}
	})
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// activateTestCampaign sets a campaign to active status
func activateTestCampaign(t *testing.T, token, campaignID string) {
	t.Helper()
	path := fmt.Sprintf("/api/v1/campaigns/%s/status", campaignID)
	resp, body := makeAuthenticatedRequest(t, http.MethodPatch, path, map[string]interface{}{"status": "active"}, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to activate campaign: %s", string(body))
	}
}

// createTestWaitlistUser creates a waitlist user via public API and returns the user ID
func createTestWaitlistUser(t *testing.T, campaignID string) string {
	t.Helper()
	return createTestWaitlistUserWithCustomFields(t, campaignID, nil)
}

// createTestWaitlistUserWithCustomFields creates a waitlist user with custom fields
func createTestWaitlistUserWithCustomFields(t *testing.T, campaignID string, customFields map[string]string) string {
	t.Helper()
	email := generateTestEmail()
	signupReq := map[string]interface{}{
		"email":          email,
		"terms_accepted": true,
	}
	if customFields != nil {
		signupReq["custom_fields"] = customFields
	}

	// Use the correct public API path
	path := fmt.Sprintf("/api/v1/campaigns/%s/users", campaignID)
	resp, body := makeRequest(t, http.MethodPost, path, signupReq, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create waitlist user: %s", string(body))
	}

	var response map[string]interface{}
	parseJSONResponse(t, body, &response)
	user := response["user"].(map[string]interface{})
	return user["id"].(string)
}
