package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthProcessor struct {
	store     store.Store
	jwtSecret string
	logger    *observability.Logger
}

func New(store store.Store, jwtSecret string, logger *observability.Logger) AuthProcessor {
	return AuthProcessor{
		store:     store,
		logger:    logger,
		jwtSecret: jwtSecret,
	}
}

var ErrEmailAlreadyExists = errors.New("email already exists")

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

func (p *AuthProcessor) Signup(
	ctx context.Context, firstName string, lastName string, email string, password string) (SignedUpUser, error) {
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
	user, email, err := p.store.CreateUserOnEmailSignup(ctx, firstName, lastName, email, string(hashedPassword))
	if err != nil {
		p.logger.Error(ctx, "failed to create user", err)
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

func (p *AuthProcessor) generateJWTToken(user store.AuthenticatedUser) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour) // Token valid for 24 hours
	cl := jwt.New(jwt.SigningMethodHS256)
	claims := cl.Claims.(jwt.MapClaims)
	claims["sub"] = user.ExternalID
	claims["auth_type"] = user.AuthType
	claims["iss"] = "base-server"
	claims["aud"] = "base-server"
	claims["exp"] = expirationTime.Unix()
	claims["iat"] = time.Now().Unix()

	// Create a new token object
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with the secret key
	tokenString, err := token.SignedString([]byte(p.jwtSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (b *BaseClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	return b.ExpirationTime, nil
}

func (b *BaseClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	return b.IssuedAt, nil
}

func (b *BaseClaims) GetNotBefore() (*jwt.NumericDate, error) {
	return b.NotBefore, nil
}

func (b *BaseClaims) GetIssuer() (string, error) {
	return b.Issuer, nil
}

func (b *BaseClaims) GetSubject() (string, error) {
	return b.Subject, nil
}

func (b *BaseClaims) GetAudience() (jwt.ClaimStrings, error) {
	return b.Audience, nil
}

func (p *AuthProcessor) ValidateJWTToken(ctx context.Context, token string) (BaseClaims, error) {
	var baseClaims BaseClaims
	// Parse the token
	t, err := jwt.ParseWithClaims(token, &baseClaims, func(token *jwt.Token) (interface{}, error) {
		// Check the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(p.jwtSecret), nil
	})
	if err != nil {
		return BaseClaims{}, ErrParseJWTToken
	}
	if !t.Valid {
		return BaseClaims{}, ErrInvalidJWTToken
	}

	// Extract claims from the parsed token
	claims, ok := t.Claims.(*BaseClaims)
	if !ok {
		return BaseClaims{}, ErrParseJWTToken
	}
	return *claims, nil
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
