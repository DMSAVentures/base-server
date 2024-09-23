package processor

import (
	"context"
	"errors"
)

var (
	ErrFailedSignIn = errors.New("failed to sign in")
)

func (p *AuthProcessor) SignInGoogleUserWithCode(ctx context.Context, code string) (string, error) {
	// Get the access token from the code
	token, err := p.googleOauthClient.GetAccessToken(ctx, code)
	if err != nil {
		p.logger.InfoWithError(ctx, "failed to get access token", err)
		return "", ErrFailedSignIn
	}

	// Get the userAuth info from the access token
	userInfo, err := p.googleOauthClient.GetUserInfo(ctx, token.AccessToken)
	if err != nil {
		p.logger.InfoWithError(ctx, "failed to get userAuth info", err)
		return "", ErrFailedSignIn
	}

	// Check if the userAuth exists in the database
	exists, err := p.store.CheckIfEmailExists(ctx, userInfo.Email)
	if err != nil {
		p.logger.InfoWithError(ctx, "failed to check if email exists", err)
		return "", ErrFailedSignIn
	}

	// If the userAuth does not exist, create a new userAuth
	if !exists {
		user, err := p.store.CreateUserOnGoogleSignIn(ctx, userInfo.ID, userInfo.Email, userInfo.FirstName,
			userInfo.LastName)

		if err != nil {
			p.logger.InfoWithError(ctx, "failed to create userAuth on google sign in", err)
			return "", ErrFailedSignIn
		}

		stripeCustomerId, err := p.billingProcessor.CreateStripeCustomer(ctx, userInfo.Email)
		if err != nil {
			p.logger.Error(ctx, "failed to create stripe customer", err)
			return "", ErrFailedSignIn
		}

		err = p.store.UpdateStripeCustomerIDByUserID(ctx, user.ID, stripeCustomerId)
		if err != nil {
			p.logger.Error(ctx, "failed to update stripe customer id", err)
			return "", ErrFailedSignIn
		}
	}
	// If the userAuth exists, get the userAuth
	oauthUser, err := p.store.GetOauthUserByEmail(ctx, userInfo.Email)
	if err != nil {
		p.logger.InfoWithError(ctx, "failed to get userAuth by email", err)
		return "", ErrFailedSignIn
	}

	authenticatedUser, err := p.store.GetUserByAuthID(ctx, oauthUser.AuthID)
	if err != nil {
		p.logger.InfoWithError(ctx, "failed to get userAuth by auth id", err)
		return "", ErrFailedSignIn
	}

	// Generate a JWT token
	jwtToken, err := p.generateJWTToken(ctx, authenticatedUser)
	if err != nil {
		p.logger.InfoWithError(ctx, "failed to generate jwt token", err)
		return "", ErrFailedSignIn
	}

	return jwtToken, nil
}
