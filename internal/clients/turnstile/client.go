package turnstile

import (
	"base-server/internal/observability"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const verifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

var (
	ErrInvalidToken     = errors.New("invalid captcha token")
	ErrVerificationFail = errors.New("captcha verification failed")
)

// VerifyResponse represents the response from Cloudflare Turnstile API
type VerifyResponse struct {
	Success     bool     `json:"success"`
	ErrorCodes  []string `json:"error-codes,omitempty"`
	ChallengeTS string   `json:"challenge_ts,omitempty"`
	Hostname    string   `json:"hostname,omitempty"`
	Action      string   `json:"action,omitempty"`
	CData       string   `json:"cdata,omitempty"`
}

// Client handles Cloudflare Turnstile verification
type Client struct {
	secretKey  string
	httpClient *http.Client
	logger     *observability.Logger
}

// NewClient creates a new Turnstile verification client
func NewClient(secretKey string, logger *observability.Logger) *Client {
	return &Client{
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// Verify validates a Turnstile token
// Returns nil if valid, error otherwise
func (c *Client) Verify(ctx context.Context, token string, remoteIP string) error {
	if token == "" {
		return ErrInvalidToken
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "captcha_type", Value: "turnstile"},
	)

	// Build request payload
	payload := map[string]string{
		"secret":   c.secretKey,
		"response": token,
	}
	if remoteIP != "" {
		payload["remoteip"] = remoteIP
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		c.logger.Error(ctx, "failed to marshal turnstile request", err)
		return fmt.Errorf("failed to prepare verification request: %w", err)
	}

	// Make request to Cloudflare
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifyURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		c.logger.Error(ctx, "failed to create turnstile request", err)
		return fmt.Errorf("failed to create verification request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error(ctx, "failed to call turnstile API", err)
		return fmt.Errorf("failed to verify captcha: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var verifyResp VerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&verifyResp); err != nil {
		c.logger.Error(ctx, "failed to parse turnstile response", err)
		return fmt.Errorf("failed to parse verification response: %w", err)
	}

	if !verifyResp.Success {
		c.logger.Info(ctx, fmt.Sprintf("turnstile verification failed: %v", verifyResp.ErrorCodes))
		return ErrVerificationFail
	}

	c.logger.Info(ctx, "turnstile verification successful")
	return nil
}

// IsEnabled returns true if the client has a secret key configured
func (c *Client) IsEnabled() bool {
	return c.secretKey != ""
}
