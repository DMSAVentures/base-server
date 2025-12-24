package service

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// WebhookStore defines the interface for webhook-related database operations
type WebhookStore interface {
	GetWebhooksByAccount(ctx context.Context, accountID uuid.UUID) ([]store.Webhook, error)
	CreateWebhookDelivery(ctx context.Context, params store.CreateWebhookDeliveryParams) (store.WebhookDelivery, error)
	UpdateWebhookDeliveryStatus(ctx context.Context, deliveryID uuid.UUID, params store.UpdateWebhookDeliveryStatusParams) error
	IncrementWebhookSent(ctx context.Context, webhookID uuid.UUID) error
	IncrementWebhookFailed(ctx context.Context, webhookID uuid.UUID) error
	IncrementDeliveryAttempt(ctx context.Context, deliveryID uuid.UUID, nextRetryAt *time.Time) error
	GetWebhookByID(ctx context.Context, webhookID uuid.UUID) (store.Webhook, error)
	GetPendingWebhookDeliveries(ctx context.Context, limit int, maxAttempt int) ([]store.WebhookDelivery, error)
}

// WebhookService handles webhook delivery operations
type WebhookService struct {
	store      WebhookStore
	logger     *observability.Logger
	httpClient *http.Client
}

// New creates a new WebhookService
func New(store WebhookStore, logger *observability.Logger) *WebhookService {
	return &WebhookService{
		store:  store,
		logger: logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// WebhookPayload represents the standard webhook payload structure
type WebhookPayload struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	CreatedAt string                 `json:"created_at"`
	Data      map[string]interface{} `json:"data"`
	AccountID string                 `json:"account_id"`
}

// DispatchEvent dispatches a webhook event to all subscribed webhooks
func (s *WebhookService) DispatchEvent(ctx context.Context, accountID uuid.UUID, campaignID *uuid.UUID, eventType string, data map[string]interface{}) error {
	// Get all webhooks for the account
	webhooks, err := s.store.GetWebhooksByAccount(ctx, accountID)
	if err != nil {
		s.logger.Error(ctx, "failed to get webhooks", err)
		return fmt.Errorf("failed to get webhooks: %w", err)
	}

	// Filter webhooks by campaign if specified
	var relevantWebhooks []store.Webhook
	for _, webhook := range webhooks {
		// Skip deleted or paused webhooks
		if webhook.DeletedAt != nil || webhook.Status != "active" {
			continue
		}

		// Check if webhook is for this campaign or account-level
		if campaignID != nil && webhook.CampaignID != nil && *webhook.CampaignID != *campaignID {
			continue
		}

		// Check if webhook subscribes to this event type
		if !s.subscribesToEvent(webhook.Events, eventType) {
			continue
		}

		relevantWebhooks = append(relevantWebhooks, webhook)
	}

	// Create webhook payload
	payload := WebhookPayload{
		ID:        uuid.New().String(),
		Type:      eventType,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
		AccountID: accountID.String(),
	}

	// Send to each webhook
	for _, webhook := range relevantWebhooks {
		err := s.sendWebhook(ctx, webhook, payload)
		if err != nil {
			s.logger.Error(ctx, fmt.Sprintf("failed to send webhook to %s", webhook.URL), err)
			// Continue sending to other webhooks even if one fails
		}
	}

	return nil
}

// subscribesToEvent checks if a webhook subscribes to a specific event type
func (s *WebhookService) subscribesToEvent(subscribedEvents []string, eventType string) bool {
	for _, event := range subscribedEvents {
		if event == eventType {
			return true
		}
	}
	return false
}

// sendWebhook sends a webhook to a specific endpoint
func (s *WebhookService) sendWebhook(ctx context.Context, webhook store.Webhook, payload WebhookPayload) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "webhook_id", Value: webhook.ID},
		observability.Field{Key: "event_type", Value: payload.Type},
	)

	// Serialize payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error(ctx, "failed to marshal payload", err)
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create webhook delivery record
	delivery, err := s.store.CreateWebhookDelivery(ctx, store.CreateWebhookDeliveryParams{
		WebhookID:   webhook.ID,
		EventType:   payload.Type,
		Payload:     store.JSONB(payload.Data),
		NextRetryAt: nil, // Will be set on failure
	})
	if err != nil {
		s.logger.Error(ctx, "failed to create webhook delivery", err)
		return fmt.Errorf("failed to create webhook delivery: %w", err)
	}

	ctx = observability.WithFields(ctx, observability.Field{Key: "delivery_id", Value: delivery.ID})

	// Attempt delivery
	success, responseStatus, responseBody, durationMs, err := s.deliverWebhook(ctx, webhook, payloadBytes)

	if success {
		// Update delivery status as success
		err = s.store.UpdateWebhookDeliveryStatus(ctx, delivery.ID, store.UpdateWebhookDeliveryStatusParams{
			Status:         "success",
			ResponseStatus: &responseStatus,
			ResponseBody:   &responseBody,
			DurationMs:     &durationMs,
			ErrorMessage:   nil,
		})
		if err != nil {
			s.logger.Error(ctx, "failed to update delivery status", err)
		}

		// Increment webhook sent counter
		err = s.store.IncrementWebhookSent(ctx, webhook.ID)
		if err != nil {
			s.logger.Error(ctx, "failed to increment webhook sent", err)
		}

		s.logger.Info(ctx, "webhook delivered successfully")
		return nil
	}

	// Handle failure
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}

	// Calculate next retry time
	var nextRetryAt *time.Time
	if webhook.RetryEnabled && delivery.AttemptNumber < webhook.MaxRetries {
		nextRetry := s.calculateNextRetry(delivery.AttemptNumber)
		nextRetryAt = &nextRetry
	}

	// Update delivery status as failed
	err = s.store.UpdateWebhookDeliveryStatus(ctx, delivery.ID, store.UpdateWebhookDeliveryStatusParams{
		Status:         "failed",
		ResponseStatus: &responseStatus,
		ResponseBody:   &responseBody,
		DurationMs:     &durationMs,
		ErrorMessage:   &errorMessage,
	})
	if err != nil {
		s.logger.Error(ctx, "failed to update delivery status", err)
	}

	// Set next retry time if applicable
	if nextRetryAt != nil {
		err = s.store.IncrementDeliveryAttempt(ctx, delivery.ID, nextRetryAt)
		if err != nil {
			s.logger.Error(ctx, "failed to set next retry time", err)
		}
		s.logger.Info(ctx, fmt.Sprintf("webhook delivery failed, will retry at %s", nextRetryAt.Format(time.RFC3339)))
	} else {
		// Increment webhook failed counter (no more retries)
		err = s.store.IncrementWebhookFailed(ctx, webhook.ID)
		if err != nil {
			s.logger.Error(ctx, "failed to increment webhook failed", err)
		}
		s.logger.Error(ctx, "webhook delivery failed, no more retries", fmt.Errorf("max retries reached"))
	}

	return fmt.Errorf("webhook delivery failed: %s", errorMessage)
}

// deliverWebhook performs the actual HTTP request to deliver the webhook
func (s *WebhookService) deliverWebhook(ctx context.Context, webhook store.Webhook, payloadBytes []byte) (success bool, responseStatus int, responseBody string, durationMs int, err error) {
	startTime := time.Now()

	// Generate HMAC signature
	signature := s.generateSignature(webhook.Secret, payloadBytes, startTime.Unix())

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", webhook.URL, io.NopCloser(bytes.NewReader(payloadBytes)))
	if err != nil {
		return false, 0, "", 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("User-Agent", "Waitlist-Platform-Webhook/1.0")

	// Send request
	resp, err := s.httpClient.Do(req)
	durationMs = int(time.Since(startTime).Milliseconds())

	if err != nil {
		return false, 0, "", durationMs, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	responseStatus = resp.StatusCode

	// Read response body (limit to 10KB)
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10240))
	if err != nil {
		s.logger.Warn(ctx, "failed to read response body")
		responseBody = ""
	} else {
		responseBody = string(bodyBytes)
	}

	// Check if request was successful (2xx status code)
	if responseStatus >= 200 && responseStatus < 300 {
		return true, responseStatus, responseBody, durationMs, nil
	}

	return false, responseStatus, responseBody, durationMs, fmt.Errorf("received non-2xx status code: %d", responseStatus)
}

// generateSignature generates an HMAC signature for the webhook payload
func (s *WebhookService) generateSignature(secret string, payload []byte, timestamp int64) string {
	// Format: t=<timestamp>,v1=<signature>
	signedPayload := fmt.Sprintf("%d.%s", timestamp, string(payload))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	signature := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("t=%d,v1=%s", timestamp, signature)
}

// calculateNextRetry calculates the next retry time based on attempt number
// Retry schedule: 2s, 10s, 1min, 10min
func (s *WebhookService) calculateNextRetry(attemptNumber int) time.Time {
	var delay time.Duration

	switch attemptNumber {
	case 1:
		delay = 2 * time.Second
	case 2:
		delay = 10 * time.Second
	case 3:
		delay = 20 * time.Second
	case 4:
		delay = 40 * time.Second
	case 5:
		delay = 1 * time.Minute
	default:
		delay = 5 * time.Minute
	}

	return time.Now().Add(delay)
}

// RetryFailedDeliveries processes failed webhook deliveries that are ready for retry
func (s *WebhookService) RetryFailedDeliveries(ctx context.Context, limit int) error {
	// Get pending deliveries
	deliveries, err := s.store.GetPendingWebhookDeliveries(ctx, limit, 5)
	if err != nil {
		s.logger.Error(ctx, "failed to get pending deliveries", err)
		return fmt.Errorf("failed to get pending deliveries: %w", err)
	}

	s.logger.Info(ctx, fmt.Sprintf("found %d pending deliveries to retry", len(deliveries)))

	for _, delivery := range deliveries {
		// Get webhook
		webhook, err := s.store.GetWebhookByID(ctx, delivery.WebhookID)
		if err != nil {
			s.logger.Error(ctx, fmt.Sprintf("failed to get webhook %s", delivery.WebhookID), err)
			continue
		}

		// Skip if webhook is deleted or paused
		if webhook.DeletedAt != nil || webhook.Status != "active" {
			s.logger.Info(ctx, fmt.Sprintf("skipping delivery for inactive webhook %s", webhook.ID))
			continue
		}

		// Reconstruct payload
		payload := WebhookPayload{
			ID:        delivery.ID.String(),
			Type:      delivery.EventType,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			Data:      delivery.Payload,
			AccountID: webhook.AccountID.String(),
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			s.logger.Error(ctx, "failed to marshal payload", err)
			continue
		}

		// Attempt delivery
		success, responseStatus, responseBody, durationMs, deliveryErr := s.deliverWebhook(ctx, webhook, payloadBytes)

		if success {
			// Update delivery status as success
			err = s.store.UpdateWebhookDeliveryStatus(ctx, delivery.ID, store.UpdateWebhookDeliveryStatusParams{
				Status:         "success",
				ResponseStatus: &responseStatus,
				ResponseBody:   &responseBody,
				DurationMs:     &durationMs,
				ErrorMessage:   nil,
			})
			if err != nil {
				s.logger.Error(ctx, "failed to update delivery status", err)
			}

			// Increment webhook sent counter
			err = s.store.IncrementWebhookSent(ctx, webhook.ID)
			if err != nil {
				s.logger.Error(ctx, "failed to increment webhook sent", err)
			}

			s.logger.Info(ctx, fmt.Sprintf("webhook delivery %s succeeded on retry", delivery.ID))
		} else {
			// Handle failure
			errorMessage := ""
			if deliveryErr != nil {
				errorMessage = deliveryErr.Error()
			}

			// Calculate next retry time
			var nextRetryAt *time.Time
			if webhook.RetryEnabled && delivery.AttemptNumber < webhook.MaxRetries {
				nextRetry := s.calculateNextRetry(delivery.AttemptNumber + 1)
				nextRetryAt = &nextRetry
			}

			// Update delivery status as failed
			err = s.store.UpdateWebhookDeliveryStatus(ctx, delivery.ID, store.UpdateWebhookDeliveryStatusParams{
				Status:         "failed",
				ResponseStatus: &responseStatus,
				ResponseBody:   &responseBody,
				DurationMs:     &durationMs,
				ErrorMessage:   &errorMessage,
			})
			if err != nil {
				s.logger.Error(ctx, "failed to update delivery status", err)
			}

			// Set next retry time if applicable
			err = s.store.IncrementDeliveryAttempt(ctx, delivery.ID, nextRetryAt)
			if err != nil {
				s.logger.Error(ctx, "failed to set next retry time", err)
			}
			s.logger.Info(ctx, fmt.Sprintf("webhook delivery %s failed, will retry at %s", delivery.ID, nextRetryAt.Format(time.RFC3339)))
			if delivery.AttemptNumber == webhook.MaxRetries {
				// Increment webhook failed counter (no more retries)
				err = s.store.IncrementWebhookFailed(ctx, webhook.ID)
				if err != nil {
					s.logger.Error(ctx, "failed to increment webhook failed", err)
				}
				s.logger.Error(ctx, fmt.Sprintf("webhook delivery %s failed permanently", delivery.ID), fmt.Errorf("max retries reached"))
			}
		}
	}

	return nil
}

// TestWebhook sends a test event to a webhook
func (s *WebhookService) TestWebhook(ctx context.Context, webhookID uuid.UUID) error {
	webhook, err := s.store.GetWebhookByID(ctx, webhookID)
	if err != nil {
		s.logger.Error(ctx, "failed to get webhook", err)
		return fmt.Errorf("failed to get webhook: %w", err)
	}

	// Create test payload
	payload := WebhookPayload{
		ID:        uuid.New().String(),
		Type:      "webhook.test",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Data: map[string]interface{}{
			"test":    true,
			"message": "This is a test webhook event",
		},
		AccountID: webhook.AccountID.String(),
	}

	return s.sendWebhook(ctx, webhook, payload)
}
