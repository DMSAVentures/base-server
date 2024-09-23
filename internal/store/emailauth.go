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
		s.logger.Error(ctx, "failed to check email exists", err)
		return false, fmt.Errorf("failed to check email on email table: %w", err)
	}
	var existsOnOauthAuth bool
	err = s.db.GetContext(ctx, &existsOnOauthAuth, sqlCheckIfOauthEmailExistsQuery, email)
	if err != nil {
		s.logger.Error(ctx, "failed to check email exists", err)
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

func (s *Store) CreateUserOnEmailSignup(
	ctx context.Context, firstName string, lastName string, email string, hashedPassword string) (User, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		s.logger.Error(ctx, "failed to begin transaction", err)
		return User{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			s.logger.Error(ctx, "rolling back transaction", err)
			err = tx.Rollback()
			if err != nil {
				s.logger.Error(ctx, "failed to rollback transaction", err)
			}
			return
		}
	}()

	var user User
	err = tx.GetContext(ctx, &user, sqlCreateUser, firstName, lastName)
	if err != nil {
		s.logger.Error(ctx, "failed to create user", err)
		return User{}, fmt.Errorf("failed to create user: %w", err)
	}
	var userAuth UserAuth
	err = tx.GetContext(ctx, &userAuth, sqlCreateUserAuth, user.ID, "email")
	if err != nil {
		s.logger.Error(ctx, "failed to create user auth entry", err)
		return User{}, fmt.Errorf("failed to create user auth entry: %w", err)
	}

	var emailAuth EmailAuth
	err = tx.GetContext(ctx, &emailAuth, sqlCreateEmailAuth, userAuth.ID, email, hashedPassword)
	if err != nil {
		s.logger.Error(ctx, "failed to create email auth entry", err)
		return User{}, fmt.Errorf("failed to create email auth entry: %w", err)
	}
	err = tx.Commit()
	if err != nil {
		s.logger.Error(ctx, "failed to commit transaction", err)
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
		s.logger.Error(ctx, "failed to get user by email", err)
		return EmailAuth{}, fmt.Errorf("failed to get user by email: %w", err)
	}
	return userAuthByEmail, nil
}

const sqlGetUserByAuthID = `
SELECT
    loggedInUser.id,
    loggedInUser.first_name,
    loggedInUser.last_name,
    auth.id as auth_id,
    auth.auth_type
FROM users AS loggedInUser
LEFT JOIN user_auth auth
ON
    loggedInUser.id = auth.user_id
WHERE auth.id = $1
`

func (s *Store) GetUserByAuthID(ctx context.Context, authID uuid.UUID) (AuthenticatedUser, error) {
	var authenticatedUser AuthenticatedUser
	err := s.db.GetContext(ctx, &authenticatedUser, sqlGetUserByAuthID, authID)
	if err != nil {
		s.logger.Error(ctx, "failed to get user by auth id", err)
		return AuthenticatedUser{}, fmt.Errorf("failed to get user by auth id: %w", err)
	}
	return authenticatedUser, nil
}
