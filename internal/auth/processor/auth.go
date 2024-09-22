package processor

import (
	billingProcessor "base-server/internal/billing/processor"
	"base-server/internal/clients/googleoauth"
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthProcessor struct {
	store             store.Store
	authConfig        AuthConfig
	logger            *observability.Logger
	googleOauthClient *googleoauth.Client
	billingProcessor  billingProcessor.BillingProcessor
}

type EmailConfig struct {
	JWTSecret string
}

type GoogleOauthConfig struct {
	ClientID          string
	ClientSecret      string
	ClientRedirectURL string
	WebAppHost        string
}

type AuthConfig struct {
	Email  EmailConfig
	Google GoogleOauthConfig
}

func New(store store.Store, authConfig AuthConfig, googleOauthClient *googleoauth.Client, billingProcessor billingProcessor.BillingProcessor,
	logger *observability.Logger) AuthProcessor {
	return AuthProcessor{
		store:             store,
		logger:            logger,
		authConfig:        authConfig,
		googleOauthClient: googleOauthClient,
		billingProcessor:  billingProcessor,
	}
}

var ErrEmailAlreadyExists = errors.New("email already exists")
var ErrEmailDoesNotExist = errors.New("email does not exist")

type SignedUpUser struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

type LoggedInUser struct {
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	ExternalID uuid.UUID `json:"external_id"`
}

type User struct {
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	ExternalID uuid.UUID `json:"external_id"`
}

var ErrInvalidJWTToken = errors.New("invalid jwt token")

var ErrParseJWTToken = errors.New("failed to parse jwt token")

type BaseClaims struct {
	ExpirationTime *jwt.NumericDate `json:"exp"`
	IssuedAt       *jwt.NumericDate `json:"iat"`
	NotBefore      *jwt.NumericDate `json:"nbf"`
	Issuer         string           `json:"iss"`
	Subject        string           `json:"sub"`
	Audience       jwt.ClaimStrings `json:"aud"`
	AuthType       string           `json:"auth_type"`
}

func (p *AuthProcessor) Signup(ctx context.Context, firstName string, lastName string, email string, password string) (SignedUpUser, error) {
	ctx = observability.WithFields(ctx, observability.Field{Key: "email", Value: email})
	exists, err := p.store.CheckIfEmailExists(ctx, email)
	if err != nil {
		p.logger.Error(ctx, "failed to check if email exists", err)
		return SignedUpUser{}, err
	}

	if exists {
		return SignedUpUser{}, ErrEmailAlreadyExists
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		p.logger.Error(ctx, "failed to hash password", err)
		return SignedUpUser{}, err
	}

	user, err := p.store.CreateUserOnEmailSignup(ctx, firstName, lastName, email, string(hashedPassword))
	if err != nil {
		p.logger.Error(ctx, "failed to create user", err)
		return SignedUpUser{}, err
	}

	stripeCustomerId, err := p.billingProcessor.CreateStripeCustomer(ctx, email)
	if err != nil {
		p.logger.Error(ctx, "failed to create stripe customer", err)
		return SignedUpUser{}, err
	}

	err = p.store.UpdateStripeCustomerIDByUserID(ctx, user.ID, stripeCustomerId)
	if err != nil {
		p.logger.Error(ctx, "failed to update stripe customer id", err)
		return SignedUpUser{}, err
	}

	return SignedUpUser{
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     email,
	}, nil
}

func (p *AuthProcessor) Login(ctx context.Context, email string, password string) (string, error) {
	ctx = observability.WithFields(ctx, observability.Field{Key: "email", Value: email})
	exists, err := p.store.CheckIfEmailExists(ctx, email)
	if err != nil {
		p.logger.Error(ctx, "failed to check if email exists", err)
		return "", err
	}
	if !exists {
		return "", ErrEmailDoesNotExist
	}
	credentialsByEmail, err := p.store.GetCredentialsByEmail(ctx, email)
	if err != nil {
		p.logger.Error(ctx, "failed to get user by email", err)
		return "", err
	}
	err = bcrypt.CompareHashAndPassword([]byte(credentialsByEmail.HashedPassword), []byte(password))
	if err != nil {
		p.logger.Error(ctx, "failed to compare hashed password", err)
		return "", err
	}
	user, err := p.store.GetUserByAuthID(ctx, credentialsByEmail.AuthID)
	token, err := p.generateJWTToken(user)
	if err != nil {
		p.logger.Error(ctx, "failed to generate jwt token", err)
		return "", err
	}
	return token, nil
}

func (p *AuthProcessor) GetUserByExternalID(ctx context.Context, externalID uuid.UUID) (User, error) {
	user, err := p.store.GetUserByExternalID(ctx, externalID)
	if err != nil {
		p.logger.Error(ctx, "failed to get user by external id", err)
		return User{}, err
	}
	return User{
		FirstName:  user.FirstName,
		LastName:   user.LastName,
		ExternalID: user.ExternalID,
	}, nil

}

func (p *AuthProcessor) GetWebAppHost() string {
	return p.authConfig.Google.WebAppHost
}
