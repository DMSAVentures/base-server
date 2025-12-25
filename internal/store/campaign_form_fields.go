package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// CreateCampaignFormFieldParams represents parameters for creating a form field
type CreateCampaignFormFieldParams struct {
	CampaignID        uuid.UUID
	Name              string
	FieldType         FormFieldType
	Label             string
	Placeholder       *string
	Required          bool
	ValidationPattern *string
	Options           StringArray
	DisplayOrder      int
}

// UpdateCampaignFormFieldParams represents parameters for updating a form field
type UpdateCampaignFormFieldParams struct {
	Name              *string
	FieldType         *FormFieldType
	Label             *string
	Placeholder       *string
	Required          *bool
	ValidationPattern *string
	Options           StringArray
	DisplayOrder      *int
}

const sqlCreateCampaignFormField = `
INSERT INTO campaign_form_fields (campaign_id, name, field_type, label, placeholder, required, validation_pattern, options, display_order)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, campaign_id, name, field_type, label, placeholder, required, validation_pattern, options, display_order, created_at, updated_at
`

// CreateCampaignFormField creates a form field for a campaign
func (s *Store) CreateCampaignFormField(ctx context.Context, params CreateCampaignFormFieldParams) (CampaignFormField, error) {
	var field CampaignFormField
	err := s.db.GetContext(ctx, &field, sqlCreateCampaignFormField,
		params.CampaignID,
		params.Name,
		params.FieldType,
		params.Label,
		params.Placeholder,
		params.Required,
		params.ValidationPattern,
		params.Options,
		params.DisplayOrder)
	if err != nil {
		s.logger.Error(ctx, "failed to create campaign form field", err)
		return CampaignFormField{}, fmt.Errorf("failed to create campaign form field: %w", err)
	}
	return field, nil
}

const sqlGetCampaignFormFieldByID = `
SELECT id, campaign_id, name, field_type, label, placeholder, required, validation_pattern, options, display_order, created_at, updated_at
FROM campaign_form_fields
WHERE id = $1
`

// GetCampaignFormFieldByID retrieves a form field by ID
func (s *Store) GetCampaignFormFieldByID(ctx context.Context, fieldID uuid.UUID) (CampaignFormField, error) {
	var field CampaignFormField
	err := s.db.GetContext(ctx, &field, sqlGetCampaignFormFieldByID, fieldID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignFormField{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get campaign form field by id", err)
		return CampaignFormField{}, fmt.Errorf("failed to get campaign form field by id: %w", err)
	}
	return field, nil
}

const sqlGetCampaignFormFields = `
SELECT id, campaign_id, name, field_type, label, placeholder, required, validation_pattern, options, display_order, created_at, updated_at
FROM campaign_form_fields
WHERE campaign_id = $1
ORDER BY display_order ASC
`

// GetCampaignFormFields retrieves all form fields for a campaign
func (s *Store) GetCampaignFormFields(ctx context.Context, campaignID uuid.UUID) ([]CampaignFormField, error) {
	var fields []CampaignFormField
	err := s.db.SelectContext(ctx, &fields, sqlGetCampaignFormFields, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to get campaign form fields", err)
		return nil, fmt.Errorf("failed to get campaign form fields: %w", err)
	}
	return fields, nil
}

const sqlUpdateCampaignFormField = `
UPDATE campaign_form_fields
SET name = COALESCE($2, name),
    field_type = COALESCE($3, field_type),
    label = COALESCE($4, label),
    placeholder = COALESCE($5, placeholder),
    required = COALESCE($6, required),
    validation_pattern = COALESCE($7, validation_pattern),
    options = COALESCE($8, options),
    display_order = COALESCE($9, display_order),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, campaign_id, name, field_type, label, placeholder, required, validation_pattern, options, display_order, created_at, updated_at
`

// UpdateCampaignFormField updates a form field
func (s *Store) UpdateCampaignFormField(ctx context.Context, fieldID uuid.UUID, params UpdateCampaignFormFieldParams) (CampaignFormField, error) {
	var field CampaignFormField
	err := s.db.GetContext(ctx, &field, sqlUpdateCampaignFormField,
		fieldID,
		params.Name,
		params.FieldType,
		params.Label,
		params.Placeholder,
		params.Required,
		params.ValidationPattern,
		params.Options,
		params.DisplayOrder)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignFormField{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to update campaign form field", err)
		return CampaignFormField{}, fmt.Errorf("failed to update campaign form field: %w", err)
	}
	return field, nil
}

const sqlDeleteCampaignFormField = `
DELETE FROM campaign_form_fields WHERE id = $1
`

// DeleteCampaignFormField deletes a form field
func (s *Store) DeleteCampaignFormField(ctx context.Context, fieldID uuid.UUID) error {
	result, err := s.db.ExecContext(ctx, sqlDeleteCampaignFormField, fieldID)
	if err != nil {
		s.logger.Error(ctx, "failed to delete campaign form field", err)
		return fmt.Errorf("failed to delete campaign form field: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		s.logger.Error(ctx, "failed to get rows affected", err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

const sqlDeleteCampaignFormFieldsByCampaignID = `
DELETE FROM campaign_form_fields WHERE campaign_id = $1
`

// DeleteCampaignFormFieldsByCampaignID deletes all form fields for a campaign
func (s *Store) DeleteCampaignFormFieldsByCampaignID(ctx context.Context, campaignID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlDeleteCampaignFormFieldsByCampaignID, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to delete campaign form fields", err)
		return fmt.Errorf("failed to delete campaign form fields: %w", err)
	}
	return nil
}

// BulkCreateCampaignFormFields creates multiple form fields in a transaction
func (s *Store) BulkCreateCampaignFormFields(ctx context.Context, fields []CreateCampaignFormFieldParams) ([]CampaignFormField, error) {
	if len(fields) == 0 {
		return []CampaignFormField{}, nil
	}

	result := make([]CampaignFormField, 0, len(fields))
	for _, params := range fields {
		field, err := s.CreateCampaignFormField(ctx, params)
		if err != nil {
			return nil, err
		}
		result = append(result, field)
	}

	return result, nil
}

// ReplaceCampaignFormFields deletes all existing fields and creates new ones
func (s *Store) ReplaceCampaignFormFields(ctx context.Context, campaignID uuid.UUID, fields []CreateCampaignFormFieldParams) ([]CampaignFormField, error) {
	// Delete existing fields
	err := s.DeleteCampaignFormFieldsByCampaignID(ctx, campaignID)
	if err != nil {
		return nil, err
	}

	// Create new fields
	return s.BulkCreateCampaignFormFields(ctx, fields)
}
