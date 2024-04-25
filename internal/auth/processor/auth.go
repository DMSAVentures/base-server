package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"

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

func (p AuthProcessor) Signup(ctx context.Context, firstName string, lastName string, email string, password string) (SignedUpUser, error) {
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
