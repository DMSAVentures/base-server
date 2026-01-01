//go:build integration
// +build integration

package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create authenticated user and return token
// Now uses direct database insertion to bypass Stripe dependency
func createAuthenticatedUser(t *testing.T) string {
	return createAuthenticatedTestUser(t)
}

func TestAPI_Campaign_Create(t *testing.T) {
	t.Parallel()
	// Use Pro tier to allow creating multiple campaigns (up to 5)
	token := createAuthenticatedTestUserWithProTier(t)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		wantError      bool
		validate       func(t *testing.T, resp *APIResponse)
	}{
		{
			name: "create waitlist campaign successfully",
			request: map[string]interface{}{
				"name":            "Test Waitlist Campaign",
				"slug":            generateTestCampaignSlug(),
				"description":     "A test waitlist campaign",
				"type":            "waitlist",
				"form_config":     map[string]interface{}{},
				"referral_config": map[string]interface{}{},
				"email_config":    map[string]interface{}{},
				"branding_config": map[string]interface{}{},
			},
			expectedStatus: http.StatusCreated,
			validate: func(t *testing.T, resp *APIResponse) {
				resp.AssertJSONFieldNotNil("id")
				resp.AssertJSONField("name", "Test Waitlist Campaign")
				resp.AssertJSONField("type", "waitlist")
				resp.AssertJSONField("status", "draft")
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
			validate: func(t *testing.T, resp *APIResponse) {
				resp.AssertJSONField("type", "referral")
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
			validate: func(t *testing.T, resp *APIResponse) {
				resp.AssertJSONField("type", "contest")
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
			validate: func(t *testing.T, resp *APIResponse) {
				resp.AssertJSONField("description", "Campaign with all fields")
				resp.AssertJSONField("privacy_policy_url", "https://example.com/privacy")
				resp.AssertJSONField("terms_url", "https://example.com/terms")
			},
		},
		{
			name: "create fails without name",
			request: map[string]interface{}{
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
			},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name: "create fails without slug",
			request: map[string]interface{}{
				"name": "Test Campaign",
				"type": "waitlist",
			},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name: "create fails without type",
			request: map[string]interface{}{
				"name": "Test Campaign",
				"slug": generateTestCampaignSlug(),
			},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name: "create fails with invalid type",
			request: map[string]interface{}{
				"name": "Test Campaign",
				"slug": generateTestCampaignSlug(),
				"type": "invalid_type",
			},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := POST(t, "/api/v1/campaigns").
				WithToken(token).
				WithBody(tt.request).
				Do()

			resp.AssertStatus(tt.expectedStatus)
			if tt.wantError {
				resp.AssertError()
			}
			if tt.validate != nil {
				tt.validate(t, resp)
			}
		})
	}
}

func TestAPI_Campaign_List(t *testing.T) {
	t.Parallel()
	// Use Pro tier to allow creating multiple campaigns (up to 5)
	token := createAuthenticatedTestUserWithProTier(t)

	// Create a few test campaigns
	for i := 0; i < 3; i++ {
		resp := POST(t, "/api/v1/campaigns").
			WithToken(token).
			WithBody(map[string]interface{}{
				"name":            fmt.Sprintf("List Test Campaign %d", i),
				"slug":            generateTestCampaignSlug(),
				"type":            "waitlist",
				"form_config":     map[string]interface{}{},
				"referral_config": map[string]interface{}{},
				"email_config":    map[string]interface{}{},
				"branding_config": map[string]interface{}{},
			}).
			Do()
		resp.RequireStatus(http.StatusCreated)
	}

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		validate       func(t *testing.T, resp *APIResponse)
	}{
		{
			name:           "list all campaigns without filters",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp *APIResponse) {
				data := resp.JSON()
				campaigns, ok := data["campaigns"].([]interface{})
				require.True(t, ok, "Expected 'campaigns' array in response")
				assert.GreaterOrEqual(t, len(campaigns), 3, "Expected at least 3 campaigns")

				pagination, ok := data["pagination"].(map[string]interface{})
				require.True(t, ok, "Expected 'pagination' object in response")
				assert.NotNil(t, pagination["page"])
				assert.NotNil(t, pagination["page_size"])
			},
		},
		{
			name:           "list campaigns with pagination",
			queryParams:    "?page=1&limit=2",
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp *APIResponse) {
				data := resp.JSON()
				campaigns, ok := data["campaigns"].([]interface{})
				require.True(t, ok, "Expected 'campaigns' array in response")
				assert.LessOrEqual(t, len(campaigns), 2, "Expected max 2 campaigns with limit=2")
			},
		},
		{
			name:           "list campaigns filtered by status",
			queryParams:    "?status=draft",
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp *APIResponse) {
				data := resp.JSON()
				campaigns, ok := data["campaigns"].([]interface{})
				require.True(t, ok, "Expected 'campaigns' array in response")

				for _, c := range campaigns {
					campaign := c.(map[string]interface{})
					assert.Equal(t, "draft", campaign["status"], "Expected all campaigns to have status 'draft'")
				}
			},
		},
		{
			name:           "list campaigns filtered by type",
			queryParams:    "?type=waitlist",
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp *APIResponse) {
				data := resp.JSON()
				campaigns, ok := data["campaigns"].([]interface{})
				require.True(t, ok, "Expected 'campaigns' array in response")

				for _, c := range campaigns {
					campaign := c.(map[string]interface{})
					assert.Equal(t, "waitlist", campaign["type"], "Expected all campaigns to have type 'waitlist'")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := GET(t, "/api/v1/campaigns"+tt.queryParams).
				WithToken(token).
				Do()

			resp.AssertStatus(tt.expectedStatus)
			if tt.validate != nil {
				tt.validate(t, resp)
			}
		})
	}
}

func TestAPI_Campaign_GetByID(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedUser(t)

	// Create a test campaign
	createResp := POST(t, "/api/v1/campaigns").
		WithToken(token).
		WithBody(map[string]interface{}{
			"name":            "Get By ID Test Campaign",
			"slug":            generateTestCampaignSlug(),
			"type":            "waitlist",
			"form_config":     map[string]interface{}{},
			"referral_config": map[string]interface{}{},
			"email_config":    map[string]interface{}{},
			"branding_config": map[string]interface{}{},
		}).
		Do()

	createResp.RequireStatus(http.StatusCreated)
	campaignID := createResp.JSON()["id"].(string)
	require.NotEmpty(t, campaignID)

	tests := []struct {
		name           string
		campaignID     string
		expectedStatus int
		wantError      bool
		validate       func(t *testing.T, resp *APIResponse)
	}{
		{
			name:           "get campaign by valid ID",
			campaignID:     campaignID,
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp *APIResponse) {
				resp.AssertJSONField("id", campaignID)
				resp.AssertJSONField("name", "Get By ID Test Campaign")
			},
		},
		{
			name:           "get campaign fails with invalid UUID",
			campaignID:     "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name:           "get campaign fails with non-existent UUID",
			campaignID:     "00000000-0000-0000-0000-000000000000",
			expectedStatus: http.StatusNotFound,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := GET(t, fmt.Sprintf("/api/v1/campaigns/%s", tt.campaignID)).
				WithToken(token).
				Do()

			resp.AssertStatus(tt.expectedStatus)
			if tt.wantError {
				resp.AssertError()
			}
			if tt.validate != nil {
				tt.validate(t, resp)
			}
		})
	}
}

func TestAPI_Campaign_Update(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedUser(t)

	// Create a test campaign
	createResp := POST(t, "/api/v1/campaigns").
		WithToken(token).
		WithBody(map[string]interface{}{
			"name":            "Update Test Campaign",
			"slug":            generateTestCampaignSlug(),
			"type":            "waitlist",
			"form_config":     map[string]interface{}{},
			"referral_config": map[string]interface{}{},
			"email_config":    map[string]interface{}{},
			"branding_config": map[string]interface{}{},
		}).
		Do()

	createResp.RequireStatus(http.StatusCreated)
	campaignID := createResp.JSON()["id"].(string)
	require.NotEmpty(t, campaignID)

	tests := []struct {
		name           string
		campaignID     string
		request        map[string]interface{}
		expectedStatus int
		wantError      bool
		validate       func(t *testing.T, resp *APIResponse)
	}{
		{
			name:       "update campaign name",
			campaignID: campaignID,
			request: map[string]interface{}{
				"name": "Updated Campaign Name",
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp *APIResponse) {
				resp.AssertJSONField("name", "Updated Campaign Name")
			},
		},
		{
			name:       "update campaign description",
			campaignID: campaignID,
			request: map[string]interface{}{
				"description": "Updated description",
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp *APIResponse) {
				resp.AssertJSONField("description", "Updated description")
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
			validate: func(t *testing.T, resp *APIResponse) {
				resp.AssertJSONField("name", "Multi Update Campaign")
				resp.AssertJSONField("description", "Multi field update")
			},
		},
		{
			name:           "update fails with invalid campaign ID",
			campaignID:     "invalid-uuid",
			request:        map[string]interface{}{"name": "New Name"},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := PUT(t, fmt.Sprintf("/api/v1/campaigns/%s", tt.campaignID)).
				WithToken(token).
				WithBody(tt.request).
				Do()

			resp.AssertStatus(tt.expectedStatus)
			if tt.wantError {
				resp.AssertError()
			}
			if tt.validate != nil {
				tt.validate(t, resp)
			}
		})
	}
}

func TestAPI_Campaign_UpdateStatus(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedUser(t)

	// Create a test campaign
	createResp := POST(t, "/api/v1/campaigns").
		WithToken(token).
		WithBody(map[string]interface{}{
			"name":            "Status Update Test Campaign",
			"slug":            generateTestCampaignSlug(),
			"type":            "waitlist",
			"form_config":     map[string]interface{}{},
			"referral_config": map[string]interface{}{},
			"email_config":    map[string]interface{}{},
			"branding_config": map[string]interface{}{},
		}).
		Do()

	createResp.RequireStatus(http.StatusCreated)
	campaignID := createResp.JSON()["id"].(string)
	require.NotEmpty(t, campaignID)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		wantError      bool
		expectedField  string
	}{
		{
			name:           "update status to active",
			request:        map[string]interface{}{"status": "active"},
			expectedStatus: http.StatusOK,
			expectedField:  "active",
		},
		{
			name:           "update status to paused",
			request:        map[string]interface{}{"status": "paused"},
			expectedStatus: http.StatusOK,
			expectedField:  "paused",
		},
		{
			name:           "update status to completed",
			request:        map[string]interface{}{"status": "completed"},
			expectedStatus: http.StatusOK,
			expectedField:  "completed",
		},
		{
			name:           "update status to draft",
			request:        map[string]interface{}{"status": "draft"},
			expectedStatus: http.StatusOK,
			expectedField:  "draft",
		},
		{
			name:           "update fails with invalid status",
			request:        map[string]interface{}{"status": "invalid_status"},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name:           "update fails without status field",
			request:        map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := PATCH(t, fmt.Sprintf("/api/v1/campaigns/%s/status", campaignID)).
				WithToken(token).
				WithBody(tt.request).
				Do()

			resp.AssertStatus(tt.expectedStatus)
			if tt.wantError {
				resp.AssertError()
			} else if tt.expectedField != "" {
				resp.AssertJSONField("status", tt.expectedField)
			}
		})
	}
}

func TestAPI_Campaign_Delete(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedUser(t)

	// Helper to create a campaign for deletion
	createCampaign := func() string {
		resp := POST(t, "/api/v1/campaigns").
			WithToken(token).
			WithBody(map[string]interface{}{
				"name":            "Delete Test Campaign",
				"slug":            generateTestCampaignSlug(),
				"type":            "waitlist",
				"form_config":     map[string]interface{}{},
				"referral_config": map[string]interface{}{},
				"email_config":    map[string]interface{}{},
				"branding_config": map[string]interface{}{},
			}).
			Do()
		resp.RequireStatus(http.StatusCreated)
		return resp.JSON()["id"].(string)
	}

	tests := []struct {
		name           string
		getCampaignID  func() string
		expectedStatus int
		verifyDeleted  bool
		wantError      bool
	}{
		{
			name:           "delete campaign successfully",
			getCampaignID:  createCampaign,
			expectedStatus: http.StatusNoContent,
			verifyDeleted:  true,
		},
		{
			name:           "delete fails with invalid campaign ID",
			getCampaignID:  func() string { return "invalid-uuid" },
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name:           "delete fails with non-existent campaign ID",
			getCampaignID:  func() string { return "00000000-0000-0000-0000-000000000000" },
			expectedStatus: http.StatusNotFound,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			campaignID := tt.getCampaignID()

			resp := DELETE(t, fmt.Sprintf("/api/v1/campaigns/%s", campaignID)).
				WithToken(token).
				Do()

			resp.AssertStatus(tt.expectedStatus)

			if tt.verifyDeleted {
				// Verify the campaign was deleted
				getResp := GET(t, fmt.Sprintf("/api/v1/campaigns/%s", campaignID)).
					WithToken(token).
					Do()
				getResp.AssertStatus(http.StatusNotFound)
			}
		})
	}
}

func TestAPI_Campaign_GetPublicCampaign(t *testing.T) {
	t.Parallel()
	// Use Pro tier to allow creating multiple campaigns for testing
	token := createAuthenticatedTestUserWithProTier(t)

	// Create a draft campaign with settings
	draftResp := POST(t, "/api/v1/campaigns").
		WithToken(token).
		WithBody(map[string]interface{}{
			"name":              "Public Draft Campaign",
			"slug":              generateTestCampaignSlug(),
			"description":       "A draft campaign for public access",
			"type":              "waitlist",
			"form_settings":     map[string]interface{}{"captcha_enabled": false},
			"referral_settings": map[string]interface{}{"enabled": true},
			"email_settings":    map[string]interface{}{},
			"branding_settings": map[string]interface{}{"primary_color": "#000000"},
		}).
		Do()
	draftResp.RequireStatus(http.StatusCreated)
	draftCampaignID := draftResp.JSON()["id"].(string)

	// Create an active campaign with settings
	activeResp := POST(t, "/api/v1/campaigns").
		WithToken(token).
		WithBody(map[string]interface{}{
			"name":              "Public Active Campaign",
			"slug":              generateTestCampaignSlug(),
			"type":              "referral",
			"form_settings":     map[string]interface{}{},
			"referral_settings": map[string]interface{}{},
			"email_settings":    map[string]interface{}{},
			"branding_settings": map[string]interface{}{},
		}).
		Do()
	activeResp.RequireStatus(http.StatusCreated)
	activeCampaignID := activeResp.JSON()["id"].(string)

	// Update status to active
	statusResp := PATCH(t, fmt.Sprintf("/api/v1/campaigns/%s/status", activeCampaignID)).
		WithToken(token).
		WithBody(map[string]interface{}{"status": "active"}).
		Do()
	statusResp.RequireStatus(http.StatusOK)

	tests := []struct {
		name           string
		campaignID     string
		expectedStatus int
		wantError      bool
		validate       func(t *testing.T, resp *APIResponse)
	}{
		{
			name:           "get public campaign successfully with draft status",
			campaignID:     draftCampaignID,
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp *APIResponse) {
				resp.AssertJSONField("id", draftCampaignID)
				resp.AssertJSONField("name", "Public Draft Campaign")
				resp.AssertJSONField("type", "waitlist")
				resp.AssertJSONField("status", "draft")
				resp.AssertJSONFieldNotNil("form_settings")
				resp.AssertJSONFieldNotNil("referral_settings")
				resp.AssertJSONFieldNotNil("branding_settings")
				resp.AssertJSONFieldNotNil("account_id")
			},
		},
		{
			name:           "get public campaign successfully with active status",
			campaignID:     activeCampaignID,
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp *APIResponse) {
				resp.AssertJSONField("id", activeCampaignID)
				resp.AssertJSONField("name", "Public Active Campaign")
				resp.AssertJSONField("status", "active")
			},
		},
		{
			name:           "get public campaign fails with invalid UUID",
			campaignID:     "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
			validate: func(t *testing.T, resp *APIResponse) {
				data := resp.JSON()
				assert.NotNil(t, data["error"], "Expected 'error' field in response")
				assert.NotNil(t, data["code"], "Expected 'code' field in response")
				assert.Equal(t, "INVALID_INPUT", data["code"], "Expected error code 'INVALID_INPUT'")

				errorMsg, ok := data["error"].(string)
				require.True(t, ok, "Expected 'error' to be a string")
				assert.NotEmpty(t, errorMsg, "Expected non-empty error message")
			},
		},
		{
			name:           "get public campaign fails with non-existent UUID",
			campaignID:     "00000000-0000-0000-0000-000000000000",
			expectedStatus: http.StatusNotFound,
			wantError:      true,
			validate: func(t *testing.T, resp *APIResponse) {
				data := resp.JSON()
				assert.NotNil(t, data["error"], "Expected 'error' field in response")
				assert.NotNil(t, data["code"], "Expected 'code' field in response")
				assert.Equal(t, "NOT_FOUND", data["code"], "Expected error code 'NOT_FOUND'")

				errorMsg := data["error"].(string)
				assert.False(t, containsSensitiveInfo(errorMsg), "Error message contains sensitive internal information")
			},
		},
		{
			name:           "get public campaign - no authentication required",
			campaignID:     draftCampaignID,
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp *APIResponse) {
				// Verify public endpoint works without authentication
				resp.AssertJSONField("id", draftCampaignID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Public campaign endpoint - no authentication required
			resp := GET(t, fmt.Sprintf("/api/v1/%s", tt.campaignID)).Do()

			resp.AssertStatus(tt.expectedStatus)
			if tt.wantError {
				resp.AssertError()
			}
			if tt.validate != nil {
				tt.validate(t, resp)
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
	t.Parallel()
	// Use Pro tier to allow creating multiple campaigns (up to 5)
	token := createAuthenticatedTestUserWithProTier(t)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		wantError      bool
	}{
		{
			name: "valid form fields with email and text",
			request: map[string]interface{}{
				"name": "Campaign with Form Fields",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_fields": []map[string]interface{}{
					{"name": "email_field", "field_type": "email", "label": "Email Address", "placeholder": "Enter your email", "required": true, "display_order": 1},
					{"name": "name_field", "field_type": "text", "label": "Full Name", "required": true, "display_order": 2},
				},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "valid form fields with all field types",
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
			name: "fails with invalid field_type",
			request: map[string]interface{}{
				"name": "Campaign with Invalid Field Type",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_fields": []map[string]interface{}{
					{"name": "test_field", "field_type": "invalid_type", "label": "Test Field", "display_order": 1},
				},
			},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name: "fails with empty field name",
			request: map[string]interface{}{
				"name": "Campaign with Empty Field Name",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_fields": []map[string]interface{}{
					{"name": "", "field_type": "text", "label": "Test Field", "display_order": 1},
				},
			},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name: "fails with empty field label",
			request: map[string]interface{}{
				"name": "Campaign with Empty Field Label",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_fields": []map[string]interface{}{
					{"name": "test_field", "field_type": "text", "label": "", "display_order": 1},
				},
			},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name: "fails with missing required field_type",
			request: map[string]interface{}{
				"name": "Campaign Missing Field Type",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_fields": []map[string]interface{}{
					{"name": "test_field", "label": "Test Field", "display_order": 1},
				},
			},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := POST(t, "/api/v1/campaigns").
				WithToken(token).
				WithBody(tt.request).
				Do()

			resp.AssertStatus(tt.expectedStatus)

			if tt.wantError {
				resp.AssertError()
			} else if tt.expectedStatus == http.StatusCreated {
				resp.AssertJSONFieldNotNil("id")
			}
		})
	}
}

// TestAPI_Campaign_CreateWithShareMessages tests share message validation
func TestAPI_Campaign_CreateWithShareMessages(t *testing.T) {
	t.Parallel()
	// Use Pro tier to allow creating multiple campaigns (up to 5)
	token := createAuthenticatedTestUserWithProTier(t)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		wantError      bool
	}{
		{
			name: "valid share messages for all channels",
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
			name: "fails with invalid share channel",
			request: map[string]interface{}{
				"name": "Campaign with Invalid Channel",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"share_messages": []map[string]interface{}{
					{"channel": "instagram", "message": "Invalid channel!"},
				},
			},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name: "fails with empty share message",
			request: map[string]interface{}{
				"name": "Campaign with Empty Message",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"share_messages": []map[string]interface{}{
					{"channel": "email", "message": ""},
				},
			},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name: "fails with missing channel",
			request: map[string]interface{}{
				"name": "Campaign Missing Channel",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"share_messages": []map[string]interface{}{
					{"message": "Message without channel"},
				},
			},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := POST(t, "/api/v1/campaigns").
				WithToken(token).
				WithBody(tt.request).
				Do()

			resp.AssertStatus(tt.expectedStatus)
			if tt.wantError {
				resp.AssertError()
			}
		})
	}
}

// TestAPI_Campaign_CreateWithTrackingIntegrations tests tracking integration validation
func TestAPI_Campaign_CreateWithTrackingIntegrations(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedUser(t)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		wantError      bool
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
			wantError:      true,
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
			wantError:      true,
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
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := POST(t, "/api/v1/campaigns").
				WithToken(token).
				WithBody(tt.request).
				Do()

			resp.AssertStatus(tt.expectedStatus)
			if tt.wantError {
				resp.AssertError()
			} else if tt.expectedStatus == http.StatusCreated {
				resp.AssertJSONFieldNotNil("id")
			}
		})
	}
}

// TestAPI_Campaign_CreateWithReferralSettings tests referral settings validation
func TestAPI_Campaign_CreateWithReferralSettings(t *testing.T) {
	t.Parallel()
	// Use Pro tier to allow creating multiple campaigns (up to 5)
	token := createAuthenticatedTestUserWithProTier(t)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		wantError      bool
	}{
		{
			name: "create campaign with valid referral settings",
			request: map[string]interface{}{
				"name": "Campaign with Referral Settings",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"referral_settings": map[string]interface{}{
					"enabled":                    true,
					"points_per_referral":        10,
					"verified_only":              true,
					"positions_to_jump":          5,
					"referrer_positions_to_jump": 2,
					"sharing_channels":           []string{"email", "twitter", "facebook"},
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
					"enabled":                    false,
					"points_per_referral":        0,
					"positions_to_jump":          0,
					"referrer_positions_to_jump": 0,
					"sharing_channels":           []string{},
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
			wantError:      true,
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
			wantError:      true,
		},
		{
			name: "create fails with negative referrer_positions_to_jump",
			request: map[string]interface{}{
				"name": "Campaign with Negative Referrer Positions",
				"slug": generateTestCampaignSlug(),
				"type": "referral",
				"referral_settings": map[string]interface{}{
					"enabled":                    true,
					"referrer_positions_to_jump": -3,
					"sharing_channels":           []string{"email"},
				},
			},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
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
			wantError:      true,
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
			resp := POST(t, "/api/v1/campaigns").
				WithToken(token).
				WithBody(tt.request).
				Do()

			resp.AssertStatus(tt.expectedStatus)
			if tt.wantError {
				resp.AssertError()
			} else if tt.expectedStatus == http.StatusCreated {
				resp.AssertJSONFieldNotNil("id")
			}
		})
	}
}

// TestAPI_Campaign_CreateWithFormSettings tests form settings validation
func TestAPI_Campaign_CreateWithFormSettings(t *testing.T) {
	t.Parallel()
	// Use Team tier to allow creating unlimited campaigns
	token := createAuthenticatedTestUserWithTeamTier(t)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		wantError      bool
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
			name: "create campaign with form settings but no design field",
			request: map[string]interface{}{
				"name": "Campaign without Design",
				"slug": generateTestCampaignSlug(),
				"type": "waitlist",
				"form_settings": map[string]interface{}{
					"captcha_enabled": true,
					"captcha_provider": "turnstile",
					"captcha_site_key": "0x123456789",
					"double_opt_in":    true,
					"success_title":    "Welcome!",
					"success_message":  "Thanks for signing up.",
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
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := POST(t, "/api/v1/campaigns").
				WithToken(token).
				WithBody(tt.request).
				Do()

			resp.AssertStatus(tt.expectedStatus)
			if tt.wantError {
				resp.AssertError()
			} else if tt.expectedStatus == http.StatusCreated {
				resp.AssertJSONFieldNotNil("id")
			}
		})
	}
}

// TestAPI_Campaign_CreateWithFullSettings tests creating a campaign with all settings
func TestAPI_Campaign_CreateWithFullSettings(t *testing.T) {
	t.Parallel()
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
			"enabled":                    true,
			"points_per_referral":        25,
			"verified_only":              true,
			"positions_to_jump":          10,
			"referrer_positions_to_jump": 5,
			"sharing_channels":           []string{"email", "twitter", "linkedin"},
		},
		"form_fields": []map[string]interface{}{
			{"name": "email", "field_type": "email", "label": "Email Address", "placeholder": "you@example.com", "required": true, "display_order": 1},
			{"name": "full_name", "field_type": "text", "label": "Full Name", "required": true, "display_order": 2},
			{"name": "company", "field_type": "text", "label": "Company", "required": false, "display_order": 3},
		},
		"share_messages": []map[string]interface{}{
			{"channel": "email", "message": "Join me on this amazing campaign!"},
			{"channel": "twitter", "message": "Check out this waitlist! #launch"},
			{"channel": "linkedin", "message": "Exciting new product launching soon!"},
		},
		"tracking_integrations": []map[string]interface{}{
			{"integration_type": "google_analytics", "enabled": true, "tracking_id": "GA-123456789", "tracking_label": "waitlist_signup"},
			{"integration_type": "meta_pixel", "enabled": true, "tracking_id": "987654321"},
		},
	}

	// Create campaign
	createResp := POST(t, "/api/v1/campaigns").
		WithToken(token).
		WithBody(request).
		Do()

	createResp.RequireStatus(http.StatusCreated)
	createResp.AssertJSONFieldNotNil("id")
	createResp.AssertJSONField("name", "Full Featured Campaign")
	createResp.AssertJSONField("type", "referral")
	createResp.AssertJSONField("status", "draft")

	campaignID := createResp.JSON()["id"].(string)

	// Retrieve campaign and verify settings are loaded
	getResp := GET(t, fmt.Sprintf("/api/v1/campaigns/%s", campaignID)).
		WithToken(token).
		Do()

	getResp.RequireStatus(http.StatusOK)
	getResp.AssertJSONFieldNotNil("email_settings")
	getResp.AssertJSONFieldNotNil("branding_settings")
	getResp.AssertJSONFieldNotNil("form_settings")
	getResp.AssertJSONFieldNotNil("referral_settings")
}

// TestAPI_Campaign_UpdateWithSettings tests updating campaign settings with validation
func TestAPI_Campaign_UpdateWithSettings(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedUser(t)

	// Create a base campaign first
	createResp := POST(t, "/api/v1/campaigns").
		WithToken(token).
		WithBody(map[string]interface{}{
			"name": "Campaign to Update",
			"slug": generateTestCampaignSlug(),
			"type": "waitlist",
		}).
		Do()

	createResp.RequireStatus(http.StatusCreated)
	campaignID := createResp.JSON()["id"].(string)
	require.NotEmpty(t, campaignID)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		wantError      bool
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
			wantError:      true,
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
			wantError:      true,
		},
		{
			name: "update fails with negative referral points",
			request: map[string]interface{}{
				"referral_settings": map[string]interface{}{
					"points_per_referral": -10,
				},
			},
			expectedStatus: http.StatusBadRequest,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := PUT(t, fmt.Sprintf("/api/v1/campaigns/%s", campaignID)).
				WithToken(token).
				WithBody(tt.request).
				Do()

			resp.AssertStatus(tt.expectedStatus)
			if tt.wantError {
				resp.AssertError()
			}
		})
	}
}

// TestAPI_Campaign_DuplicateSlug tests that duplicate slugs are rejected
func TestAPI_Campaign_DuplicateSlug(t *testing.T) {
	t.Parallel()
	// Use Pro tier to allow creating multiple campaigns for duplicate slug test
	token := createAuthenticatedTestUserWithProTier(t)
	slug := generateTestCampaignSlug()

	// Create first campaign with the slug
	firstResp := POST(t, "/api/v1/campaigns").
		WithToken(token).
		WithBody(map[string]interface{}{
			"name": "First Campaign",
			"slug": slug,
			"type": "waitlist",
		}).
		Do()

	firstResp.RequireStatus(http.StatusCreated)
	firstResp.AssertJSONFieldNotNil("id")

	// Try to create second campaign with the same slug
	duplicateResp := POST(t, "/api/v1/campaigns").
		WithToken(token).
		WithBody(map[string]interface{}{
			"name": "Second Campaign",
			"slug": slug,
			"type": "waitlist",
		}).
		Do()

	duplicateResp.AssertStatus(http.StatusConflict)
	duplicateResp.AssertError()
}
