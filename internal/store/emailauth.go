package store

import "context"
import "github.com/google/uuid"

type User struct {
	ID         int       `db:"id"`
	ExternalID uuid.UUID `db:"external_id"`
	FirstName  string    `db:"first_name"`
	LastName   string    `db:"last_name"`
}

type UserAuth struct {
	ID       int    `db:"auth_id"`
	UserID   int    `db:"user_id"`
	AuthType string `db:"auth_type"`
}

type EmailAuth struct {
	Email          string `db:"email"`
	HashedPassword string `db:"hashed_password"`
	AuthID         int    `db:"auth_id"`
}

type UserWithEmail struct {
	User
	Email string
}

const sqlCheckIfEmailExistsQuery = `SELECT EXISTS(SELECT 1 FROM email_auth WHERE email  = $1)`
const sqlCheckIfOauthEmailExistsQuery = `SELECT EXISTS(SELECT 1 FROM oauth_auth WHERE email  = $1)`
const sqlCreateUser = `INSERT INTO users (first_name, last_name) VALUES ($1, $2) RETURNING id, external_id, first_name, last_name`
const sqlCreateUserAuth = `INSERT INTO user_auth (user_id, auth_type) VALUES ($1, $2) RETURNING auth_id, user_id, auth_type`
const sqlCreateEmailAuth = `INSERT INTO email_auth (auth_id, email, hashed_password) VALUES ($1, $2, $3) RETURNING email, hashed_password, auth_id`

func (s *Store) CheckIfEmailExists(ctx context.Context, email string) (bool, error) {
	var existsOnEmailAuth bool
	err := s.db.GetContext(ctx, &existsOnEmailAuth, sqlCheckIfEmailExistsQuery, email)
	if err != nil {
		s.logger.Error(ctx, "failed to check if email existsOnEmailAuth on email auth table", err)
		return false, err
	}
	var existsOnOauthAuth bool
	err = s.db.GetContext(ctx, &existsOnOauthAuth, sqlCheckIfOauthEmailExistsQuery, email)
	if err != nil {
		s.logger.Error(ctx, "failed to check if email existsOnEmailAuth on email auth table", err)
		return false, err
	}
	return existsOnEmailAuth || existsOnOauthAuth, nil
}

func (s *Store) CreateUserOnEmailSignup(ctx context.Context, firstName string, lastName string, email string, hashedPassword string) (User, string, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		s.logger.Error(ctx, "failed to begin transaction", err)
		return User{}, "", err
	}
	defer func() {
		tx.Rollback()
	}()

	var user User
	err = tx.GetContext(ctx, &user, sqlCreateUser, firstName, lastName)
	if err != nil {
		s.logger.Error(ctx, "failed to create user", err)
		return User{}, "", err
	}
	var userAuth UserAuth
	err = tx.GetContext(ctx, &userAuth, sqlCreateUserAuth, user.ID, "email")
	if err != nil {
		s.logger.Error(ctx, "failed to create user auth entry", err)
		return User{}, "", err
	}

	var emailAuth EmailAuth
	err = tx.GetContext(ctx, &emailAuth, sqlCreateEmailAuth, userAuth.ID, email, hashedPassword)
	if err != nil {
		s.logger.Error(ctx, "failed to create email auth entry", err)
		return User{}, "", err
	}
	err = tx.Commit()
	if err != nil {
		s.logger.Error(ctx, "failed to commit transaction", err)
		return User{}, "", err
	}
	return user, email, nil
}
