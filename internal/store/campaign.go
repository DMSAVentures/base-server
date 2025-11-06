package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateCampaignParams represents parameters for creating a campaign
type CreateCampaignParams struct {
	AccountID        uuid.UUID
	Name             string
	Slug             string
	Description      *string
	Type             string
	FormConfig       JSONB
	ReferralConfig   JSONB
	EmailConfig      JSONB
	BrandingConfig   JSONB
	PrivacyPolicyURL *string
	TermsURL         *string
	MaxSignups       *int
}

// UpdateCampaignParams represents parameters for updating a campaign
type UpdateCampaignParams struct {
	Name             *string
	Description      *string
	Status           *string
	LaunchDate       *time.Time
	EndDate          *time.Time
	FormConfig       JSONB
	ReferralConfig   JSONB
	EmailConfig      JSONB
	BrandingConfig   JSONB
	PrivacyPolicyURL *string
	TermsURL         *string
	MaxSignups       *int
}

const sqlCreateCampaign = `
INSERT INTO campaigns (account_id, name, slug, description, type, form_config, referral_config, email_config, branding_config, privacy_policy_url, terms_url, max_signups)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id, account_id, name, slug, description, status, type, launch_date, end_date, form_config, referral_config, email_config, branding_config, privacy_policy_url, terms_url, max_signups, total_signups, total_verified, total_referrals, created_at, updated_at, deleted_at
`

// CreateCampaign creates a new campaign
func (s *Store) CreateCampaign(ctx context.Context, params CreateCampaignParams) (Campaign, error) {
	var campaign Campaign
	err := s.db.GetContext(ctx, &campaign, sqlCreateCampaign,
		params.AccountID,
		params.Name,
		params.Slug,
		params.Description,
		params.Type,
		params.FormConfig,
		params.ReferralConfig,
		params.EmailConfig,
		params.BrandingConfig,
		params.PrivacyPolicyURL,
		params.TermsURL,
		params.MaxSignups)
	if err != nil {
		s.logger.Error(ctx, "failed to create campaign", err)
		return Campaign{}, fmt.Errorf("failed to create campaign: %w", err)
	}
	return campaign, nil
}

const sqlGetCampaignByID = `
SELECT id, account_id, name, slug, description, status, type, launch_date, end_date, form_config, referral_config, email_config, branding_config, privacy_policy_url, terms_url, max_signups, total_signups, total_verified, total_referrals, created_at, updated_at, deleted_at
FROM campaigns
WHERE id = $1 AND deleted_at IS NULL
`

// GetCampaignByID retrieves a campaign by ID
func (s *Store) GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (Campaign, error) {
	var campaign Campaign
	err := s.db.GetContext(ctx, &campaign, sqlGetCampaignByID, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Campaign{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get campaign by id", err)
		return Campaign{}, fmt.Errorf("failed to get campaign by id: %w", err)
	}
	return campaign, nil
}

const sqlGetCampaignBySlug = `
SELECT id, account_id, name, slug, description, status, type, launch_date, end_date, form_config, referral_config, email_config, branding_config, privacy_policy_url, terms_url, max_signups, total_signups, total_verified, total_referrals, created_at, updated_at, deleted_at
FROM campaigns
WHERE account_id = $1 AND slug = $2 AND deleted_at IS NULL
`

// GetCampaignBySlug retrieves a campaign by account ID and slug
func (s *Store) GetCampaignBySlug(ctx context.Context, accountID uuid.UUID, slug string) (Campaign, error) {
	var campaign Campaign
	err := s.db.GetContext(ctx, &campaign, sqlGetCampaignBySlug, accountID, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Campaign{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get campaign by slug", err)
		return Campaign{}, fmt.Errorf("failed to get campaign by slug: %w", err)
	}
	return campaign, nil
}

const sqlGetCampaignsByAccountID = `
SELECT id, account_id, name, slug, description, status, type, launch_date, end_date, form_config, referral_config, email_config, branding_config, privacy_policy_url, terms_url, max_signups, total_signups, total_verified, total_referrals, created_at, updated_at, deleted_at
FROM campaigns
WHERE account_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
`

// GetCampaignsByAccountID retrieves all campaigns for an account
func (s *Store) GetCampaignsByAccountID(ctx context.Context, accountID uuid.UUID) ([]Campaign, error) {
	var campaigns []Campaign
	err := s.db.SelectContext(ctx, &campaigns, sqlGetCampaignsByAccountID, accountID)
	if err != nil {
		s.logger.Error(ctx, "failed to get campaigns by account id", err)
		return nil, fmt.Errorf("failed to get campaigns by account id: %w", err)
	}
	return campaigns, nil
}

const sqlGetCampaignsByStatus = `
SELECT id, account_id, name, slug, description, status, type, launch_date, end_date, form_config, referral_config, email_config, branding_config, privacy_policy_url, terms_url, max_signups, total_signups, total_verified, total_referrals, created_at, updated_at, deleted_at
FROM campaigns
WHERE account_id = $1 AND status = $2 AND deleted_at IS NULL
ORDER BY created_at DESC
`

// GetCampaignsByStatus retrieves campaigns by account ID and status
func (s *Store) GetCampaignsByStatus(ctx context.Context, accountID uuid.UUID, status string) ([]Campaign, error) {
	var campaigns []Campaign
	err := s.db.SelectContext(ctx, &campaigns, sqlGetCampaignsByStatus, accountID, status)
	if err != nil {
		s.logger.Error(ctx, "failed to get campaigns by status", err)
		return nil, fmt.Errorf("failed to get campaigns by status: %w", err)
	}
	return campaigns, nil
}

// ListCampaignsParams represents parameters for listing campaigns with filters
type ListCampaignsParams struct {
	AccountID uuid.UUID
	Status    *string
	Type      *string
	Page      int
	Limit     int
}

// ListCampaignsResult represents the result of listing campaigns with pagination
type ListCampaignsResult struct {
	Campaigns  []Campaign
	TotalCount int
	Page       int
	Limit      int
	TotalPages int
}

// ListCampaigns retrieves campaigns with pagination and filters
func (s *Store) ListCampaigns(ctx context.Context, params ListCampaignsParams) (ListCampaignsResult, error) {
	// Build dynamic query
	query := `SELECT id, account_id, name, slug, description, status, type, launch_date, end_date,
	          form_config, referral_config, email_config, branding_config, privacy_policy_url, terms_url,
	          max_signups, total_signups, total_verified, total_referrals, created_at, updated_at, deleted_at
	          FROM campaigns
	          WHERE account_id = $1 AND deleted_at IS NULL`
	countQuery := `SELECT COUNT(*) FROM campaigns WHERE account_id = $1 AND deleted_at IS NULL`

	args := []interface{}{params.AccountID}
	argCount := 1

	// Add filters
	if params.Status != nil {
		argCount++
		query += fmt.Sprintf(" AND status = $%d", argCount)
		countQuery += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, *params.Status)
	}

	if params.Type != nil {
		argCount++
		query += fmt.Sprintf(" AND type = $%d", argCount)
		countQuery += fmt.Sprintf(" AND type = $%d", argCount)
		args = append(args, *params.Type)
	}

	// Get total count
	var totalCount int
	err := s.db.GetContext(ctx, &totalCount, countQuery, args...)
	if err != nil {
		s.logger.Error(ctx, "failed to get total campaign count", err)
		return ListCampaignsResult{}, fmt.Errorf("failed to get total campaign count: %w", err)
	}

	// Add pagination
	offset := (params.Page - 1) * params.Limit
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argCount+1, argCount+2)
	args = append(args, params.Limit, offset)

	// Get campaigns
	var campaigns []Campaign
	err = s.db.SelectContext(ctx, &campaigns, query, args...)
	if err != nil {
		s.logger.Error(ctx, "failed to list campaigns", err)
		return ListCampaignsResult{}, fmt.Errorf("failed to list campaigns: %w", err)
	}

	totalPages := (totalCount + params.Limit - 1) / params.Limit

	return ListCampaignsResult{
		Campaigns:  campaigns,
		TotalCount: totalCount,
		Page:       params.Page,
		Limit:      params.Limit,
		TotalPages: totalPages,
	}, nil
}

const sqlUpdateCampaign = `
UPDATE campaigns
SET name = COALESCE($2, name),
    description = COALESCE($3, description),
    status = COALESCE($4, status),
    launch_date = COALESCE($5, launch_date),
    end_date = COALESCE($6, end_date),
    form_config = COALESCE($7, form_config),
    referral_config = COALESCE($8, referral_config),
    email_config = COALESCE($9, email_config),
    branding_config = COALESCE($10, branding_config),
    privacy_policy_url = COALESCE($11, privacy_policy_url),
    terms_url = COALESCE($12, terms_url),
    max_signups = COALESCE($13, max_signups),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, account_id, name, slug, description, status, type, launch_date, end_date, form_config, referral_config, email_config, branding_config, privacy_policy_url, terms_url, max_signups, total_signups, total_verified, total_referrals, created_at, updated_at, deleted_at
`

// UpdateCampaign updates a campaign
func (s *Store) UpdateCampaign(ctx context.Context, campaignID uuid.UUID, params UpdateCampaignParams) (Campaign, error) {
	var campaign Campaign
	err := s.db.GetContext(ctx, &campaign, sqlUpdateCampaign,
		campaignID,
		params.Name,
		params.Description,
		params.Status,
		params.LaunchDate,
		params.EndDate,
		params.FormConfig,
		params.ReferralConfig,
		params.EmailConfig,
		params.BrandingConfig,
		params.PrivacyPolicyURL,
		params.TermsURL,
		params.MaxSignups)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Campaign{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to update campaign", err)
		return Campaign{}, fmt.Errorf("failed to update campaign: %w", err)
	}
	return campaign, nil
}

const sqlUpdateCampaignStatus = `
UPDATE campaigns
SET status = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, account_id, name, slug, description, status, type, launch_date, end_date, form_config, referral_config, email_config, branding_config, privacy_policy_url, terms_url, max_signups, total_signups, total_verified, total_referrals, created_at, updated_at, deleted_at
`

// UpdateCampaignStatus updates a campaign's status
func (s *Store) UpdateCampaignStatus(ctx context.Context, campaignID uuid.UUID, status string) (Campaign, error) {
	var campaign Campaign
	err := s.db.GetContext(ctx, &campaign, sqlUpdateCampaignStatus, campaignID, status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Campaign{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to update campaign status", err)
		return Campaign{}, fmt.Errorf("failed to update campaign status: %w", err)
	}
	return campaign, nil
}

const sqlDeleteCampaign = `
UPDATE campaigns
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// DeleteCampaign soft deletes a campaign
func (s *Store) DeleteCampaign(ctx context.Context, campaignID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, sqlDeleteCampaign, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to delete campaign", err)
		return fmt.Errorf("failed to delete campaign: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		s.logger.Error(ctx, "failed to get rows affected", err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

const sqlIncrementCampaignSignups = `
UPDATE campaigns
SET total_signups = total_signups + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// IncrementCampaignSignups increments the total signups counter
func (s *Store) IncrementCampaignSignups(ctx context.Context, campaignID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlIncrementCampaignSignups, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to increment campaign signups", err)
		return fmt.Errorf("failed to increment campaign signups: %w", err)
	}
	return nil
}

const sqlIncrementCampaignVerified = `
UPDATE campaigns
SET total_verified = total_verified + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// IncrementCampaignVerified increments the total verified counter
func (s *Store) IncrementCampaignVerified(ctx context.Context, campaignID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlIncrementCampaignVerified, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to increment campaign verified", err)
		return fmt.Errorf("failed to increment campaign verified: %w", err)
	}
	return nil
}

const sqlIncrementCampaignReferrals = `
UPDATE campaigns
SET total_referrals = total_referrals + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// IncrementCampaignReferrals increments the total referrals counter
func (s *Store) IncrementCampaignReferrals(ctx context.Context, campaignID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlIncrementCampaignReferrals, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to increment campaign referrals", err)
		return fmt.Errorf("failed to increment campaign referrals: %w", err)
	}
	return nil
}
