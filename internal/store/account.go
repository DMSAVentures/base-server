package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// CreateAccountParams represents parameters for creating an account
type CreateAccountParams struct {
	Name             string
	Slug             string
	OwnerUserID      uuid.UUID
	Plan             string
	StripeCustomerID *string
}

// UpdateAccountParams represents parameters for updating an account
type UpdateAccountParams struct {
	Name             *string
	Plan             *string
	Status           *string
	StripeCustomerID *string
	Settings         JSONB
}

const sqlCreateAccount = `
INSERT INTO accounts (name, slug, owner_user_id, plan, stripe_customer_id, settings)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, name, slug, owner_user_id, plan, status, stripe_customer_id, trial_ends_at, settings, created_at, updated_at, deleted_at
`

// CreateAccount creates a new account
func (s *Store) CreateAccount(ctx context.Context, params CreateAccountParams) (Account, error) {
	var account Account
	err := s.db.GetContext(ctx, &account, sqlCreateAccount,
		params.Name,
		params.Slug,
		params.OwnerUserID,
		params.Plan,
		params.StripeCustomerID,
		JSONB{})
	if err != nil {
		return Account{}, fmt.Errorf("failed to create account: %w", err)
	}
	return account, nil
}

const sqlGetAccountByID = `
SELECT id, name, slug, owner_user_id, plan, status, stripe_customer_id, trial_ends_at, settings, created_at, updated_at, deleted_at
FROM accounts
WHERE id = $1 AND deleted_at IS NULL
`

// GetAccountByID retrieves an account by ID
func (s *Store) GetAccountByID(ctx context.Context, accountID uuid.UUID) (Account, error) {
	var account Account
	err := s.db.GetContext(ctx, &account, sqlGetAccountByID, accountID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Account{}, ErrNotFound
		}
		return Account{}, fmt.Errorf("failed to get account by id: %w", err)
	}
	return account, nil
}

const sqlGetAccountBySlug = `
SELECT id, name, slug, owner_user_id, plan, status, stripe_customer_id, trial_ends_at, settings, created_at, updated_at, deleted_at
FROM accounts
WHERE slug = $1 AND deleted_at IS NULL
`

// GetAccountBySlug retrieves an account by slug
func (s *Store) GetAccountBySlug(ctx context.Context, slug string) (Account, error) {
	var account Account
	err := s.db.GetContext(ctx, &account, sqlGetAccountBySlug, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Account{}, ErrNotFound
		}
		return Account{}, fmt.Errorf("failed to get account by slug: %w", err)
	}
	return account, nil
}

const sqlGetAccountsByOwnerUserID = `
SELECT id, name, slug, owner_user_id, plan, status, stripe_customer_id, trial_ends_at, settings, created_at, updated_at, deleted_at
FROM accounts
WHERE owner_user_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
`

// GetAccountsByOwnerUserID retrieves all accounts owned by a user
func (s *Store) GetAccountsByOwnerUserID(ctx context.Context, userID uuid.UUID) ([]Account, error) {
	var accounts []Account
	err := s.db.SelectContext(ctx, &accounts, sqlGetAccountsByOwnerUserID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts by owner user id: %w", err)
	}
	return accounts, nil
}

const sqlUpdateAccount = `
UPDATE accounts
SET name = COALESCE($2, name),
    plan = COALESCE($3, plan),
    status = COALESCE($4, status),
    stripe_customer_id = COALESCE($5, stripe_customer_id),
    settings = COALESCE($6, settings),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, name, slug, owner_user_id, plan, status, stripe_customer_id, trial_ends_at, settings, created_at, updated_at, deleted_at
`

// UpdateAccount updates an account
func (s *Store) UpdateAccount(ctx context.Context, accountID uuid.UUID, params UpdateAccountParams) (Account, error) {
	var account Account
	err := s.db.GetContext(ctx, &account, sqlUpdateAccount,
		accountID,
		params.Name,
		params.Plan,
		params.Status,
		params.StripeCustomerID,
		params.Settings)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Account{}, ErrNotFound
		}
		return Account{}, fmt.Errorf("failed to update account: %w", err)
	}
	return account, nil
}

const sqlDeleteAccount = `
UPDATE accounts
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// DeleteAccount soft deletes an account
func (s *Store) DeleteAccount(ctx context.Context, accountID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, sqlDeleteAccount, accountID)
	if err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

const sqlUpdateAccountStripeCustomerID = `
UPDATE accounts
SET stripe_customer_id = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// UpdateAccountStripeCustomerID updates the Stripe customer ID for an account
func (s *Store) UpdateAccountStripeCustomerID(ctx context.Context, accountID uuid.UUID, stripeCustomerID string) error {
	res, err := s.db.ExecContext(ctx, sqlUpdateAccountStripeCustomerID, accountID, stripeCustomerID)
	if err != nil {
		return fmt.Errorf("failed to update account stripe customer id: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// Team Member operations

// CreateTeamMemberParams represents parameters for adding a team member
type CreateTeamMemberParams struct {
	AccountID   uuid.UUID
	UserID      uuid.UUID
	Role        string
	Permissions JSONB
	InvitedBy   *uuid.UUID
}

const sqlCreateTeamMember = `
INSERT INTO team_members (account_id, user_id, role, permissions, invited_by, invited_at)
VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
RETURNING id, account_id, user_id, role, permissions, invited_by, invited_at, joined_at, created_at, updated_at
`

// CreateTeamMember adds a team member to an account
func (s *Store) CreateTeamMember(ctx context.Context, params CreateTeamMemberParams) (TeamMember, error) {
	var teamMember TeamMember
	err := s.db.GetContext(ctx, &teamMember, sqlCreateTeamMember,
		params.AccountID,
		params.UserID,
		params.Role,
		params.Permissions,
		params.InvitedBy)
	if err != nil {
		return TeamMember{}, fmt.Errorf("failed to create team member: %w", err)
	}
	return teamMember, nil
}

const sqlGetTeamMembersByAccountID = `
SELECT id, account_id, user_id, role, permissions, invited_by, invited_at, joined_at, created_at, updated_at
FROM team_members
WHERE account_id = $1
ORDER BY created_at DESC
`

// GetTeamMembersByAccountID retrieves all team members for an account
func (s *Store) GetTeamMembersByAccountID(ctx context.Context, accountID uuid.UUID) ([]TeamMember, error) {
	var members []TeamMember
	err := s.db.SelectContext(ctx, &members, sqlGetTeamMembersByAccountID, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}
	return members, nil
}

const sqlGetTeamMemberByAccountAndUserID = `
SELECT id, account_id, user_id, role, permissions, invited_by, invited_at, joined_at, created_at, updated_at
FROM team_members
WHERE account_id = $1 AND user_id = $2
`

// GetTeamMemberByAccountAndUserID retrieves a specific team member
func (s *Store) GetTeamMemberByAccountAndUserID(ctx context.Context, accountID, userID uuid.UUID) (TeamMember, error) {
	var member TeamMember
	err := s.db.GetContext(ctx, &member, sqlGetTeamMemberByAccountAndUserID, accountID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TeamMember{}, ErrNotFound
		}
		return TeamMember{}, fmt.Errorf("failed to get team member: %w", err)
	}
	return member, nil
}

const sqlUpdateTeamMemberRole = `
UPDATE team_members
SET role = $3, updated_at = CURRENT_TIMESTAMP
WHERE account_id = $1 AND user_id = $2
`

// UpdateTeamMemberRole updates a team member's role
func (s *Store) UpdateTeamMemberRole(ctx context.Context, accountID, userID uuid.UUID, role string) error {
	res, err := s.db.ExecContext(ctx, sqlUpdateTeamMemberRole, accountID, userID, role)
	if err != nil {
		return fmt.Errorf("failed to update team member role: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

const sqlDeleteTeamMember = `
DELETE FROM team_members
WHERE account_id = $1 AND user_id = $2
`

// DeleteTeamMember removes a team member from an account
func (s *Store) DeleteTeamMember(ctx context.Context, accountID, userID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, sqlDeleteTeamMember, accountID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete team member: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}
