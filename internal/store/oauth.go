package store

import "context"

type OauthAuth struct {
	AuthID       int    `db:"auth_id"`
	UserID       string `db:"user_id"`
	Email        string `db:"email"`
	FullName     string `db:"full_name"`
	AuthProvider string `db:"auth_provider"`
}

const sqlCreateOAuth = `
INSERT INTO oauth_auth (auth_id, user_id, email, full_name, auth_provider)
VALUES ($1, $2, $3, $4, $5)
RETURNING auth_id, user_id, email, full_name, auth_provider
`

func (s *Store) CreateUserOnGoogleSignIn(ctx context.Context, googleUserId string, email string, firstName string,
	lastName string) (User, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		s.logger.Error(ctx, "failed to begin transaction", err)
		return User{}, err
	}
	defer func() {
		err := tx.Rollback()
		if err != nil {
			s.logger.Error(ctx, "failed to rollback transaction", err)
		}
	}()

	var user User
	err = tx.GetContext(ctx, &user, sqlCreateUser, firstName, lastName)
	if err != nil {
		s.logger.Error(ctx, "failed to create user", err)
		return User{}, err
	}
	var userAuth UserAuth
	err = tx.GetContext(ctx, &userAuth, sqlCreateUserAuth, user.ID, "oauth")
	if err != nil {
		s.logger.Error(ctx, "failed to create user auth entry", err)
		return User{}, err
	}

	var oauthAuth OauthAuth
	err = tx.GetContext(ctx, &oauthAuth, sqlCreateOAuth, userAuth.ID, googleUserId, email,
		firstName+" "+lastName, "google")
	if err != nil {
		s.logger.Error(ctx, "failed to create google oauth entry", err)
		return User{}, err
	}
	err = tx.Commit()
	if err != nil {
		s.logger.Error(ctx, "failed to commit transaction", err)
		return User{}, err
	}
	return user, nil
}

const sqlSelectOauthUserByEmail = `
SELECT 
    auth_id,
    user_id,
    auth_provider,
    email
FROM oauth_auth
WHERE email = $1
`

func (s *Store) GetOauthUserByEmail(ctx context.Context, email string) (OauthAuth, error) {
	var userAuthByOauth OauthAuth
	err := s.db.GetContext(ctx, &userAuthByOauth, sqlSelectOauthUserByEmail, email)
	if err != nil {
		s.logger.Error(ctx, "failed to get user by email", err)
		return OauthAuth{}, err
	}
	return userAuthByOauth, err
}
