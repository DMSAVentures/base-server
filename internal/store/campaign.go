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
	PrivacyPolicyURL *string
	TermsURL         *string
	MaxSignups       *int
}

const sqlCreateCampaign = `
INSERT INTO campaigns (account_id, name, slug, description, type, privacy_policy_url, terms_url, max_signups)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, account_id, name, slug, description, status, type, launch_date, end_date, privacy_policy_url, terms_url, max_signups, total_signups, total_verified, total_referrals, created_at, updated_at, deleted_at
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
		params.PrivacyPolicyURL,
		params.TermsURL,
		params.MaxSignups)
	if err != nil {
		return Campaign{}, fmt.Errorf("failed to create campaign: %w", err)
	}
	return campaign, nil
}

const sqlGetCampaignByID = `
SELECT
    c.id, c.account_id, c.name, c.slug, c.description, c.status, c.type,
    c.launch_date, c.end_date, c.privacy_policy_url, c.terms_url, c.max_signups,
    COALESCE(COUNT(w.id), 0)::int as total_signups,
    COALESCE(COUNT(w.id) FILTER (WHERE w.email_verified = true), 0)::int as total_verified,
    COALESCE(COUNT(w.id) FILTER (WHERE w.referred_by_id IS NOT NULL), 0)::int as total_referrals,
    c.created_at, c.updated_at, c.deleted_at
FROM campaigns c
LEFT JOIN waitlist_users w ON w.campaign_id = c.id AND w.deleted_at IS NULL
WHERE c.id = $1 AND c.deleted_at IS NULL
GROUP BY c.id
`

// GetCampaignByID retrieves a campaign by ID
func (s *Store) GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (Campaign, error) {
	var campaign Campaign
	err := s.db.GetContext(ctx, &campaign, sqlGetCampaignByID, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Campaign{}, ErrNotFound
		}
		return Campaign{}, fmt.Errorf("failed to get campaign by id: %w", err)
	}
	return campaign, nil
}

const sqlGetCampaignBySlug = `
SELECT
    c.id, c.account_id, c.name, c.slug, c.description, c.status, c.type,
    c.launch_date, c.end_date, c.privacy_policy_url, c.terms_url, c.max_signups,
    COALESCE(COUNT(w.id), 0)::int as total_signups,
    COALESCE(COUNT(w.id) FILTER (WHERE w.email_verified = true), 0)::int as total_verified,
    COALESCE(COUNT(w.id) FILTER (WHERE w.referred_by_id IS NOT NULL), 0)::int as total_referrals,
    c.created_at, c.updated_at, c.deleted_at
FROM campaigns c
LEFT JOIN waitlist_users w ON w.campaign_id = c.id AND w.deleted_at IS NULL
WHERE c.account_id = $1 AND c.slug = $2 AND c.deleted_at IS NULL
GROUP BY c.id
`

// GetCampaignBySlug retrieves a campaign by account ID and slug
func (s *Store) GetCampaignBySlug(ctx context.Context, accountID uuid.UUID, slug string) (Campaign, error) {
	var campaign Campaign
	err := s.db.GetContext(ctx, &campaign, sqlGetCampaignBySlug, accountID, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Campaign{}, ErrNotFound
		}
		return Campaign{}, fmt.Errorf("failed to get campaign by slug: %w", err)
	}
	return campaign, nil
}

const sqlGetCampaignsByAccountID = `
SELECT
    c.id, c.account_id, c.name, c.slug, c.description, c.status, c.type,
    c.launch_date, c.end_date, c.privacy_policy_url, c.terms_url, c.max_signups,
    COALESCE(COUNT(w.id), 0)::int as total_signups,
    COALESCE(COUNT(w.id) FILTER (WHERE w.email_verified = true), 0)::int as total_verified,
    COALESCE(COUNT(w.id) FILTER (WHERE w.referred_by_id IS NOT NULL), 0)::int as total_referrals,
    c.created_at, c.updated_at, c.deleted_at
FROM campaigns c
LEFT JOIN waitlist_users w ON w.campaign_id = c.id AND w.deleted_at IS NULL
WHERE c.account_id = $1 AND c.deleted_at IS NULL
GROUP BY c.id
ORDER BY c.created_at DESC
`

// GetCampaignsByAccountID retrieves all campaigns for an account
func (s *Store) GetCampaignsByAccountID(ctx context.Context, accountID uuid.UUID) ([]Campaign, error) {
	var campaigns []Campaign
	err := s.db.SelectContext(ctx, &campaigns, sqlGetCampaignsByAccountID, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get campaigns by account id: %w", err)
	}
	return campaigns, nil
}

const sqlGetCampaignsByStatus = `
SELECT
    c.id, c.account_id, c.name, c.slug, c.description, c.status, c.type,
    c.launch_date, c.end_date, c.privacy_policy_url, c.terms_url, c.max_signups,
    COALESCE(COUNT(w.id), 0)::int as total_signups,
    COALESCE(COUNT(w.id) FILTER (WHERE w.email_verified = true), 0)::int as total_verified,
    COALESCE(COUNT(w.id) FILTER (WHERE w.referred_by_id IS NOT NULL), 0)::int as total_referrals,
    c.created_at, c.updated_at, c.deleted_at
FROM campaigns c
LEFT JOIN waitlist_users w ON w.campaign_id = c.id AND w.deleted_at IS NULL
WHERE c.account_id = $1 AND c.status = $2 AND c.deleted_at IS NULL
GROUP BY c.id
ORDER BY c.created_at DESC
`

// GetCampaignsByStatus retrieves campaigns by account ID and status
func (s *Store) GetCampaignsByStatus(ctx context.Context, accountID uuid.UUID, status string) ([]Campaign, error) {
	var campaigns []Campaign
	err := s.db.SelectContext(ctx, &campaigns, sqlGetCampaignsByStatus, accountID, status)
	if err != nil {
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
	query := `SELECT
	          c.id, c.account_id, c.name, c.slug, c.description, c.status, c.type,
	          c.launch_date, c.end_date, c.privacy_policy_url, c.terms_url, c.max_signups,
	          COALESCE(COUNT(w.id), 0)::int as total_signups,
	          COALESCE(COUNT(w.id) FILTER (WHERE w.email_verified = true), 0)::int as total_verified,
	          COALESCE(COUNT(w.id) FILTER (WHERE w.referred_by_id IS NOT NULL), 0)::int as total_referrals,
	          c.created_at, c.updated_at, c.deleted_at
	          FROM campaigns c
	          LEFT JOIN waitlist_users w ON w.campaign_id = c.id AND w.deleted_at IS NULL
	          WHERE c.account_id = $1 AND c.deleted_at IS NULL`
	countQuery := `SELECT COUNT(*) FROM campaigns WHERE account_id = $1 AND deleted_at IS NULL`

	args := []interface{}{params.AccountID}
	argCount := 1

	// Add filters
	if params.Status != nil {
		argCount++
		query += fmt.Sprintf(" AND c.status = $%d", argCount)
		countQuery += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, *params.Status)
	}

	if params.Type != nil {
		argCount++
		query += fmt.Sprintf(" AND c.type = $%d", argCount)
		countQuery += fmt.Sprintf(" AND type = $%d", argCount)
		args = append(args, *params.Type)
	}

	// Get total count
	var totalCount int
	err := s.db.GetContext(ctx, &totalCount, countQuery, args...)
	if err != nil {
		return ListCampaignsResult{}, fmt.Errorf("failed to get total campaign count: %w", err)
	}

	// Add GROUP BY and pagination
	offset := (params.Page - 1) * params.Limit
	query += fmt.Sprintf(" GROUP BY c.id ORDER BY c.created_at DESC LIMIT $%d OFFSET $%d", argCount+1, argCount+2)
	args = append(args, params.Limit, offset)

	// Get campaigns
	var campaigns []Campaign
	err = s.db.SelectContext(ctx, &campaigns, query, args...)
	if err != nil {
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
SET name = COALESCE($3, name),
    description = COALESCE($4, description),
    status = COALESCE($5, status),
    launch_date = COALESCE($6, launch_date),
    end_date = COALESCE($7, end_date),
    privacy_policy_url = COALESCE($8, privacy_policy_url),
    terms_url = COALESCE($9, terms_url),
    max_signups = COALESCE($10, max_signups),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND account_id = $2 AND deleted_at IS NULL
RETURNING id, account_id, name, slug, description, status, type, launch_date, end_date, privacy_policy_url, terms_url, max_signups, total_signups, total_verified, total_referrals, created_at, updated_at, deleted_at
`

// UpdateCampaign updates a campaign
func (s *Store) UpdateCampaign(ctx context.Context, accountID, campaignID uuid.UUID, params UpdateCampaignParams) (Campaign, error) {
	var campaign Campaign
	err := s.db.GetContext(ctx, &campaign, sqlUpdateCampaign,
		campaignID,
		accountID,
		params.Name,
		params.Description,
		params.Status,
		params.LaunchDate,
		params.EndDate,
		params.PrivacyPolicyURL,
		params.TermsURL,
		params.MaxSignups)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Campaign{}, ErrNotFound
		}
		return Campaign{}, fmt.Errorf("failed to update campaign: %w", err)
	}
	return campaign, nil
}

const sqlUpdateCampaignStatus = `
UPDATE campaigns
SET status = $3, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND account_id = $2 AND deleted_at IS NULL
RETURNING id, account_id, name, slug, description, status, type, launch_date, end_date, privacy_policy_url, terms_url, max_signups, total_signups, total_verified, total_referrals, created_at, updated_at, deleted_at
`

// UpdateCampaignStatus updates a campaign's status
func (s *Store) UpdateCampaignStatus(ctx context.Context, accountID, campaignID uuid.UUID, status string) (Campaign, error) {
	var campaign Campaign
	err := s.db.GetContext(ctx, &campaign, sqlUpdateCampaignStatus, campaignID, accountID, status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Campaign{}, ErrNotFound
		}
		return Campaign{}, fmt.Errorf("failed to update campaign status: %w", err)
	}
	return campaign, nil
}

const sqlDeleteCampaign = `
UPDATE campaigns
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1 AND account_id = $2 AND deleted_at IS NULL
`

// DeleteCampaign soft deletes a campaign
func (s *Store) DeleteCampaign(ctx context.Context, accountID, campaignID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, sqlDeleteCampaign, campaignID, accountID)
	if err != nil {
		return fmt.Errorf("failed to delete campaign: %w", err)
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

const sqlIncrementCampaignSignups = `
UPDATE campaigns
SET total_signups = total_signups + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// IncrementCampaignSignups increments the total signups counter
func (s *Store) IncrementCampaignSignups(ctx context.Context, campaignID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlIncrementCampaignSignups, campaignID)
	if err != nil {
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
		return fmt.Errorf("failed to increment campaign referrals: %w", err)
	}
	return nil
}

// GetCampaignWithSettings retrieves a campaign with all its related settings
func (s *Store) GetCampaignWithSettings(ctx context.Context, campaignID uuid.UUID) (Campaign, error) {
	campaign, err := s.GetCampaignByID(ctx, campaignID)
	if err != nil {
		return Campaign{}, err
	}

	return s.loadCampaignSettings(ctx, campaign)
}

// GetCampaignBySlugWithSettings retrieves a campaign by slug with all its related settings
func (s *Store) GetCampaignBySlugWithSettings(ctx context.Context, accountID uuid.UUID, slug string) (Campaign, error) {
	campaign, err := s.GetCampaignBySlug(ctx, accountID, slug)
	if err != nil {
		return Campaign{}, err
	}

	return s.loadCampaignSettings(ctx, campaign)
}

// loadCampaignSettings loads all related settings for a campaign
func (s *Store) loadCampaignSettings(ctx context.Context, campaign Campaign) (Campaign, error) {
	// Load email settings
	emailSettings, err := s.GetCampaignEmailSettings(ctx, campaign.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Campaign{}, fmt.Errorf("failed to load email settings: %w", err)
	}
	if err == nil {
		campaign.EmailSettings = &emailSettings
	}

	// Load branding settings
	brandingSettings, err := s.GetCampaignBrandingSettings(ctx, campaign.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Campaign{}, fmt.Errorf("failed to load branding settings: %w", err)
	}
	if err == nil {
		campaign.BrandingSettings = &brandingSettings
	}

	// Load form settings
	formSettings, err := s.GetCampaignFormSettings(ctx, campaign.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Campaign{}, fmt.Errorf("failed to load form settings: %w", err)
	}
	if err == nil {
		campaign.FormSettings = &formSettings
	}

	// Load referral settings
	referralSettings, err := s.GetCampaignReferralSettings(ctx, campaign.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Campaign{}, fmt.Errorf("failed to load referral settings: %w", err)
	}
	if err == nil {
		campaign.ReferralSettings = &referralSettings
	}

	// Load form fields
	formFields, err := s.GetCampaignFormFields(ctx, campaign.ID)
	if err != nil {
		return Campaign{}, fmt.Errorf("failed to load form fields: %w", err)
	}
	campaign.FormFields = formFields

	// Load share messages
	shareMessages, err := s.GetCampaignShareMessages(ctx, campaign.ID)
	if err != nil {
		return Campaign{}, fmt.Errorf("failed to load share messages: %w", err)
	}
	campaign.ShareMessages = shareMessages

	// Load tracking integrations
	trackingIntegrations, err := s.GetCampaignTrackingIntegrations(ctx, campaign.ID)
	if err != nil {
		return Campaign{}, fmt.Errorf("failed to load tracking integrations: %w", err)
	}
	campaign.TrackingIntegrations = trackingIntegrations

	return campaign, nil
}

// LoadCampaignsSettings loads settings for multiple campaigns
func (s *Store) LoadCampaignsSettings(ctx context.Context, campaigns []Campaign) ([]Campaign, error) {
	result := make([]Campaign, len(campaigns))
	for i, c := range campaigns {
		loaded, err := s.loadCampaignSettings(ctx, c)
		if err != nil {
			return nil, err
		}
		result[i] = loaded
	}
	return result, nil
}
