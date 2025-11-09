package processor

import (
	"base-server/internal/clients/googleoauth"
	"base-server/internal/store"
	"context"

	"github.com/google/uuid"
)

// AuthStore defines the database operations required by AuthProcessor
type AuthStore interface {
	CheckIfEmailExists(ctx context.Context, email string) (bool, error)
	CreateUserOnEmailSignup(ctx context.Context, firstName string, lastName string, email string, hashedPassword string) (store.User, error)
	CreateUserOnGoogleSignIn(ctx context.Context, googleUserId string, email string, firstName string, lastName string) (store.User, error)
	GetCredentialsByEmail(ctx context.Context, email string) (store.EmailAuth, error)
	GetOauthUserByEmail(ctx context.Context, email string) (store.OauthAuth, error)
	GetUserByAuthID(ctx context.Context, authID uuid.UUID) (store.AuthenticatedUser, error)
	GetUserByExternalID(ctx context.Context, externalID uuid.UUID) (store.User, error)
	UpdateStripeCustomerIDByUserID(ctx context.Context, userID uuid.UUID, stripeCustomerID string) error
}

// BillingProcessor defines the billing operations required by AuthProcessor
type BillingProcessor interface {
	CreateStripeCustomer(ctx context.Context, email string) (string, error)
}

// GoogleOAuthClient defines the OAuth operations required by AuthProcessor
type GoogleOAuthClient interface {
	GetAccessToken(ctx context.Context, code string) (googleoauth.GoogleOauthTokenResponse, error)
	GetUserInfo(ctx context.Context, token string) (googleoauth.UserInfo, error)
}

// EmailService defines the email operations required by AuthProcessor
type EmailService interface {
	SendEmail(ctx context.Context, to, subject, htmlContent string) error
}
