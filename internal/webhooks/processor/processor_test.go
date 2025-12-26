package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

// Test CreateWebhook

func TestCreateWebhook_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	params := CreateWebhookParams{
		AccountID:    accountID,
		CampaignID:   &campaignID,
		URL:          "https://example.com/webhook",
		Events:       []string{"user.created", "user.updated"},
		RetryEnabled: true,
		MaxRetries:   3,
	}

	expectedWebhook := store.Webhook{
		ID:           uuid.New(),
		AccountID:    accountID,
		CampaignID:   &campaignID,
		URL:          params.URL,
		Status:       "active",
		Events:       store.StringArray(params.Events),
		RetryEnabled: params.RetryEnabled,
		MaxRetries:   params.MaxRetries,
	}

	mockStore.EXPECT().CreateWebhook(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, p store.CreateWebhookParams) (store.Webhook, error) {
			if p.AccountID != accountID {
				t.Errorf("expected account ID %s, got %s", accountID, p.AccountID)
			}
			if p.URL != params.URL {
				t.Errorf("expected URL %s, got %s", params.URL, p.URL)
			}
			if len(p.Events) != 2 {
				t.Errorf("expected 2 events, got %d", len(p.Events))
			}
			if p.Secret == "" {
				t.Error("expected secret to be generated")
			}
			return expectedWebhook, nil
		})

	webhook, secret, err := processor.CreateWebhook(ctx, params)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if webhook.ID != expectedWebhook.ID {
		t.Errorf("expected webhook ID %s, got %s", expectedWebhook.ID, webhook.ID)
	}
	if secret == "" {
		t.Error("expected secret to be returned")
	}
}

func TestCreateWebhook_DefaultMaxRetries(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	accountID := uuid.New()

	params := CreateWebhookParams{
		AccountID:    accountID,
		URL:          "https://example.com/webhook",
		Events:       []string{"user.created"},
		RetryEnabled: true,
		MaxRetries:   0, // Should default to 5
	}

	mockStore.EXPECT().CreateWebhook(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, p store.CreateWebhookParams) (store.Webhook, error) {
			if p.MaxRetries != 5 {
				t.Errorf("expected max retries to default to 5, got %d", p.MaxRetries)
			}
			return store.Webhook{ID: uuid.New()}, nil
		})

	_, _, err := processor.CreateWebhook(ctx, params)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestCreateWebhook_EmptyEvents(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	accountID := uuid.New()

	params := CreateWebhookParams{
		AccountID:    accountID,
		URL:          "https://example.com/webhook",
		Events:       []string{},
		RetryEnabled: true,
	}

	_, _, err := processor.CreateWebhook(ctx, params)

	if err == nil {
		t.Error("expected error for empty events, got nil")
	}
}

func TestCreateWebhook_InvalidEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	accountID := uuid.New()

	params := CreateWebhookParams{
		AccountID:    accountID,
		URL:          "https://example.com/webhook",
		Events:       []string{"invalid.event"},
		RetryEnabled: true,
	}

	_, _, err := processor.CreateWebhook(ctx, params)

	if err == nil {
		t.Error("expected error for invalid event, got nil")
	}
}

func TestCreateWebhook_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	accountID := uuid.New()

	params := CreateWebhookParams{
		AccountID:    accountID,
		URL:          "https://example.com/webhook",
		Events:       []string{"user.created"},
		RetryEnabled: true,
	}

	storeErr := errors.New("database error")
	mockStore.EXPECT().CreateWebhook(gomock.Any(), gomock.Any()).
		Return(store.Webhook{}, storeErr)

	_, _, err := processor.CreateWebhook(ctx, params)

	if err == nil {
		t.Error("expected error, got nil")
	}
}

// Test UpdateWebhook

func TestUpdateWebhook_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()
	newURL := "https://example.com/new-webhook"
	newStatus := "paused"

	params := UpdateWebhookParams{
		URL:    &newURL,
		Events: []string{"user.created", "user.deleted"},
		Status: &newStatus,
	}

	expectedWebhook := store.Webhook{
		ID:     webhookID,
		URL:    newURL,
		Status: newStatus,
		Events: store.StringArray{"user.created", "user.deleted"},
	}

	mockStore.EXPECT().UpdateWebhook(gomock.Any(), webhookID, gomock.Any()).
		Return(expectedWebhook, nil)

	webhook, err := processor.UpdateWebhook(ctx, webhookID, params)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if webhook.URL != newURL {
		t.Errorf("expected URL %s, got %s", newURL, webhook.URL)
	}
	if webhook.Status != newStatus {
		t.Errorf("expected status %s, got %s", newStatus, webhook.Status)
	}
}

func TestUpdateWebhook_InvalidEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()

	params := UpdateWebhookParams{
		Events: []string{"invalid.event"},
	}

	_, err := processor.UpdateWebhook(ctx, webhookID, params)

	if err == nil {
		t.Error("expected error for invalid event, got nil")
	}
}

func TestUpdateWebhook_InvalidStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()
	invalidStatus := "invalid"

	params := UpdateWebhookParams{
		Status: &invalidStatus,
	}

	_, err := processor.UpdateWebhook(ctx, webhookID, params)

	if err == nil {
		t.Error("expected error for invalid status, got nil")
	}
}

func TestUpdateWebhook_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()
	newURL := "https://example.com/new-webhook"

	params := UpdateWebhookParams{
		URL: &newURL,
	}

	storeErr := errors.New("database error")
	mockStore.EXPECT().UpdateWebhook(gomock.Any(), webhookID, gomock.Any()).
		Return(store.Webhook{}, storeErr)

	_, err := processor.UpdateWebhook(ctx, webhookID, params)

	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestUpdateWebhook_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()
	newURL := "https://example.com/new-webhook"

	params := UpdateWebhookParams{
		URL: &newURL,
	}

	mockStore.EXPECT().UpdateWebhook(gomock.Any(), webhookID, gomock.Any()).
		Return(store.Webhook{}, store.ErrNotFound)

	_, err := processor.UpdateWebhook(ctx, webhookID, params)

	if err == nil {
		t.Error("expected error, got nil")
	}
}

// Test GetWebhook

func TestGetWebhook_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()
	accountID := uuid.New()

	expectedWebhook := store.Webhook{
		ID:        webhookID,
		AccountID: accountID,
		URL:       "https://example.com/webhook",
		Status:    "active",
	}

	mockStore.EXPECT().GetWebhookByID(gomock.Any(), webhookID).
		Return(expectedWebhook, nil)

	webhook, err := processor.GetWebhook(ctx, webhookID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if webhook.ID != webhookID {
		t.Errorf("expected webhook ID %s, got %s", webhookID, webhook.ID)
	}
}

func TestGetWebhook_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()

	mockStore.EXPECT().GetWebhookByID(gomock.Any(), webhookID).
		Return(store.Webhook{}, store.ErrNotFound)

	_, err := processor.GetWebhook(ctx, webhookID)

	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGetWebhook_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()

	storeErr := errors.New("database error")
	mockStore.EXPECT().GetWebhookByID(gomock.Any(), webhookID).
		Return(store.Webhook{}, storeErr)

	_, err := processor.GetWebhook(ctx, webhookID)

	if err == nil {
		t.Error("expected error, got nil")
	}
}

// Test GetWebhooksByAccount

func TestGetWebhooksByAccount_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	accountID := uuid.New()

	expectedWebhooks := []store.Webhook{
		{ID: uuid.New(), AccountID: accountID, URL: "https://example.com/webhook1"},
		{ID: uuid.New(), AccountID: accountID, URL: "https://example.com/webhook2"},
	}

	mockStore.EXPECT().GetWebhooksByAccount(gomock.Any(), accountID).
		Return(expectedWebhooks, nil)

	webhooks, err := processor.GetWebhooksByAccount(ctx, accountID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(webhooks) != 2 {
		t.Errorf("expected 2 webhooks, got %d", len(webhooks))
	}
}

func TestGetWebhooksByAccount_EmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	accountID := uuid.New()

	mockStore.EXPECT().GetWebhooksByAccount(gomock.Any(), accountID).
		Return(nil, nil)

	webhooks, err := processor.GetWebhooksByAccount(ctx, accountID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if webhooks == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(webhooks) != 0 {
		t.Errorf("expected 0 webhooks, got %d", len(webhooks))
	}
}

func TestGetWebhooksByAccount_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	accountID := uuid.New()

	storeErr := errors.New("database error")
	mockStore.EXPECT().GetWebhooksByAccount(gomock.Any(), accountID).
		Return(nil, storeErr)

	_, err := processor.GetWebhooksByAccount(ctx, accountID)

	if err == nil {
		t.Error("expected error, got nil")
	}
}

// Test GetWebhooksByCampaign

func TestGetWebhooksByCampaign_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	campaignID := uuid.New()

	expectedWebhooks := []store.Webhook{
		{ID: uuid.New(), CampaignID: &campaignID, URL: "https://example.com/webhook1"},
	}

	mockStore.EXPECT().GetWebhooksByCampaign(gomock.Any(), campaignID).
		Return(expectedWebhooks, nil)

	webhooks, err := processor.GetWebhooksByCampaign(ctx, campaignID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(webhooks) != 1 {
		t.Errorf("expected 1 webhook, got %d", len(webhooks))
	}
}

func TestGetWebhooksByCampaign_EmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	campaignID := uuid.New()

	mockStore.EXPECT().GetWebhooksByCampaign(gomock.Any(), campaignID).
		Return(nil, nil)

	webhooks, err := processor.GetWebhooksByCampaign(ctx, campaignID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if webhooks == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(webhooks) != 0 {
		t.Errorf("expected 0 webhooks, got %d", len(webhooks))
	}
}

func TestGetWebhooksByCampaign_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	campaignID := uuid.New()

	storeErr := errors.New("database error")
	mockStore.EXPECT().GetWebhooksByCampaign(gomock.Any(), campaignID).
		Return(nil, storeErr)

	_, err := processor.GetWebhooksByCampaign(ctx, campaignID)

	if err == nil {
		t.Error("expected error, got nil")
	}
}

// Test DeleteWebhook

func TestDeleteWebhook_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()

	mockStore.EXPECT().DeleteWebhook(gomock.Any(), webhookID).
		Return(nil)

	err := processor.DeleteWebhook(ctx, webhookID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDeleteWebhook_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()

	mockStore.EXPECT().DeleteWebhook(gomock.Any(), webhookID).
		Return(store.ErrNotFound)

	err := processor.DeleteWebhook(ctx, webhookID)

	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestDeleteWebhook_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()

	storeErr := errors.New("database error")
	mockStore.EXPECT().DeleteWebhook(gomock.Any(), webhookID).
		Return(storeErr)

	err := processor.DeleteWebhook(ctx, webhookID)

	if err == nil {
		t.Error("expected error, got nil")
	}
}

// Test GetWebhookDeliveries

func TestGetWebhookDeliveries_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()
	limit := 10
	offset := 0

	expectedDeliveries := []store.WebhookDelivery{
		{ID: uuid.New(), WebhookID: webhookID, EventType: "user.created", Status: "delivered"},
		{ID: uuid.New(), WebhookID: webhookID, EventType: "user.updated", Status: "delivered"},
	}

	mockStore.EXPECT().GetWebhookDeliveriesByWebhook(gomock.Any(), webhookID, limit, offset).
		Return(expectedDeliveries, nil)

	deliveries, err := processor.GetWebhookDeliveries(ctx, webhookID, limit, offset)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(deliveries) != 2 {
		t.Errorf("expected 2 deliveries, got %d", len(deliveries))
	}
}

func TestGetWebhookDeliveries_EmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()
	limit := 10
	offset := 0

	mockStore.EXPECT().GetWebhookDeliveriesByWebhook(gomock.Any(), webhookID, limit, offset).
		Return(nil, nil)

	deliveries, err := processor.GetWebhookDeliveries(ctx, webhookID, limit, offset)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if deliveries == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(deliveries) != 0 {
		t.Errorf("expected 0 deliveries, got %d", len(deliveries))
	}
}

func TestGetWebhookDeliveries_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()
	limit := 10
	offset := 0

	storeErr := errors.New("database error")
	mockStore.EXPECT().GetWebhookDeliveriesByWebhook(gomock.Any(), webhookID, limit, offset).
		Return(nil, storeErr)

	_, err := processor.GetWebhookDeliveries(ctx, webhookID, limit, offset)

	if err == nil {
		t.Error("expected error, got nil")
	}
}

// Test TestWebhook

func TestTestWebhook_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()

	mockService.EXPECT().TestWebhook(gomock.Any(), webhookID).
		Return(nil)

	err := processor.TestWebhook(ctx, webhookID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestTestWebhook_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()

	serviceErr := errors.New("webhook service error")
	mockService.EXPECT().TestWebhook(gomock.Any(), webhookID).
		Return(serviceErr)

	err := processor.TestWebhook(ctx, webhookID)

	if err == nil {
		t.Error("expected error, got nil")
	}
}

// Test valid events

func TestCreateWebhook_AllValidEvents(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	accountID := uuid.New()

	validEvents := []string{
		"user.created",
		"user.updated",
		"user.verified",
		"user.deleted",
		"user.position_changed",
		"user.converted",
		"referral.created",
		"referral.verified",
		"referral.converted",
		"reward.earned",
		"reward.delivered",
		"reward.redeemed",
		"campaign.milestone",
		"campaign.launched",
		"campaign.completed",
		"email.sent",
		"email.delivered",
		"email.opened",
		"email.clicked",
		"email.bounced",
	}

	for _, event := range validEvents {
		params := CreateWebhookParams{
			AccountID: accountID,
			URL:       "https://example.com/webhook",
			Events:    []string{event},
		}

		mockStore.EXPECT().CreateWebhook(gomock.Any(), gomock.Any()).
			Return(store.Webhook{ID: uuid.New()}, nil)

		_, _, err := processor.CreateWebhook(ctx, params)

		if err != nil {
			t.Errorf("expected event %s to be valid, got error: %v", event, err)
		}
	}
}

// Test valid statuses

func TestUpdateWebhook_AllValidStatuses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockWebhookStore(ctrl)
	mockService := NewMockWebhookService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger, mockService)

	ctx := context.Background()
	webhookID := uuid.New()

	validStatuses := []string{"active", "paused", "failed"}

	for _, status := range validStatuses {
		statusCopy := status
		params := UpdateWebhookParams{
			Status: &statusCopy,
		}

		mockStore.EXPECT().UpdateWebhook(gomock.Any(), webhookID, gomock.Any()).
			Return(store.Webhook{ID: webhookID, Status: status}, nil)

		webhook, err := processor.UpdateWebhook(ctx, webhookID, params)

		if err != nil {
			t.Errorf("expected status %s to be valid, got error: %v", status, err)
		}
		if webhook.Status != status {
			t.Errorf("expected status %s, got %s", status, webhook.Status)
		}
	}
}
