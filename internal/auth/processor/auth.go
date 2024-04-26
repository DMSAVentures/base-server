package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthProcessor struct {
	store  store.Store
	logger *observability.Logger
}

func New(store store.Store, logger *observability.Logger) AuthProcessor {
	return AuthProcessor{
		store:  store,
		logger: logger,
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

func (p *AuthProcessor) Login(ctx context.Context, email string, password string) (LoggedInUser, error) {
	ctx = observability.WithFields(ctx, observability.Field{Key: "email", Value: email})
	credentialsByEmail, err := p.store.GetCredentialsByEmail(ctx, email)
	if err != nil {
		p.logger.Error(ctx, "failed to get user by email", err)
		return LoggedInUser{}, err
	}
	err = bcrypt.CompareHashAndPassword([]byte(credentialsByEmail.HashedPassword), []byte(password))
	if err != nil {
		p.logger.Error(ctx, "failed to compare hashed password", err)
		return LoggedInUser{}, err
	}
	user, err := p.store.GetUserByAuthID(ctx, credentialsByEmail.AuthID)
	return LoggedInUser{
		FirstName:  user.FirstName,
		LastName:   user.LastName,
		ExternalID: user.ExternalID,
	}, nil

}
