package googleoauth

import (
	"base-server/internal/observability"
	"context"
	"encoding/json"
	"io/ioutil"
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
		return GoogleOauthTokenResponse{}, err
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
		return GoogleOauthTokenResponse{}, err
	}
	defer resp.Body.Close()

	// Parse the response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.logger.InfoWithError(ctx, "failed to read response body", err)
		return GoogleOauthTokenResponse{}, err
	}
	var tokenResponse GoogleOauthTokenResponse
	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		c.logger.InfoWithError(ctx, "failed to marshal response body", err)
		return GoogleOauthTokenResponse{}, err
	}

	return tokenResponse, nil
}

func (c *Client) GetUserInfo(ctx context.Context, token string) (UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		c.logger.InfoWithError(ctx, "failed to create request", err)
		return UserInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.InfoWithError(ctx, "failed to make request", err)
		return UserInfo{}, err
	}
	defer resp.Body.Close()

	// Parse the response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.logger.InfoWithError(ctx, "failed to read response body", err)
		return UserInfo{}, err
	}
	var userInfo UserInfo
	err = json.Unmarshal(body, &userInfo)
	if err != nil {
		c.logger.InfoWithError(ctx, "failed to marshal response body", err)
		return UserInfo{}, err
	}

	return userInfo, nil
}
