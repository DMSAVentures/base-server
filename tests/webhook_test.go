//go:build integration
// +build integration

package tests

import (
	"fmt"
	"net"
	"net/http"
	"testing"
)

func TestAPI_Webhook_Create(t *testing.T) {
	token := createAuthenticatedUser(t)

	// Create a campaign for testing webhook with campaign_id
	campaignReq := map[string]interface{}{
		"name":            "Webhook Test Campaign",
		"slug":            generateTestCampaignSlug(),
		"type":            "waitlist",
		"form_config":     map[string]interface{}{},
		"referral_config": map[string]interface{}{},
		"email_config":    map[string]interface{}{},
		"branding_config": map[string]interface{}{},
	}
	campaignResp, campaignBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/v1/campaigns", campaignReq, token)
	if campaignResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test campaign: %s", string(campaignBody))
	}
	var campaignData map[string]interface{}
	parseJSONResponse(t, campaignBody, &campaignData)
	testCampaignID := campaignData["id"].(string)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name: "create webhook successfully",
			request: map[string]interface{}{
				"url":    "https://example.com/webhook",
				"events": []string{"user.created", "user.verified"},
			},
			expectedStatus: http.StatusCreated,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				webhook, ok := response["webhook"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected 'webhook' object in response")
				}

				if webhook["id"] == nil {
					t.Error("Expected webhook ID in response")
				}
				if webhook["url"] != "https://example.com/webhook" {
					t.Error("Expected URL to match request")
				}

				events, ok := webhook["events"].([]interface{})
				if !ok || len(events) != 2 {
					t.Error("Expected 2 events in response")
				}

				secret, ok := response["secret"].(string)
				if !ok || secret == "" {
					t.Error("Expected webhook secret in response")
				}
			},
		},
		{
			name: "create webhook with campaign ID",
			request: map[string]interface{}{
				"url":         "https://example.com/webhook",
				"events":      []string{"user.created"},
				"campaign_id": testCampaignID,
			},
			expectedStatus: http.StatusCreated,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				webhook, ok := response["webhook"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected 'webhook' object in response")
				}

				if webhook["campaign_id"] == nil {
					t.Error("Expected campaign_id in response")
				}
			},
		},
		{
			name: "create webhook with retry configuration",
			request: map[string]interface{}{
				"url":           "https://example.com/webhook",
				"events":        []string{"user.created"},
				"retry_enabled": true,
				"max_retries":   5,
			},
			expectedStatus: http.StatusCreated,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				webhook, ok := response["webhook"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected 'webhook' object in response")
				}

				if webhook["retry_enabled"] != true {
					t.Error("Expected retry_enabled to be true")
				}
			},
		},
		{
			name: "create fails without URL",
			request: map[string]interface{}{
				"events": []string{"user.created"},
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
			name: "create fails without events",
			request: map[string]interface{}{
				"url": "https://example.com/webhook",
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
			name: "create fails with empty events array",
			request: map[string]interface{}{
				"url":    "https://example.com/webhook",
				"events": []string{},
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
			name: "create fails with invalid URL",
			request: map[string]interface{}{
				"url":    "not-a-valid-url",
				"events": []string{"user.created"},
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
			resp, body := makeAuthenticatedRequest(t, http.MethodPost, "/api/protected/webhooks", tt.request, token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

func TestAPI_Webhook_List(t *testing.T) {
	token := createAuthenticatedUser(t)

	// Create a few test webhooks
	for i := 0; i < 3; i++ {
		req := map[string]interface{}{
			"url":    fmt.Sprintf("https://example.com/webhook-%d", i),
			"events": []string{"user.created"},
		}
		makeAuthenticatedRequest(t, http.MethodPost, "/api/protected/webhooks", req, token)
	}

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name:           "list all webhooks",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var webhooks []map[string]interface{}
				parseJSONResponse(t, body, &webhooks)

				if len(webhooks) < 3 {
					t.Errorf("Expected at least 3 webhooks, got %d", len(webhooks))
				}

				for _, webhook := range webhooks {
					if webhook["id"] == nil {
						t.Error("Expected webhook ID")
					}
					if webhook["url"] == nil {
						t.Error("Expected webhook URL")
					}
					if webhook["events"] == nil {
						t.Error("Expected webhook events")
					}
				}
			},
		},
		{
			name:           "list webhooks filtered by campaign",
			queryParams:    "?campaign_id=00000000-0000-0000-0000-000000000001",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var webhooks []map[string]interface{}
				parseJSONResponse(t, body, &webhooks)

				// Should return empty or only webhooks for that campaign
				for _, webhook := range webhooks {
					if webhook["campaign_id"] != nil {
						campaignID := webhook["campaign_id"].(string)
						if campaignID != "00000000-0000-0000-0000-000000000001" {
							t.Error("Expected only webhooks for specified campaign")
						}
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/protected/webhooks" + tt.queryParams
			resp, body := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

func TestAPI_Webhook_GetByID(t *testing.T) {
	token := createAuthenticatedUser(t)

	// Create a test webhook
	createReq := map[string]interface{}{
		"url":    "https://example.com/get-test",
		"events": []string{"user.created", "user.verified"},
	}
	createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/protected/webhooks", createReq, token)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test webhook: %s", string(createBody))
	}

	var createRespData map[string]interface{}
	parseJSONResponse(t, createBody, &createRespData)
	webhook := createRespData["webhook"].(map[string]interface{})
	webhookID := webhook["id"].(string)

	tests := []struct {
		name           string
		webhookID      string
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name:           "get webhook by valid ID",
			webhookID:      webhookID,
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var webhook map[string]interface{}
				parseJSONResponse(t, body, &webhook)

				if webhook["id"] != webhookID {
					t.Errorf("Expected webhook ID %s, got %v", webhookID, webhook["id"])
				}
				if webhook["url"] != "https://example.com/get-test" {
					t.Error("Expected URL to match created webhook")
				}

				events, ok := webhook["events"].([]interface{})
				if !ok || len(events) != 2 {
					t.Error("Expected 2 events")
				}
			},
		},
		{
			name:           "get webhook fails with invalid UUID",
			webhookID:      "invalid-uuid",
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
			name:           "get webhook fails with non-existent UUID",
			webhookID:      "00000000-0000-0000-0000-000000000000",
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
			path := fmt.Sprintf("/api/protected/webhooks/%s", tt.webhookID)
			resp, body := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

func TestAPI_Webhook_Update(t *testing.T) {
	token := createAuthenticatedUser(t)

	// Create a test webhook
	createReq := map[string]interface{}{
		"url":    "https://example.com/update-test",
		"events": []string{"user.created"},
	}
	createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/protected/webhooks", createReq, token)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test webhook: %s", string(createBody))
	}

	var createRespData map[string]interface{}
	parseJSONResponse(t, createBody, &createRespData)
	webhook := createRespData["webhook"].(map[string]interface{})
	webhookID := webhook["id"].(string)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name: "update webhook URL",
			request: map[string]interface{}{
				"url": "https://example.com/updated-webhook",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var webhook map[string]interface{}
				parseJSONResponse(t, body, &webhook)

				if webhook["url"] != "https://example.com/updated-webhook" {
					t.Error("Expected URL to be updated")
				}
			},
		},
		{
			name: "update webhook events",
			request: map[string]interface{}{
				"events": []string{"user.created", "user.verified", "reward.earned"},
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var webhook map[string]interface{}
				parseJSONResponse(t, body, &webhook)

				events, ok := webhook["events"].([]interface{})
				if !ok || len(events) != 3 {
					t.Error("Expected 3 events after update")
				}
			},
		},
		{
			name: "update webhook status",
			request: map[string]interface{}{
				"status": "paused",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var webhook map[string]interface{}
				parseJSONResponse(t, body, &webhook)

				if webhook["status"] != "paused" {
					t.Error("Expected status to be 'paused'")
				}
			},
		},
		{
			name: "update webhook retry settings",
			request: map[string]interface{}{
				"retry_enabled": true,
				"max_retries":   3,
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var webhook map[string]interface{}
				parseJSONResponse(t, body, &webhook)

				if webhook["retry_enabled"] != true {
					t.Error("Expected retry_enabled to be true")
				}
			},
		},
		{
			name: "update multiple fields",
			request: map[string]interface{}{
				"url":           "https://example.com/multi-update",
				"events":        []string{"user.created"},
				"retry_enabled": false,
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var webhook map[string]interface{}
				parseJSONResponse(t, body, &webhook)

				if webhook["url"] != "https://example.com/multi-update" {
					t.Error("Expected URL to be updated")
				}
				if webhook["retry_enabled"] != false {
					t.Error("Expected retry_enabled to be false")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fmt.Sprintf("/api/protected/webhooks/%s", webhookID)
			resp, body := makeAuthenticatedRequest(t, http.MethodPut, path, tt.request, token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

func TestAPI_Webhook_Delete(t *testing.T) {
	token := createAuthenticatedUser(t)

	tests := []struct {
		name           string
		setupFunc      func() string
		webhookID      string
		expectedStatus int
		validateFunc   func(t *testing.T, webhookID string)
	}{
		{
			name: "delete webhook successfully",
			setupFunc: func() string {
				req := map[string]interface{}{
					"url":    "https://example.com/delete-test",
					"events": []string{"user.created"},
				}
				resp, body := makeAuthenticatedRequest(t, http.MethodPost, "/api/protected/webhooks", req, token)
				if resp.StatusCode != http.StatusCreated {
					t.Fatalf("Failed to create test webhook: %s", string(body))
				}
				var respData map[string]interface{}
				parseJSONResponse(t, body, &respData)
				webhook := respData["webhook"].(map[string]interface{})
				return webhook["id"].(string)
			},
			expectedStatus: http.StatusNoContent,
			validateFunc: func(t *testing.T, webhookID string) {
				// Try to get the deleted webhook
				path := fmt.Sprintf("/api/protected/webhooks/%s", webhookID)
				resp, _ := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)
				if resp.StatusCode != http.StatusNotFound {
					t.Error("Expected webhook to be deleted")
				}
			},
		},
		{
			name:           "delete fails with invalid webhook ID",
			webhookID:      "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			validateFunc:   nil,
		},
		{
			name:           "delete fails with non-existent webhook ID",
			webhookID:      "00000000-0000-0000-0000-000000000000",
			expectedStatus: http.StatusNotFound,
			validateFunc:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhookID := tt.webhookID
			if tt.setupFunc != nil {
				webhookID = tt.setupFunc()
			}

			path := fmt.Sprintf("/api/protected/webhooks/%s", webhookID)
			resp, _ := makeAuthenticatedRequest(t, http.MethodDelete, path, nil, token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, webhookID)
			}
		})
	}
}

func TestAPI_Webhook_GetDeliveries(t *testing.T) {
	token := createAuthenticatedUser(t)

	// Create a test webhook
	createReq := map[string]interface{}{
		"url":    "https://example.com/deliveries-test",
		"events": []string{"user.created"},
	}
	createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/protected/webhooks", createReq, token)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test webhook: %s", string(createBody))
	}

	var createRespData map[string]interface{}
	parseJSONResponse(t, createBody, &createRespData)
	webhook := createRespData["webhook"].(map[string]interface{})
	webhookID := webhook["id"].(string)

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name:           "get webhook deliveries",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				deliveries, ok := response["deliveries"].([]interface{})
				if !ok {
					t.Fatal("Expected 'deliveries' array in response")
				}

				// May be empty if no deliveries yet
				_ = deliveries

				pagination, ok := response["pagination"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected 'pagination' object in response")
				}

				if pagination["page"] == nil || pagination["limit"] == nil {
					t.Error("Expected pagination details")
				}
			},
		},
		{
			name:           "get webhook deliveries with pagination",
			queryParams:    "?page=1&limit=10",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				if response["deliveries"] == nil {
					t.Error("Expected 'deliveries' in response")
				}
				if response["pagination"] == nil {
					t.Error("Expected 'pagination' in response")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fmt.Sprintf("/api/protected/webhooks/%s/deliveries%s", webhookID, tt.queryParams)
			resp, body := makeAuthenticatedRequest(t, http.MethodGet, path, nil, token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}

func TestAPI_Webhook_TestWebhook(t *testing.T) {
	token := createAuthenticatedUser(t)

	// Start a mock webhook server to receive webhook deliveries
	deliveryReceived := make(chan bool, 2)
	mockServer := http.NewServeMux()
	mockServer.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		// Successfully receive the webhook
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"received"}`))
		deliveryReceived <- true
	})

	server := &http.Server{
		Addr:    "127.0.0.1:0", // Random port
		Handler: mockServer,
	}

	// Start server in background
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		t.Fatalf("Failed to start mock webhook server: %v", err)
	}
	defer listener.Close()

	go server.Serve(listener)

	// Get the actual port
	mockServerURL := fmt.Sprintf("http://%s/webhook", listener.Addr().String())

	// Create a test webhook pointing to our mock server
	createReq := map[string]interface{}{
		"url":    mockServerURL,
		"events": []string{"user.created", "user.verified"},
	}
	createResp, createBody := makeAuthenticatedRequest(t, http.MethodPost, "/api/protected/webhooks", createReq, token)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test webhook: %s", string(createBody))
	}

	var createRespData map[string]interface{}
	parseJSONResponse(t, createBody, &createRespData)
	webhook := createRespData["webhook"].(map[string]interface{})
	webhookID := webhook["id"].(string)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		validateFunc   func(t *testing.T, body []byte)
	}{
		{
			name: "test webhook with valid event",
			request: map[string]interface{}{
				"event_type": "user.created",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				message, ok := response["message"].(string)
				if !ok || message == "" {
					t.Error("Expected success message in response")
				}
			},
		},
		{
			name: "test webhook with another valid event",
			request: map[string]interface{}{
				"event_type": "user.verified",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				parseJSONResponse(t, body, &response)

				message, ok := response["message"].(string)
				if !ok || message == "" {
					t.Error("Expected success message in response")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fmt.Sprintf("/api/protected/webhooks/%s/test", webhookID)
			resp, body := makeAuthenticatedRequest(t, http.MethodPost, path, tt.request, token)
			assertStatusCode(t, resp, tt.expectedStatus)

			if tt.validateFunc != nil {
				tt.validateFunc(t, body)
			}
		})
	}
}
