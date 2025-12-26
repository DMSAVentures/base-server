package service

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

// createTestAccount creates an account for testing
func createTestAccount(t *testing.T, testDB *store.TestDB) store.Account {
	t.Helper()
	ctx := context.Background()

	// Create a user first
	var user store.User
	err := testDB.GetDB().GetContext(ctx, &user,
		`INSERT INTO users (first_name, last_name) VALUES ($1, $2) RETURNING id, first_name, last_name`,
		"Test", "User")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create account
	account, err := testDB.Store.CreateAccount(ctx, store.CreateAccountParams{
		Name:        "Test Account",
		Slug:        "test-account-" + uuid.New().String()[:8],
		OwnerUserID: user.ID,
		Plan:        "pro",
	})
	if err != nil {
		t.Fatalf("failed to create test account: %v", err)
	}
	return account
}

// createTestCampaign creates a campaign for testing
func createTestCampaign(t *testing.T, testDB *store.TestDB, accountID uuid.UUID) store.Campaign {
	t.Helper()
	ctx := context.Background()

	campaign, err := testDB.Store.CreateCampaign(ctx, store.CreateCampaignParams{
		AccountID: accountID,
		Name:      "Test Campaign",
		Slug:      "test-campaign-" + uuid.New().String()[:8],
		Type:      "waitlist",
	})
	if err != nil {
		t.Fatalf("failed to create test campaign: %v", err)
	}
	return campaign
}

func TestGenerateSignature(t *testing.T) {
	testDB := store.SetupTestDB(t, store.TestDBTypePostgres)
	defer testDB.Close()

	logger := observability.NewLogger()
	service := New(&testDB.Store, logger)

	secret := "test-secret"
	payload := []byte(`{"test":"data"}`)
	timestamp := int64(1234567890)

	signature := service.generateSignature(secret, payload, timestamp)

	// Verify signature format
	if len(signature) == 0 {
		t.Error("Signature should not be empty")
	}

	// Should start with "t=" followed by timestamp
	expected := "t=1234567890,v1="
	if len(signature) < len(expected) {
		t.Errorf("Signature too short: %s", signature)
	}
	if signature[:len(expected)] != expected {
		t.Errorf("Signature should start with %s, got: %s", expected, signature[:len(expected)])
	}
}

func TestCalculateNextRetry(t *testing.T) {
	testDB := store.SetupTestDB(t, store.TestDBTypePostgres)
	defer testDB.Close()

	logger := observability.NewLogger()
	service := New(&testDB.Store, logger)

	tests := []struct {
		attemptNumber int
		expectedDelay time.Duration
	}{
		{1, 2 * time.Second},
		{2, 10 * time.Second},
		{3, 20 * time.Second},
		{4, 40 * time.Second},
		{5, 1 * time.Minute},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			before := time.Now()
			nextRetry := service.calculateNextRetry(tt.attemptNumber)
			delay := nextRetry.Sub(before)

			// Allow some tolerance (100ms) for test execution time
			if delay < tt.expectedDelay-100*time.Millisecond || delay > tt.expectedDelay+100*time.Millisecond {
				t.Errorf("For attempt %d, expected delay ~%v, got %v", tt.attemptNumber, tt.expectedDelay, delay)
			}
		})
	}
}

func TestSubscribesToEvent(t *testing.T) {
	testDB := store.SetupTestDB(t, store.TestDBTypePostgres)
	defer testDB.Close()

	logger := observability.NewLogger()
	service := New(&testDB.Store, logger)

	subscribedEvents := []string{"user.created", "user.verified", "referral.created"}

	tests := []struct {
		eventType string
		expected  bool
	}{
		{"user.created", true},
		{"user.verified", true},
		{"referral.created", true},
		{"user.deleted", false},
		{"unknown.event", false},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			result := service.subscribesToEvent(subscribedEvents, tt.eventType)
			if result != tt.expected {
				t.Errorf("subscribesToEvent(%v, %s) = %v, want %v", subscribedEvents, tt.eventType, result, tt.expected)
			}
		})
	}
}

func TestDispatchEvent(t *testing.T) {
	testDB := store.SetupTestDB(t, store.TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	// Create a test HTTP server to receive webhooks
	receivedWebhooks := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedWebhooks++
		// Verify headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Expected Content-Type: application/json")
		}
		if r.Header.Get("X-Webhook-Signature") == "" {
			t.Error("Expected X-Webhook-Signature header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Create test account
	account := createTestAccount(t, testDB)

	// Create a webhook pointing to our test server
	ctx := context.Background()
	webhook, err := testDB.Store.CreateWebhook(ctx, store.CreateWebhookParams{
		AccountID:    account.ID,
		CampaignID:   nil,
		URL:          testServer.URL,
		Secret:       "test-secret",
		Events:       []string{"user.created", "user.verified"},
		RetryEnabled: true,
		MaxRetries:   5,
	})
	if err != nil {
		t.Fatalf("Failed to create webhook: %v", err)
	}

	logger := observability.NewLogger()
	service := New(&testDB.Store, logger)

	// Test dispatch
	eventType := "user.created"
	data := map[string]interface{}{
		"user_id": "123",
		"email":   "test@example.com",
	}

	err = service.DispatchEvent(ctx, account.ID, nil, eventType, data)
	if err != nil {
		t.Errorf("DispatchEvent failed: %v", err)
	}

	if receivedWebhooks != 1 {
		t.Errorf("Expected 1 webhook to be received, got %d", receivedWebhooks)
	}

	// Verify delivery was recorded in database
	deliveries, err := testDB.Store.GetWebhookDeliveriesByWebhook(ctx, webhook.ID, 10, 0)
	if err != nil {
		t.Fatalf("Failed to get deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Errorf("Expected 1 delivery record, got %d", len(deliveries))
	}
	if len(deliveries) > 0 && deliveries[0].Status != "success" {
		t.Errorf("Expected delivery status 'success', got '%s'", deliveries[0].Status)
	}
}

func TestTestWebhook(t *testing.T) {
	testDB := store.SetupTestDB(t, store.TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	// Create a test HTTP server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Create test account
	account := createTestAccount(t, testDB)

	ctx := context.Background()
	webhook, err := testDB.Store.CreateWebhook(ctx, store.CreateWebhookParams{
		AccountID:    account.ID,
		CampaignID:   nil,
		URL:          testServer.URL,
		Secret:       "test-secret",
		Events:       []string{"webhook.test"},
		RetryEnabled: true,
		MaxRetries:   5,
	})
	if err != nil {
		t.Fatalf("Failed to create webhook: %v", err)
	}

	logger := observability.NewLogger()
	service := New(&testDB.Store, logger)

	err = service.TestWebhook(ctx, webhook.ID)
	if err != nil {
		t.Errorf("TestWebhook failed: %v", err)
	}
}

func TestDispatchEvent_WebhookNotSubscribed(t *testing.T) {
	testDB := store.SetupTestDB(t, store.TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	// Create a test HTTP server - should NOT receive any webhooks
	receivedWebhooks := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedWebhooks++
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Create test account
	account := createTestAccount(t, testDB)

	ctx := context.Background()
	_, err := testDB.Store.CreateWebhook(ctx, store.CreateWebhookParams{
		AccountID:    account.ID,
		CampaignID:   nil,
		URL:          testServer.URL,
		Secret:       "test-secret",
		Events:       []string{"user.deleted"}, // subscribed to different event
		RetryEnabled: true,
		MaxRetries:   5,
	})
	if err != nil {
		t.Fatalf("Failed to create webhook: %v", err)
	}

	logger := observability.NewLogger()
	service := New(&testDB.Store, logger)

	// Dispatch an event the webhook is NOT subscribed to
	err = service.DispatchEvent(ctx, account.ID, nil, "user.created", map[string]interface{}{"test": true})
	if err != nil {
		t.Errorf("DispatchEvent failed: %v", err)
	}

	// Webhook should NOT have been called
	if receivedWebhooks != 0 {
		t.Errorf("Expected 0 webhooks to be received (not subscribed), got %d", receivedWebhooks)
	}
}

func TestDispatchEvent_PausedWebhook(t *testing.T) {
	testDB := store.SetupTestDB(t, store.TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	// Create a test HTTP server - should NOT receive any webhooks
	receivedWebhooks := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedWebhooks++
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Create test account
	account := createTestAccount(t, testDB)

	ctx := context.Background()
	// Create webhook and then update it to paused
	webhook, err := testDB.Store.CreateWebhook(ctx, store.CreateWebhookParams{
		AccountID:    account.ID,
		CampaignID:   nil,
		URL:          testServer.URL,
		Secret:       "test-secret",
		Events:       []string{"user.created"},
		RetryEnabled: true,
		MaxRetries:   5,
	})
	if err != nil {
		t.Fatalf("Failed to create webhook: %v", err)
	}

	// Update webhook to paused status
	pausedStatus := "paused"
	_, err = testDB.Store.UpdateWebhook(ctx, webhook.ID, store.UpdateWebhookParams{
		Status: &pausedStatus,
	})
	if err != nil {
		t.Fatalf("Failed to pause webhook: %v", err)
	}

	logger := observability.NewLogger()
	service := New(&testDB.Store, logger)

	err = service.DispatchEvent(ctx, account.ID, nil, "user.created", map[string]interface{}{"test": true})
	if err != nil {
		t.Errorf("DispatchEvent failed: %v", err)
	}

	// Webhook should NOT have been called (paused)
	if receivedWebhooks != 0 {
		t.Errorf("Expected 0 webhooks to be received (paused), got %d", receivedWebhooks)
	}
}

func TestDispatchEvent_CampaignSpecific(t *testing.T) {
	testDB := store.SetupTestDB(t, store.TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	// Create a test HTTP server
	receivedWebhooks := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedWebhooks++
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Create test account and campaign
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID)

	ctx := context.Background()

	// Create a webhook for a specific campaign
	_, err := testDB.Store.CreateWebhook(ctx, store.CreateWebhookParams{
		AccountID:    account.ID,
		CampaignID:   &campaign.ID,
		URL:          testServer.URL,
		Secret:       "test-secret",
		Events:       []string{"user.created"},
		RetryEnabled: true,
		MaxRetries:   5,
	})
	if err != nil {
		t.Fatalf("Failed to create webhook: %v", err)
	}

	logger := observability.NewLogger()
	service := New(&testDB.Store, logger)

	// Dispatch event for the correct campaign
	err = service.DispatchEvent(ctx, account.ID, &campaign.ID, "user.created", map[string]interface{}{"test": true})
	if err != nil {
		t.Errorf("DispatchEvent failed: %v", err)
	}

	if receivedWebhooks != 1 {
		t.Errorf("Expected 1 webhook to be received, got %d", receivedWebhooks)
	}

	// Dispatch event for a different campaign - should NOT trigger webhook
	receivedWebhooks = 0
	otherCampaignID := uuid.New()
	err = service.DispatchEvent(ctx, account.ID, &otherCampaignID, "user.created", map[string]interface{}{"test": true})
	if err != nil {
		t.Errorf("DispatchEvent failed: %v", err)
	}

	if receivedWebhooks != 0 {
		t.Errorf("Expected 0 webhooks for different campaign, got %d", receivedWebhooks)
	}
}
