package service

import (
	"base-server/internal/integrations"
	"base-server/internal/observability"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ZapierDeliverer handles delivery of events to Zapier webhooks
type ZapierDeliverer struct {
	client *http.Client
	logger *observability.Logger
}

// NewZapierDeliverer creates a new ZapierDeliverer
func NewZapierDeliverer(logger *observability.Logger) *ZapierDeliverer {
	return &ZapierDeliverer{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// Type returns the integration type this deliverer handles
func (z *ZapierDeliverer) Type() integrations.IntegrationType {
	return integrations.IntegrationZapier
}

// ZapierPayload represents the payload structure for Zapier
type ZapierPayload struct {
	ID         string                 `json:"id"`
	Event      string                 `json:"event"`
	OccurredAt string                 `json:"occurred_at"`
	AccountID  string                 `json:"account_id"`
	CampaignID string                 `json:"campaign_id,omitempty"`
	Data       map[string]interface{} `json:"data"`
}

// Deliver sends an event to the Zapier webhook URL
func (z *ZapierDeliverer) Deliver(ctx context.Context, sub integrations.Subscription, event integrations.Event) error {
	payload, err := z.FormatPayload(event, sub.Config)
	if err != nil {
		return fmt.Errorf("failed to format payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.TargetURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Waitlist-Integration/1.0")

	resp, err := z.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for logging
	body, _ := io.ReadAll(resp.Body)

	// Zapier returns 200 for success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		ctx = observability.WithFields(ctx,
			observability.Field{Key: "status_code", Value: resp.StatusCode},
			observability.Field{Key: "target_url", Value: sub.TargetURL})
		z.logger.Info(ctx, "zapier delivery successful")
		return nil
	}

	return fmt.Errorf("zapier returned status %d: %s", resp.StatusCode, string(body))
}

// FormatPayload formats the event into Zapier's expected format
func (z *ZapierDeliverer) FormatPayload(event integrations.Event, config map[string]interface{}) ([]byte, error) {
	payload := ZapierPayload{
		ID:         event.ID,
		Event:      event.Type,
		OccurredAt: event.Timestamp.UTC().Format(time.RFC3339),
		AccountID:  event.AccountID.String(),
		Data:       event.Data,
	}

	if event.CampaignID != nil {
		payload.CampaignID = event.CampaignID.String()
	}

	// Flatten the data for Zapier compatibility if configured
	if flatten, ok := config["flatten"].(bool); ok && flatten {
		return z.flattenPayload(payload)
	}

	return json.Marshal(payload)
}

// flattenPayload flattens nested data for better Zapier field mapping
func (z *ZapierDeliverer) flattenPayload(payload ZapierPayload) ([]byte, error) {
	flat := map[string]interface{}{
		"id":          payload.ID,
		"event":       payload.Event,
		"occurred_at": payload.OccurredAt,
		"account_id":  payload.AccountID,
	}

	if payload.CampaignID != "" {
		flat["campaign_id"] = payload.CampaignID
	}

	// Flatten nested data with prefixes
	flattenMap(flat, payload.Data, "")

	return json.Marshal(flat)
}

// flattenMap recursively flattens a map with dot notation keys
func flattenMap(result map[string]interface{}, data map[string]interface{}, prefix string) {
	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "_" + key
		}

		switch v := value.(type) {
		case map[string]interface{}:
			flattenMap(result, v, fullKey)
		default:
			result[fullKey] = v
		}
	}
}
