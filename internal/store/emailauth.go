package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `db:"id"`
	FirstName string    `db:"first_name"`
	LastName  string    `db:"last_name"`
}

type UserAuth struct {
	ID       uuid.UUID `db:"id"`
	UserID   uuid.UUID `db:"user_id"`
	AuthType string    `db:"auth_type"`
}

type EmailAuth struct {
	Email          string    `db:"email"`
	HashedPassword string    `db:"hashed_password"`
	AuthID         uuid.UUID `db:"auth_id"`
}

type UserWithEmail struct {
	User
	Email string
}

type AuthenticatedUser struct {
	UserID    uuid.UUID `db:"id"`
	AccountID uuid.UUID `db:"account_id"`
	FirstName string    `db:"first_name"`
	LastName  string    `db:"last_name"`
	AuthID    uuid.UUID `db:"auth_id"`
	AuthType  string    `db:"auth_type"`
}

const sqlCheckIfEmailExistsQuery = `
SELECT EXISTS(SELECT 1 
              FROM email_auth 
              WHERE email  = $1
              )`

const sqlCheckIfOauthEmailExistsQuery = `
SELECT EXISTS(SELECT 1 
              FROM oauth_auth 
              WHERE email  = $1)`

func (s *Store) CheckIfEmailExists(ctx context.Context, email string) (bool, error) {
	var existsOnEmailAuth bool
	err := s.db.GetContext(ctx, &existsOnEmailAuth, sqlCheckIfEmailExistsQuery, email)
	if err != nil {
		return false, fmt.Errorf("failed to check email on email table: %w", err)
	}
	var existsOnOauthAuth bool
	err = s.db.GetContext(ctx, &existsOnOauthAuth, sqlCheckIfOauthEmailExistsQuery, email)
	if err != nil {
		return false, fmt.Errorf("failed to check email on email table: %w", err)
	}
	return existsOnEmailAuth || existsOnOauthAuth, nil
}

const sqlCreateUser = `
INSERT INTO users (first_name, last_name) 
VALUES ($1, $2) 
RETURNING id, first_name, last_name`

const sqlCreateUserAuth = `
INSERT INTO user_auth (user_id, auth_type) 
VALUES ($1, $2)
RETURNING id, user_id, auth_type`

const sqlCreateEmailAuth = `
INSERT INTO email_auth (auth_id, email, hashed_password) 
VALUES ($1, $2, $3) 
RETURNING email, hashed_password, auth_id`

const sqlCreateAccountForUser = `
INSERT INTO accounts (name, slug, owner_user_id, plan, settings)
VALUES ($1, $2, $3, $4, $5)
RETURNING id`

func (s *Store) CreateUserOnEmailSignup(
	ctx context.Context, firstName string, lastName string, email string, hashedPassword string) (User, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			return
		}
	}()

	var user User
	err = tx.GetContext(ctx, &user, sqlCreateUser, firstName, lastName)
	if err != nil {
		return User{}, fmt.Errorf("failed to create user: %w", err)
	}
	var userAuth UserAuth
	err = tx.GetContext(ctx, &userAuth, sqlCreateUserAuth, user.ID, "email")
	if err != nil {
		return User{}, fmt.Errorf("failed to create user auth entry: %w", err)
	}

	var emailAuth EmailAuth
	err = tx.GetContext(ctx, &emailAuth, sqlCreateEmailAuth, userAuth.ID, email, hashedPassword)
	if err != nil {
		return User{}, fmt.Errorf("failed to create email auth entry: %w", err)
	}

	// Create default account for user
	accountName := fmt.Sprintf("%s %s's Account", firstName, lastName)
	accountSlug := fmt.Sprintf("user-%s", user.ID.String()[:8])
	var accountID uuid.UUID
	err = tx.GetContext(ctx, &accountID, sqlCreateAccountForUser, accountName, accountSlug, user.ID, "free", JSONB{})
	if err != nil {
		return User{}, fmt.Errorf("failed to create account for user: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return User{}, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return user, nil
}

const sqlGetUserByEmail = `
SELECT 
    email,
    hashed_password,
    auth_id 
FROM email_auth 
WHERE email = $1`

func (s *Store) GetCredentialsByEmail(ctx context.Context, email string) (EmailAuth, error) {
	var userAuthByEmail EmailAuth
	err := s.db.GetContext(ctx, &userAuthByEmail, sqlGetUserByEmail, email)
	if err != nil {
		return EmailAuth{}, fmt.Errorf("failed to get user by email: %w", err)
	}
	return userAuthByEmail, nil
}

const sqlGetUserByAuthID = `
SELECT
    loggedInUser.id,
    acc.id as account_id,
    loggedInUser.first_name,
    loggedInUser.last_name,
    auth.id as auth_id,
    auth.auth_type
FROM users AS loggedInUser
LEFT JOIN user_auth auth ON loggedInUser.id = auth.user_id
LEFT JOIN accounts acc ON acc.owner_user_id = loggedInUser.id AND acc.deleted_at IS NULL
WHERE auth.id = $1
`

func (s *Store) GetUserByAuthID(ctx context.Context, authID uuid.UUID) (AuthenticatedUser, error) {
	var authenticatedUser AuthenticatedUser
	err := s.db.GetContext(ctx, &authenticatedUser, sqlGetUserByAuthID, authID)
	if err != nil {
		return AuthenticatedUser{}, fmt.Errorf("failed to get user by auth id: %w", err)
	}
	return authenticatedUser, nil
}
