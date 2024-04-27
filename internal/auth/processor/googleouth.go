package processor

import (
	"context"
)

func (p *AuthProcessor) SignInGoogleUserWithCode(ctx context.Context, code string) (string, error) {
	// Get the access token from the code
	token, err := p.googleOauthClient.GetAccessToken(ctx, code)
	if err != nil {
		p.logger.InfoWithError(ctx, "failed to get access token", err)
		return "", err
	}

	// Get the userAuth info from the access token
	userInfo, err := p.googleOauthClient.GetUserInfo(ctx, token.AccessToken)
	if err != nil {
		p.logger.InfoWithError(ctx, "failed to get userAuth info", err)
		return "", err
	}

	// Check if the userAuth exists in the database
	exists, err := p.store.CheckIfEmailExists(ctx, userInfo.Email)
	if err != nil {
		p.logger.InfoWithError(ctx, "failed to check if email exists", err)
		return "", err
	}

	// If the userAuth does not exist, create a new userAuth
	if !exists {
		_, err = p.store.CreateUserOnGoogleSignIn(ctx, userInfo.ID, userInfo.Email, userInfo.FirstName,
			userInfo.LastName)
		if err != nil {
			p.logger.InfoWithError(ctx, "failed to create userAuth on google sign in", err)
			return "", err
		}
	}
	// If the userAuth exists, get the userAuth
	oauthUser, err := p.store.GetOauthUserByEmail(ctx, userInfo.Email)
	if err != nil {
		p.logger.InfoWithError(ctx, "failed to get userAuth by email", err)
		return "", err
	}

	authenticatedUser, err := p.store.GetUserByAuthID(ctx, oauthUser.AuthID)
	if err != nil {
		p.logger.InfoWithError(ctx, "failed to get userAuth by auth id", err)
		return "", err
	}

	// Generate a JWT token
	jwtToken, err := p.generateJWTToken(authenticatedUser)
	if err != nil {
		p.logger.InfoWithError(ctx, "failed to generate jwt token", err)
		return "", err
	}

	return jwtToken, nil
}
