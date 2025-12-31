package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// CreateBlastEmailTemplateParams represents parameters for creating a blast email template
type CreateBlastEmailTemplateParams struct {
	AccountID  uuid.UUID
	Name       string
	Subject    string
	HTMLBody   string
	BlocksJSON *JSONB
}

const sqlCreateBlastEmailTemplate = `
INSERT INTO blast_email_templates (account_id, name, subject, html_body, blocks_json)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, account_id, name, subject, html_body, blocks_json, created_at, updated_at, deleted_at
`

// CreateBlastEmailTemplate creates a new blast email template
func (s *Store) CreateBlastEmailTemplate(ctx context.Context, params CreateBlastEmailTemplateParams) (BlastEmailTemplate, error) {
	var template BlastEmailTemplate
	err := s.db.GetContext(ctx, &template, sqlCreateBlastEmailTemplate,
		params.AccountID,
		params.Name,
		params.Subject,
		params.HTMLBody,
		params.BlocksJSON)
	if err != nil {
		return BlastEmailTemplate{}, fmt.Errorf("failed to create blast email template: %w", err)
	}
	return template, nil
}

const sqlGetBlastEmailTemplateByID = `
SELECT id, account_id, name, subject, html_body, blocks_json, created_at, updated_at, deleted_at
FROM blast_email_templates
WHERE id = $1 AND deleted_at IS NULL
`

// GetBlastEmailTemplateByID retrieves a blast email template by ID
func (s *Store) GetBlastEmailTemplateByID(ctx context.Context, templateID uuid.UUID) (BlastEmailTemplate, error) {
	var template BlastEmailTemplate
	err := s.db.GetContext(ctx, &template, sqlGetBlastEmailTemplateByID, templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BlastEmailTemplate{}, ErrNotFound
		}
		return BlastEmailTemplate{}, fmt.Errorf("failed to get blast email template: %w", err)
	}
	return template, nil
}

const sqlGetBlastEmailTemplatesByAccount = `
SELECT id, account_id, name, subject, html_body, blocks_json, created_at, updated_at, deleted_at
FROM blast_email_templates
WHERE account_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
`

// GetBlastEmailTemplatesByAccount retrieves all blast email templates for an account
func (s *Store) GetBlastEmailTemplatesByAccount(ctx context.Context, accountID uuid.UUID) ([]BlastEmailTemplate, error) {
	var templates []BlastEmailTemplate
	err := s.db.SelectContext(ctx, &templates, sqlGetBlastEmailTemplatesByAccount, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get blast email templates: %w", err)
	}
	return templates, nil
}

const sqlUpdateBlastEmailTemplate = `
UPDATE blast_email_templates
SET name = COALESCE($2, name),
    subject = COALESCE($3, subject),
    html_body = COALESCE($4, html_body),
    blocks_json = COALESCE($5, blocks_json),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, account_id, name, subject, html_body, blocks_json, created_at, updated_at, deleted_at
`

// UpdateBlastEmailTemplateParams represents parameters for updating a blast email template
type UpdateBlastEmailTemplateParams struct {
	Name       *string
	Subject    *string
	HTMLBody   *string
	BlocksJSON *JSONB
}

// UpdateBlastEmailTemplate updates a blast email template
func (s *Store) UpdateBlastEmailTemplate(ctx context.Context, templateID uuid.UUID, params UpdateBlastEmailTemplateParams) (BlastEmailTemplate, error) {
	var template BlastEmailTemplate
	err := s.db.GetContext(ctx, &template, sqlUpdateBlastEmailTemplate,
		templateID,
		params.Name,
		params.Subject,
		params.HTMLBody,
		params.BlocksJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BlastEmailTemplate{}, ErrNotFound
		}
		return BlastEmailTemplate{}, fmt.Errorf("failed to update blast email template: %w", err)
	}
	return template, nil
}

const sqlDeleteBlastEmailTemplate = `
UPDATE blast_email_templates
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// DeleteBlastEmailTemplate soft deletes a blast email template
func (s *Store) DeleteBlastEmailTemplate(ctx context.Context, templateID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, sqlDeleteBlastEmailTemplate, templateID)
	if err != nil {
		return fmt.Errorf("failed to delete blast email template: %w", err)
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

const sqlCountBlastEmailTemplatesByAccount = `
SELECT COUNT(*) FROM blast_email_templates
WHERE account_id = $1 AND deleted_at IS NULL
`

// CountBlastEmailTemplatesByAccount counts blast email templates for an account
func (s *Store) CountBlastEmailTemplatesByAccount(ctx context.Context, accountID uuid.UUID) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count, sqlCountBlastEmailTemplatesByAccount, accountID)
	if err != nil {
		return 0, fmt.Errorf("failed to count blast email templates: %w", err)
	}
	return count, nil
}

const sqlGetBlastEmailTemplateByName = `
SELECT id, account_id, name, subject, html_body, blocks_json, created_at, updated_at, deleted_at
FROM blast_email_templates
WHERE account_id = $1 AND name = $2 AND deleted_at IS NULL
`

// GetBlastEmailTemplateByName retrieves a blast email template by account and name
func (s *Store) GetBlastEmailTemplateByName(ctx context.Context, accountID uuid.UUID, name string) (BlastEmailTemplate, error) {
	var template BlastEmailTemplate
	err := s.db.GetContext(ctx, &template, sqlGetBlastEmailTemplateByName, accountID, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BlastEmailTemplate{}, ErrNotFound
		}
		return BlastEmailTemplate{}, fmt.Errorf("failed to get blast email template by name: %w", err)
	}
	return template, nil
}
