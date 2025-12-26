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
	"go.uber.org/mock/gomock"
)

func TestGenerateSignature(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	logger := observability.NewLogger()
	service := New(mockStore, logger)

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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	logger := observability.NewLogger()
	service := New(mockStore, logger)

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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	logger := observability.NewLogger()
	service := New(mockStore, logger)

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

func TestDispatchEvent_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a test HTTP server to receive webhooks
	receivedWebhooks := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedWebhooks++
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Expected Content-Type: application/json")
		}
		if r.Header.Get("X-Webhook-Signature") == "" {
			t.Error("Expected X-Webhook-Signature header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	mockStore := NewMockWebhookStore(ctrl)
	logger := observability.NewLogger()
	service := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	webhookID := uuid.New()
	deliveryID := uuid.New()

	webhook := store.Webhook{
		ID:           webhookID,
		AccountID:    accountID,
		URL:          testServer.URL,
		Secret:       "test-secret",
		Events:       []string{"user.created", "user.verified"},
		Status:       "active",
		RetryEnabled: true,
		MaxRetries:   5,
	}

	// Setup mock expectations
	mockStore.EXPECT().GetWebhooksByAccount(gomock.Any(), accountID).Return([]store.Webhook{webhook}, nil)
	mockStore.EXPECT().CreateWebhookDelivery(gomock.Any(), gomock.Any()).Return(store.WebhookDelivery{
		ID:        deliveryID,
		WebhookID: webhookID,
		EventType: "user.created",
		Status:    "pending",
	}, nil)
	mockStore.EXPECT().UpdateWebhookDeliveryStatus(gomock.Any(), deliveryID, gomock.Any()).Return(nil)
	mockStore.EXPECT().IncrementWebhookSent(gomock.Any(), webhookID).Return(nil)

	eventType := "user.created"
	data := map[string]interface{}{
		"user_id": "123",
		"email":   "test@example.com",
	}

	err := service.DispatchEvent(ctx, accountID, nil, eventType, data)
	if err != nil {
		t.Errorf("DispatchEvent failed: %v", err)
	}

	if receivedWebhooks != 1 {
		t.Errorf("Expected 1 webhook to be received, got %d", receivedWebhooks)
	}
}

func TestDispatchEvent_WebhookNotSubscribed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a test HTTP server - should NOT receive any webhooks
	receivedWebhooks := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedWebhooks++
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	mockStore := NewMockWebhookStore(ctrl)
	logger := observability.NewLogger()
	service := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	webhookID := uuid.New()

	webhook := store.Webhook{
		ID:           webhookID,
		AccountID:    accountID,
		URL:          testServer.URL,
		Secret:       "test-secret",
		Events:       []string{"user.deleted"}, // subscribed to different event
		Status:       "active",
		RetryEnabled: true,
		MaxRetries:   5,
	}

	mockStore.EXPECT().GetWebhooksByAccount(gomock.Any(), accountID).Return([]store.Webhook{webhook}, nil)

	// Dispatch an event the webhook is NOT subscribed to
	err := service.DispatchEvent(ctx, accountID, nil, "user.created", map[string]interface{}{"test": true})
	if err != nil {
		t.Errorf("DispatchEvent failed: %v", err)
	}

	// Webhook should NOT have been called
	if receivedWebhooks != 0 {
		t.Errorf("Expected 0 webhooks to be received (not subscribed), got %d", receivedWebhooks)
	}
}

func TestDispatchEvent_PausedWebhook(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a test HTTP server - should NOT receive any webhooks
	receivedWebhooks := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedWebhooks++
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	mockStore := NewMockWebhookStore(ctrl)
	logger := observability.NewLogger()
	service := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	webhookID := uuid.New()

	webhook := store.Webhook{
		ID:           webhookID,
		AccountID:    accountID,
		URL:          testServer.URL,
		Secret:       "test-secret",
		Events:       []string{"user.created"},
		Status:       "paused", // Webhook is paused
		RetryEnabled: true,
		MaxRetries:   5,
	}

	mockStore.EXPECT().GetWebhooksByAccount(gomock.Any(), accountID).Return([]store.Webhook{webhook}, nil)

	err := service.DispatchEvent(ctx, accountID, nil, "user.created", map[string]interface{}{"test": true})
	if err != nil {
		t.Errorf("DispatchEvent failed: %v", err)
	}

	// Webhook should NOT have been called (paused)
	if receivedWebhooks != 0 {
		t.Errorf("Expected 0 webhooks to be received (paused), got %d", receivedWebhooks)
	}
}

func TestDispatchEvent_CampaignSpecific(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a test HTTP server
	receivedWebhooks := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedWebhooks++
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	mockStore := NewMockWebhookStore(ctrl)
	logger := observability.NewLogger()
	service := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()
	webhookID := uuid.New()
	deliveryID := uuid.New()

	webhook := store.Webhook{
		ID:           webhookID,
		AccountID:    accountID,
		CampaignID:   &campaignID,
		URL:          testServer.URL,
		Secret:       "test-secret",
		Events:       []string{"user.created"},
		Status:       "active",
		RetryEnabled: true,
		MaxRetries:   5,
	}

	// First call: dispatch event for the correct campaign
	mockStore.EXPECT().GetWebhooksByAccount(gomock.Any(), accountID).Return([]store.Webhook{webhook}, nil)
	mockStore.EXPECT().CreateWebhookDelivery(gomock.Any(), gomock.Any()).Return(store.WebhookDelivery{
		ID:        deliveryID,
		WebhookID: webhookID,
		EventType: "user.created",
		Status:    "pending",
	}, nil)
	mockStore.EXPECT().UpdateWebhookDeliveryStatus(gomock.Any(), deliveryID, gomock.Any()).Return(nil)
	mockStore.EXPECT().IncrementWebhookSent(gomock.Any(), webhookID).Return(nil)

	err := service.DispatchEvent(ctx, accountID, &campaignID, "user.created", map[string]interface{}{"test": true})
	if err != nil {
		t.Errorf("DispatchEvent failed: %v", err)
	}

	if receivedWebhooks != 1 {
		t.Errorf("Expected 1 webhook to be received, got %d", receivedWebhooks)
	}

	// Second call: dispatch event for a different campaign - should NOT trigger webhook
	receivedWebhooks = 0
	otherCampaignID := uuid.New()
	mockStore.EXPECT().GetWebhooksByAccount(gomock.Any(), accountID).Return([]store.Webhook{webhook}, nil)

	err = service.DispatchEvent(ctx, accountID, &otherCampaignID, "user.created", map[string]interface{}{"test": true})
	if err != nil {
		t.Errorf("DispatchEvent failed: %v", err)
	}

	if receivedWebhooks != 0 {
		t.Errorf("Expected 0 webhooks for different campaign, got %d", receivedWebhooks)
	}
}

func TestTestWebhook_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a test HTTP server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	mockStore := NewMockWebhookStore(ctrl)
	logger := observability.NewLogger()
	service := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	webhookID := uuid.New()
	deliveryID := uuid.New()

	webhook := store.Webhook{
		ID:           webhookID,
		AccountID:    accountID,
		URL:          testServer.URL,
		Secret:       "test-secret",
		Events:       []string{"webhook.test"},
		Status:       "active",
		RetryEnabled: true,
		MaxRetries:   5,
	}

	mockStore.EXPECT().GetWebhookByID(gomock.Any(), webhookID).Return(webhook, nil)
	mockStore.EXPECT().CreateWebhookDelivery(gomock.Any(), gomock.Any()).Return(store.WebhookDelivery{
		ID:        deliveryID,
		WebhookID: webhookID,
		EventType: "webhook.test",
		Status:    "pending",
	}, nil)
	mockStore.EXPECT().UpdateWebhookDeliveryStatus(gomock.Any(), deliveryID, gomock.Any()).Return(nil)
	mockStore.EXPECT().IncrementWebhookSent(gomock.Any(), webhookID).Return(nil)

	err := service.TestWebhook(ctx, webhookID)
	if err != nil {
		t.Errorf("TestWebhook failed: %v", err)
	}
}

func TestTestWebhook_WebhookNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	logger := observability.NewLogger()
	service := New(mockStore, logger)

	ctx := context.Background()
	webhookID := uuid.New()

	mockStore.EXPECT().GetWebhookByID(gomock.Any(), webhookID).Return(store.Webhook{}, store.ErrNotFound)

	err := service.TestWebhook(ctx, webhookID)
	if err == nil {
		t.Error("Expected error when webhook not found")
	}
}

func TestDispatchEvent_DeliveryFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a test HTTP server that returns 500
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer testServer.Close()

	mockStore := NewMockWebhookStore(ctrl)
	logger := observability.NewLogger()
	service := New(mockStore, logger)

	ctx := context.Background()
	accountID := uuid.New()
	webhookID := uuid.New()
	deliveryID := uuid.New()

	webhook := store.Webhook{
		ID:           webhookID,
		AccountID:    accountID,
		URL:          testServer.URL,
		Secret:       "test-secret",
		Events:       []string{"user.created"},
		Status:       "active",
		RetryEnabled: true,
		MaxRetries:   5,
	}

	mockStore.EXPECT().GetWebhooksByAccount(gomock.Any(), accountID).Return([]store.Webhook{webhook}, nil)
	mockStore.EXPECT().CreateWebhookDelivery(gomock.Any(), gomock.Any()).Return(store.WebhookDelivery{
		ID:            deliveryID,
		WebhookID:     webhookID,
		EventType:     "user.created",
		Status:        "pending",
		AttemptNumber: 1,
	}, nil)
	mockStore.EXPECT().UpdateWebhookDeliveryStatus(gomock.Any(), deliveryID, gomock.Any()).Return(nil)
	mockStore.EXPECT().IncrementDeliveryAttempt(gomock.Any(), deliveryID, gomock.Any()).Return(nil)

	err := service.DispatchEvent(ctx, accountID, nil, "user.created", map[string]interface{}{"test": true})
	// DispatchEvent continues even if individual webhooks fail
	if err != nil {
		t.Errorf("DispatchEvent should not fail: %v", err)
	}
}
