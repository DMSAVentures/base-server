//go:build !integration

package processor

import (
	"base-server/internal/clients/googleoauth"
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSignInGoogleUserWithCode_NewUser_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockGoogleOAuth := NewMockGoogleOAuthClient(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	authConfig := AuthConfig{
		Email: EmailConfig{JWTSecret: "test-secret-key-that-is-long-enough"},
	}
	processor := New(mockStore, authConfig, mockGoogleOAuth, mockBilling, mockEmail, logger)

	ctx := context.Background()
	code := "google_auth_code_123"
	accessToken := "google_access_token_123"
	email := "newuser@gmail.com"
	googleUserID := "google_user_123"
	firstName := "Jane"
	lastName := "Doe"
	userID := uuid.New()
	authID := uuid.New()
	accountID := uuid.New()
	stripeCustomerID := "cus_new_123"

	// Mock Google OAuth flow
	mockGoogleOAuth.EXPECT().GetAccessToken(gomock.Any(), code).Return(googleoauth.GoogleOauthTokenResponse{
		AccessToken: accessToken,
	}, nil)

	mockGoogleOAuth.EXPECT().GetUserInfo(gomock.Any(), accessToken).Return(googleoauth.UserInfo{
		ID:        googleUserID,
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
	}, nil)

	// User doesn't exist yet
	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(false, nil)

	// Create new user flow
	mockStore.EXPECT().CreateUserOnGoogleSignIn(gomock.Any(), googleUserID, email, firstName, lastName).
		Return(store.User{ID: userID, FirstName: firstName, LastName: lastName}, nil)

	mockBilling.EXPECT().CreateStripeCustomer(gomock.Any(), email).Return(stripeCustomerID, nil)
	mockStore.EXPECT().UpdateStripeCustomerIDByUserID(gomock.Any(), userID, stripeCustomerID).Return(nil)
	mockBilling.EXPECT().CreateFreeSubscription(gomock.Any(), stripeCustomerID).Return(nil)

	// Get OAuth user and generate token
	mockStore.EXPECT().GetOauthUserByEmail(gomock.Any(), email).Return(store.OauthAuth{
		AuthID: authID,
	}, nil)

	mockStore.EXPECT().GetUserByAuthID(gomock.Any(), authID).Return(store.AuthenticatedUser{
		UserID:    userID,
		AccountID: accountID,
		AuthID:    authID,
		AuthType:  "oauth",
	}, nil)

	token, err := processor.SignInGoogleUserWithCode(ctx, code)

	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestSignInGoogleUserWithCode_ExistingUser_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockGoogleOAuth := NewMockGoogleOAuthClient(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	authConfig := AuthConfig{
		Email: EmailConfig{JWTSecret: "test-secret-key-that-is-long-enough"},
	}
	processor := New(mockStore, authConfig, mockGoogleOAuth, mockBilling, mockEmail, logger)

	ctx := context.Background()
	code := "google_auth_code_123"
	accessToken := "google_access_token_123"
	email := "existinguser@gmail.com"
	googleUserID := "google_user_456"
	firstName := "John"
	lastName := "Smith"
	authID := uuid.New()
	userID := uuid.New()
	accountID := uuid.New()

	// Mock Google OAuth flow
	mockGoogleOAuth.EXPECT().GetAccessToken(gomock.Any(), code).Return(googleoauth.GoogleOauthTokenResponse{
		AccessToken: accessToken,
	}, nil)

	mockGoogleOAuth.EXPECT().GetUserInfo(gomock.Any(), accessToken).Return(googleoauth.UserInfo{
		ID:        googleUserID,
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
	}, nil)

	// User already exists
	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(true, nil)

	// Get OAuth user and generate token (no user creation needed)
	mockStore.EXPECT().GetOauthUserByEmail(gomock.Any(), email).Return(store.OauthAuth{
		AuthID: authID,
	}, nil)

	mockStore.EXPECT().GetUserByAuthID(gomock.Any(), authID).Return(store.AuthenticatedUser{
		UserID:    userID,
		AccountID: accountID,
		AuthID:    authID,
		AuthType:  "oauth",
	}, nil)

	token, err := processor.SignInGoogleUserWithCode(ctx, code)

	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestSignInGoogleUserWithCode_GetAccessTokenError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockGoogleOAuth := NewMockGoogleOAuthClient(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, mockGoogleOAuth, mockBilling, mockEmail, logger)

	ctx := context.Background()
	code := "invalid_code"

	mockGoogleOAuth.EXPECT().GetAccessToken(gomock.Any(), code).Return(
		googleoauth.GoogleOauthTokenResponse{}, errors.New("invalid code"))

	_, err := processor.SignInGoogleUserWithCode(ctx, code)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFailedSignIn)
}

func TestSignInGoogleUserWithCode_GetUserInfoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockGoogleOAuth := NewMockGoogleOAuthClient(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, mockGoogleOAuth, mockBilling, mockEmail, logger)

	ctx := context.Background()
	code := "google_auth_code_123"
	accessToken := "invalid_token"

	mockGoogleOAuth.EXPECT().GetAccessToken(gomock.Any(), code).Return(googleoauth.GoogleOauthTokenResponse{
		AccessToken: accessToken,
	}, nil)

	mockGoogleOAuth.EXPECT().GetUserInfo(gomock.Any(), accessToken).Return(
		googleoauth.UserInfo{}, errors.New("invalid token"))

	_, err := processor.SignInGoogleUserWithCode(ctx, code)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFailedSignIn)
}

func TestSignInGoogleUserWithCode_CheckEmailExistsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockGoogleOAuth := NewMockGoogleOAuthClient(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, mockGoogleOAuth, mockBilling, mockEmail, logger)

	ctx := context.Background()
	code := "google_auth_code_123"
	accessToken := "google_access_token_123"
	email := "test@gmail.com"

	mockGoogleOAuth.EXPECT().GetAccessToken(gomock.Any(), code).Return(googleoauth.GoogleOauthTokenResponse{
		AccessToken: accessToken,
	}, nil)

	mockGoogleOAuth.EXPECT().GetUserInfo(gomock.Any(), accessToken).Return(googleoauth.UserInfo{
		Email: email,
	}, nil)

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(false, errors.New("db error"))

	_, err := processor.SignInGoogleUserWithCode(ctx, code)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFailedSignIn)
}

func TestSignInGoogleUserWithCode_CreateUserError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockGoogleOAuth := NewMockGoogleOAuthClient(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, mockGoogleOAuth, mockBilling, mockEmail, logger)

	ctx := context.Background()
	code := "google_auth_code_123"
	accessToken := "google_access_token_123"
	email := "newuser@gmail.com"
	googleUserID := "google_user_123"
	firstName := "Jane"
	lastName := "Doe"

	mockGoogleOAuth.EXPECT().GetAccessToken(gomock.Any(), code).Return(googleoauth.GoogleOauthTokenResponse{
		AccessToken: accessToken,
	}, nil)

	mockGoogleOAuth.EXPECT().GetUserInfo(gomock.Any(), accessToken).Return(googleoauth.UserInfo{
		ID:        googleUserID,
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
	}, nil)

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(false, nil)
	mockStore.EXPECT().CreateUserOnGoogleSignIn(gomock.Any(), googleUserID, email, firstName, lastName).
		Return(store.User{}, errors.New("db error"))

	_, err := processor.SignInGoogleUserWithCode(ctx, code)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFailedSignIn)
}

func TestSignInGoogleUserWithCode_CreateStripeCustomerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockGoogleOAuth := NewMockGoogleOAuthClient(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, mockGoogleOAuth, mockBilling, mockEmail, logger)

	ctx := context.Background()
	code := "google_auth_code_123"
	accessToken := "google_access_token_123"
	email := "newuser@gmail.com"
	googleUserID := "google_user_123"
	firstName := "Jane"
	lastName := "Doe"
	userID := uuid.New()

	mockGoogleOAuth.EXPECT().GetAccessToken(gomock.Any(), code).Return(googleoauth.GoogleOauthTokenResponse{
		AccessToken: accessToken,
	}, nil)

	mockGoogleOAuth.EXPECT().GetUserInfo(gomock.Any(), accessToken).Return(googleoauth.UserInfo{
		ID:        googleUserID,
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
	}, nil)

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(false, nil)
	mockStore.EXPECT().CreateUserOnGoogleSignIn(gomock.Any(), googleUserID, email, firstName, lastName).
		Return(store.User{ID: userID}, nil)
	mockBilling.EXPECT().CreateStripeCustomer(gomock.Any(), email).Return("", errors.New("stripe error"))

	_, err := processor.SignInGoogleUserWithCode(ctx, code)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFailedSignIn)
}

func TestSignInGoogleUserWithCode_UpdateStripeCustomerIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockGoogleOAuth := NewMockGoogleOAuthClient(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, mockGoogleOAuth, mockBilling, mockEmail, logger)

	ctx := context.Background()
	code := "google_auth_code_123"
	accessToken := "google_access_token_123"
	email := "newuser@gmail.com"
	googleUserID := "google_user_123"
	firstName := "Jane"
	lastName := "Doe"
	userID := uuid.New()
	stripeCustomerID := "cus_123"

	mockGoogleOAuth.EXPECT().GetAccessToken(gomock.Any(), code).Return(googleoauth.GoogleOauthTokenResponse{
		AccessToken: accessToken,
	}, nil)

	mockGoogleOAuth.EXPECT().GetUserInfo(gomock.Any(), accessToken).Return(googleoauth.UserInfo{
		ID:        googleUserID,
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
	}, nil)

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(false, nil)
	mockStore.EXPECT().CreateUserOnGoogleSignIn(gomock.Any(), googleUserID, email, firstName, lastName).
		Return(store.User{ID: userID}, nil)
	mockBilling.EXPECT().CreateStripeCustomer(gomock.Any(), email).Return(stripeCustomerID, nil)
	mockStore.EXPECT().UpdateStripeCustomerIDByUserID(gomock.Any(), userID, stripeCustomerID).Return(errors.New("db error"))

	_, err := processor.SignInGoogleUserWithCode(ctx, code)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFailedSignIn)
}

func TestSignInGoogleUserWithCode_CreateFreeSubscriptionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockGoogleOAuth := NewMockGoogleOAuthClient(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, mockGoogleOAuth, mockBilling, mockEmail, logger)

	ctx := context.Background()
	code := "google_auth_code_123"
	accessToken := "google_access_token_123"
	email := "newuser@gmail.com"
	googleUserID := "google_user_123"
	firstName := "Jane"
	lastName := "Doe"
	userID := uuid.New()
	stripeCustomerID := "cus_123"

	mockGoogleOAuth.EXPECT().GetAccessToken(gomock.Any(), code).Return(googleoauth.GoogleOauthTokenResponse{
		AccessToken: accessToken,
	}, nil)

	mockGoogleOAuth.EXPECT().GetUserInfo(gomock.Any(), accessToken).Return(googleoauth.UserInfo{
		ID:        googleUserID,
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
	}, nil)

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(false, nil)
	mockStore.EXPECT().CreateUserOnGoogleSignIn(gomock.Any(), googleUserID, email, firstName, lastName).
		Return(store.User{ID: userID}, nil)
	mockBilling.EXPECT().CreateStripeCustomer(gomock.Any(), email).Return(stripeCustomerID, nil)
	mockStore.EXPECT().UpdateStripeCustomerIDByUserID(gomock.Any(), userID, stripeCustomerID).Return(nil)
	mockBilling.EXPECT().CreateFreeSubscription(gomock.Any(), stripeCustomerID).Return(errors.New("free subscription error"))

	_, err := processor.SignInGoogleUserWithCode(ctx, code)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFailedSignIn)
}

func TestSignInGoogleUserWithCode_GetOauthUserByEmailError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockGoogleOAuth := NewMockGoogleOAuthClient(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, mockGoogleOAuth, mockBilling, mockEmail, logger)

	ctx := context.Background()
	code := "google_auth_code_123"
	accessToken := "google_access_token_123"
	email := "existinguser@gmail.com"

	mockGoogleOAuth.EXPECT().GetAccessToken(gomock.Any(), code).Return(googleoauth.GoogleOauthTokenResponse{
		AccessToken: accessToken,
	}, nil)

	mockGoogleOAuth.EXPECT().GetUserInfo(gomock.Any(), accessToken).Return(googleoauth.UserInfo{
		Email: email,
	}, nil)

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(true, nil)
	mockStore.EXPECT().GetOauthUserByEmail(gomock.Any(), email).Return(store.OauthAuth{}, errors.New("db error"))

	_, err := processor.SignInGoogleUserWithCode(ctx, code)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFailedSignIn)
}

func TestSignInGoogleUserWithCode_GetUserByAuthIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockGoogleOAuth := NewMockGoogleOAuthClient(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, mockGoogleOAuth, mockBilling, mockEmail, logger)

	ctx := context.Background()
	code := "google_auth_code_123"
	accessToken := "google_access_token_123"
	email := "existinguser@gmail.com"
	authID := uuid.New()

	mockGoogleOAuth.EXPECT().GetAccessToken(gomock.Any(), code).Return(googleoauth.GoogleOauthTokenResponse{
		AccessToken: accessToken,
	}, nil)

	mockGoogleOAuth.EXPECT().GetUserInfo(gomock.Any(), accessToken).Return(googleoauth.UserInfo{
		Email: email,
	}, nil)

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(true, nil)
	mockStore.EXPECT().GetOauthUserByEmail(gomock.Any(), email).Return(store.OauthAuth{
		AuthID: authID,
	}, nil)
	mockStore.EXPECT().GetUserByAuthID(gomock.Any(), authID).Return(store.AuthenticatedUser{}, errors.New("db error"))

	_, err := processor.SignInGoogleUserWithCode(ctx, code)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFailedSignIn)
}
