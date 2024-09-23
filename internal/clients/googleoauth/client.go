package googleoauth

import (
	"base-server/internal/observability"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const googleOauthTokenURL = "https://oauth2.googleapis.com/token"

type GoogleOauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	IdToken     string `json:"id_token"`
}

type UserInfo struct {
	ID        string `json:"sub"`
	Email     string `json:"email"`
	FirstName string `json:"given_name"`
	LastName  string `json:"family_name"`
}

type Client struct {
	clientID     string
	clientSecret string
	redirectURL  string
	logger       *observability.Logger
	httpClient   *http.Client
}

func NewClient(clientID, clientSecret, redirectURL string, logger *observability.Logger) *Client {
	httpClient := &http.Client{}
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		logger:       logger,
		httpClient:   httpClient,
	}
}

func (c *Client) GetAccessToken(ctx context.Context, code string) (GoogleOauthTokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", googleOauthTokenURL, nil)
	if err != nil {
		c.logger.InfoWithError(ctx, "failed to create request", err)
		return GoogleOauthTokenResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("code", code)
	q.Add("client_id", c.clientID)
	q.Add("client_secret", c.clientSecret)
	q.Add("redirect_uri", c.redirectURL)
	q.Add("grant_type", "authorization_code")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error(ctx, "failed to make request", err)
		return GoogleOauthTokenResponse{}, fmt.Errorf("failed to make request: %w", err)
	}

	defer resp.Body.Close()

	// Parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.InfoWithError(ctx, "failed to read response body", err)
		return GoogleOauthTokenResponse{}, fmt.Errorf("failed to read response body: %w", err)
	}
	c.logger.Info(ctx, fmt.Sprintf("Request URL was: %s\n StatusCode: %d, Response was: %s",
		resp.Request.URL.String(), resp.StatusCode,
		string(body)))

	if resp.StatusCode != http.StatusOK {
		var errorResponse struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		err = json.Unmarshal(body, &errorResponse)
		if err != nil {
			c.logger.InfoWithError(ctx, "failed to marshal response body", err)
			return GoogleOauthTokenResponse{}, fmt.Errorf("failed to marshal response body: %w", err)
		}
		c.logger.Error(ctx, "failed to get access token", fmt.Errorf("error: %s, description: %s", errorResponse.Error, errorResponse.ErrorDescription))
		return GoogleOauthTokenResponse{}, fmt.Errorf("failed to get access token: %s", errorResponse.ErrorDescription)
	}

	var tokenResponse GoogleOauthTokenResponse
	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		c.logger.InfoWithError(ctx, "failed to marshal response body", err)
		return GoogleOauthTokenResponse{}, fmt.Errorf("failed to marshal response body: %w", err)
	}

	return tokenResponse, nil
}

func (c *Client) GetUserInfo(ctx context.Context, token string) (UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		c.logger.InfoWithError(ctx, "failed to create request", err)
		return UserInfo{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.InfoWithError(ctx, "failed to make request", err)
		return UserInfo{}, fmt.Errorf("failed to make request: %w", err)
	}

	defer resp.Body.Close()

	// Parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.InfoWithError(ctx, "failed to read response body", err)
		return UserInfo{}, fmt.Errorf("failed to read response body: %w", err)
	}

	var userInfo UserInfo
	err = json.Unmarshal(body, &userInfo)
	if err != nil {
		c.logger.InfoWithError(ctx, "failed to marshal response body", err)
		return UserInfo{}, fmt.Errorf("failed to marshal response body: %w", err)
	}

	return userInfo, nil
}
