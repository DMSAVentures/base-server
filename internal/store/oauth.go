package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type OauthAuth struct {
	AuthID       uuid.UUID `db:"auth_id"`
	ExternalID   string    `db:"external_id"`
	Email        string    `db:"email"`
	FullName     string    `db:"full_name"`
	AuthProvider string    `db:"auth_provider"`
}

const sqlCreateOAuth = `
INSERT INTO oauth_auth (auth_id, external_id, email, full_name, auth_provider)
VALUES ($1, $2, $3, $4, $5)
RETURNING auth_id, external_id, email, full_name, auth_provider
`

func (s *Store) CreateUserOnGoogleSignIn(ctx context.Context, googleUserId string, email string, firstName string,
	lastName string) (User, error) {
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
	err = tx.GetContext(ctx, &userAuth, sqlCreateUserAuth, user.ID, "oauth")
	if err != nil {
		return User{}, fmt.Errorf("failed to create user auth entry: %w", err)
	}

	var oauthAuth OauthAuth
	err = tx.GetContext(ctx, &oauthAuth, sqlCreateOAuth, userAuth.ID, googleUserId, email,
		firstName+" "+lastName, "google")
	if err != nil {
		return User{}, fmt.Errorf("failed to create google oauth entry: %w", err)
	}

	// Create default account for user
	accountName := fmt.Sprintf("%s %s's Account", firstName, lastName)
	accountSlug := fmt.Sprintf("user-%s", user.ID.String()[:8])
	var accountID uuid.UUID
	const sqlCreateAccountForOAuthUser = `
		INSERT INTO accounts (name, slug, owner_user_id, plan, settings)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`
	err = tx.GetContext(ctx, &accountID, sqlCreateAccountForOAuthUser, accountName, accountSlug, user.ID, "free", JSONB{})
	if err != nil {
		return User{}, fmt.Errorf("failed to create account for user: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return User{}, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return user, nil
}

const sqlSelectOauthUserByEmail = `
SELECT 
    auth_id,
    external_id,
    auth_provider,
    full_name,
    email
FROM oauth_auth
WHERE email = $1
`

func (s *Store) GetOauthUserByEmail(ctx context.Context, email string) (OauthAuth, error) {
	var userAuthByOauth OauthAuth
	err := s.db.GetContext(ctx, &userAuthByOauth, sqlSelectOauthUserByEmail, email)
	if err != nil {
		return OauthAuth{}, fmt.Errorf("failed to get user by email: %w", err)
	}
	return userAuthByOauth, err
}
