package googleoauth

import (
	"context"
)

// GoogleOAuthClient defines the interface for Google OAuth operations
type GoogleOAuthClient interface {
	// GetAccessToken exchanges an authorization code for access and ID tokens
	GetAccessToken(ctx context.Context, code string) (GoogleOauthTokenResponse, error)

	// GetUserInfo retrieves user information from Google using an access token
	GetUserInfo(ctx context.Context, token string) (UserInfo, error)
}
