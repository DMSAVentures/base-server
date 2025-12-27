package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateWaitlistUserParams represents parameters for creating a waitlist user
type CreateWaitlistUserParams struct {
	CampaignID        uuid.UUID
	Email             string
	FirstName         *string
	LastName          *string
	ReferralCode      string
	ReferredByID      *uuid.UUID
	Position          int
	OriginalPosition  int
	Source            *string
	UTMSource         *string
	UTMMedium         *string
	UTMCampaign       *string
	UTMTerm           *string
	UTMContent        *string
	IPAddress         *string
	UserAgent         *string
	CountryCode       *string
	City              *string
	DeviceFingerprint *string
	// CloudFront geographic data
	Country      *string
	Region       *string
	RegionCode   *string
	PostalCode   *string
	UserTimezone *string
	Latitude     *float64
	Longitude    *float64
	MetroCode    *string
	// CloudFront device detection (enum values)
	DeviceType *string // desktop, mobile, tablet, smarttv, unknown
	DeviceOS   *string // android, ios, other
	// CloudFront connection info
	ASN         *string
	TLSVersion  *string
	HTTPVersion *string

	Metadata JSONB
	MarketingConsent  bool
	TermsAccepted     bool
	VerificationToken *string
}

// UpdateWaitlistUserParams represents parameters for updating a waitlist user
type UpdateWaitlistUserParams struct {
	FirstName *string
	LastName  *string
	Status    *string
	Position  *int
	Points    *int
	Metadata  JSONB
}

const sqlCreateWaitlistUser = `
INSERT INTO waitlist_users (
	campaign_id, email, first_name, last_name, referral_code, referred_by_id, position, original_position,
	source, utm_source, utm_medium, utm_campaign, utm_term, utm_content,
	ip_address, user_agent, country_code, city, device_fingerprint,
	country, region, region_code, postal_code, user_timezone, latitude, longitude, metro_code,
	device_type, device_os, asn, tls_version, http_version,
	metadata, marketing_consent, terms_accepted, verification_token
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36)
RETURNING id, campaign_id, email, first_name, last_name, status, position, original_position, referral_code, referred_by_id, referral_count, verified_referral_count, points, email_verified, verification_token, verification_sent_at, verified_at, source, utm_source, utm_medium, utm_campaign, utm_term, utm_content, ip_address, user_agent, country_code, city, device_fingerprint, country, region, region_code, postal_code, user_timezone, latitude, longitude, metro_code, device_type, device_os, asn, tls_version, http_version, metadata, marketing_consent, marketing_consent_at, terms_accepted, terms_accepted_at, last_activity_at, share_count, created_at, updated_at, deleted_at
`

// CreateWaitlistUser creates a new waitlist user
func (s *Store) CreateWaitlistUser(ctx context.Context, params CreateWaitlistUserParams) (WaitlistUser, error) {
	var user WaitlistUser
	err := s.db.GetContext(ctx, &user, sqlCreateWaitlistUser,
		params.CampaignID,
		params.Email,
		params.FirstName,
		params.LastName,
		params.ReferralCode,
		params.ReferredByID,
		params.Position,
		params.OriginalPosition,
		params.Source,
		params.UTMSource,
		params.UTMMedium,
		params.UTMCampaign,
		params.UTMTerm,
		params.UTMContent,
		params.IPAddress,
		params.UserAgent,
		params.CountryCode,
		params.City,
		params.DeviceFingerprint,
		// CloudFront geographic data
		params.Country,
		params.Region,
		params.RegionCode,
		params.PostalCode,
		params.UserTimezone,
		params.Latitude,
		params.Longitude,
		params.MetroCode,
		// CloudFront device detection (enums)
		params.DeviceType,
		params.DeviceOS,
		// CloudFront connection info
		params.ASN,
		params.TLSVersion,
		params.HTTPVersion,
		params.Metadata,
		params.MarketingConsent,
		params.TermsAccepted,
		params.VerificationToken)
	if err != nil {
		return WaitlistUser{}, fmt.Errorf("failed to create waitlist user: %w", err)
	}
	return user, nil
}

// waitlistUserColumns contains all columns for SELECT queries
const waitlistUserColumns = `id, campaign_id, email, first_name, last_name, status, position, original_position, referral_code, referred_by_id, referral_count, verified_referral_count, points, email_verified, verification_token, verification_sent_at, verified_at, source, utm_source, utm_medium, utm_campaign, utm_term, utm_content, ip_address, user_agent, country_code, city, device_fingerprint, country, region, region_code, postal_code, user_timezone, latitude, longitude, metro_code, device_type, device_os, asn, tls_version, http_version, metadata, marketing_consent, marketing_consent_at, terms_accepted, terms_accepted_at, last_activity_at, share_count, created_at, updated_at, deleted_at`

const sqlGetWaitlistUserByID = `
SELECT ` + waitlistUserColumns + `
FROM waitlist_users
WHERE id = $1 AND deleted_at IS NULL
`

// GetWaitlistUserByID retrieves a waitlist user by ID
func (s *Store) GetWaitlistUserByID(ctx context.Context, userID uuid.UUID) (WaitlistUser, error) {
	var user WaitlistUser
	err := s.db.GetContext(ctx, &user, sqlGetWaitlistUserByID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WaitlistUser{}, ErrNotFound
		}
		return WaitlistUser{}, fmt.Errorf("failed to get waitlist user by id: %w", err)
	}
	return user, nil
}

const sqlGetWaitlistUserByEmail = `
SELECT ` + waitlistUserColumns + `
FROM waitlist_users
WHERE campaign_id = $1 AND email = $2 AND deleted_at IS NULL
`

// GetWaitlistUserByEmail retrieves a waitlist user by campaign ID and email
func (s *Store) GetWaitlistUserByEmail(ctx context.Context, campaignID uuid.UUID, email string) (WaitlistUser, error) {
	var user WaitlistUser
	err := s.db.GetContext(ctx, &user, sqlGetWaitlistUserByEmail, campaignID, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WaitlistUser{}, ErrNotFound
		}
		return WaitlistUser{}, fmt.Errorf("failed to get waitlist user by email: %w", err)
	}
	return user, nil
}

const sqlGetWaitlistUserByReferralCode = `
SELECT ` + waitlistUserColumns + `
FROM waitlist_users
WHERE referral_code = $1 AND deleted_at IS NULL
`

// GetWaitlistUserByReferralCode retrieves a waitlist user by referral code
func (s *Store) GetWaitlistUserByReferralCode(ctx context.Context, referralCode string) (WaitlistUser, error) {
	var user WaitlistUser
	err := s.db.GetContext(ctx, &user, sqlGetWaitlistUserByReferralCode, referralCode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WaitlistUser{}, ErrNotFound
		}
		return WaitlistUser{}, fmt.Errorf("failed to get waitlist user by referral code: %w", err)
	}
	return user, nil
}

const sqlGetWaitlistUsersByCampaign = `
SELECT ` + waitlistUserColumns + `
FROM waitlist_users
WHERE campaign_id = $1 AND deleted_at IS NULL
ORDER BY position ASC
LIMIT $2 OFFSET $3
`

// ListWaitlistUsersParams represents parameters for listing waitlist users
type ListWaitlistUsersParams struct {
	Limit  int
	Offset int
}

// GetWaitlistUsersByCampaign retrieves waitlist users for a campaign with pagination
func (s *Store) GetWaitlistUsersByCampaign(ctx context.Context, campaignID uuid.UUID, params ListWaitlistUsersParams) ([]WaitlistUser, error) {
	var users []WaitlistUser
	err := s.db.SelectContext(ctx, &users, sqlGetWaitlistUsersByCampaign, campaignID, params.Limit, params.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get waitlist users by campaign: %w", err)
	}
	return users, nil
}

const sqlGetWaitlistUsersByStatus = `
SELECT ` + waitlistUserColumns + `
FROM waitlist_users
WHERE campaign_id = $1 AND status = $2 AND deleted_at IS NULL
ORDER BY position ASC
`

// GetWaitlistUsersByStatus retrieves waitlist users by campaign ID and status
func (s *Store) GetWaitlistUsersByStatus(ctx context.Context, campaignID uuid.UUID, status string) ([]WaitlistUser, error) {
	var users []WaitlistUser
	err := s.db.SelectContext(ctx, &users, sqlGetWaitlistUsersByStatus, campaignID, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get waitlist users by status: %w", err)
	}
	return users, nil
}

const sqlCountWaitlistUsersByCampaign = `
SELECT COUNT(*)
FROM waitlist_users
WHERE campaign_id = $1 AND deleted_at IS NULL
`

// CountWaitlistUsersByCampaign counts total waitlist users for a campaign
func (s *Store) CountWaitlistUsersByCampaign(ctx context.Context, campaignID uuid.UUID) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count, sqlCountWaitlistUsersByCampaign, campaignID)
	if err != nil {
		return 0, fmt.Errorf("failed to count waitlist users: %w", err)
	}
	return count, nil
}

const sqlUpdateWaitlistUser = `
UPDATE waitlist_users
SET first_name = COALESCE($2, first_name),
    last_name = COALESCE($3, last_name),
    status = COALESCE($4, status),
    position = COALESCE($5, position),
    points = COALESCE($6, points),
    metadata = COALESCE($7, metadata),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
RETURNING ` + waitlistUserColumns

// UpdateWaitlistUser updates a waitlist user
func (s *Store) UpdateWaitlistUser(ctx context.Context, userID uuid.UUID, params UpdateWaitlistUserParams) (WaitlistUser, error) {
	var user WaitlistUser
	err := s.db.GetContext(ctx, &user, sqlUpdateWaitlistUser,
		userID,
		params.FirstName,
		params.LastName,
		params.Status,
		params.Position,
		params.Points,
		params.Metadata)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WaitlistUser{}, ErrNotFound
		}
		return WaitlistUser{}, fmt.Errorf("failed to update waitlist user: %w", err)
	}
	return user, nil
}

const sqlVerifyWaitlistUserEmail = `
UPDATE waitlist_users
SET email_verified = TRUE,
    verified_at = CURRENT_TIMESTAMP,
    verification_token = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// VerifyWaitlistUserEmail marks a user's email as verified
func (s *Store) VerifyWaitlistUserEmail(ctx context.Context, userID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, sqlVerifyWaitlistUserEmail, userID)
	if err != nil {
		return fmt.Errorf("failed to verify waitlist user email: %w", err)
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

const sqlIncrementReferralCount = `
UPDATE waitlist_users
SET referral_count = referral_count + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// IncrementReferralCount increments the referral count for a user
func (s *Store) IncrementReferralCount(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlIncrementReferralCount, userID)
	if err != nil {
		return fmt.Errorf("failed to increment referral count: %w", err)
	}
	return nil
}

const sqlIncrementVerifiedReferralCount = `
UPDATE waitlist_users
SET verified_referral_count = verified_referral_count + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// IncrementVerifiedReferralCount increments the verified referral count for a user
func (s *Store) IncrementVerifiedReferralCount(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlIncrementVerifiedReferralCount, userID)
	if err != nil {
		return fmt.Errorf("failed to increment verified referral count: %w", err)
	}
	return nil
}

const sqlUpdateWaitlistUserPosition = `
UPDATE waitlist_users
SET position = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// UpdateWaitlistUserPosition updates a user's position in the waitlist
func (s *Store) UpdateWaitlistUserPosition(ctx context.Context, userID uuid.UUID, position int) error {
	_, err := s.db.ExecContext(ctx, sqlUpdateWaitlistUserPosition, userID, position)
	if err != nil {
		return fmt.Errorf("failed to update waitlist user position: %w", err)
	}
	return nil
}

const sqlUpdateLastActivity = `
UPDATE waitlist_users
SET last_activity_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// UpdateLastActivity updates a user's last activity timestamp
func (s *Store) UpdateLastActivity(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlUpdateLastActivity, userID)
	if err != nil {
		return fmt.Errorf("failed to update last activity: %w", err)
	}
	return nil
}

const sqlDeleteWaitlistUser = `
UPDATE waitlist_users
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// DeleteWaitlistUser soft deletes a waitlist user
func (s *Store) DeleteWaitlistUser(ctx context.Context, userID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, sqlDeleteWaitlistUser, userID)
	if err != nil {
		return fmt.Errorf("failed to delete waitlist user: %w", err)
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

const sqlUpdateVerificationToken = `
UPDATE waitlist_users
SET verification_token = $2,
    verification_sent_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// UpdateVerificationToken updates the verification token and sent timestamp
func (s *Store) UpdateVerificationToken(ctx context.Context, userID uuid.UUID, token string) error {
	res, err := s.db.ExecContext(ctx, sqlUpdateVerificationToken, userID, token)
	if err != nil {
		return fmt.Errorf("failed to update verification token: %w", err)
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

const sqlGetWaitlistUserByVerificationToken = `
SELECT ` + waitlistUserColumns + `
FROM waitlist_users
WHERE verification_token = $1 AND deleted_at IS NULL
`

// GetWaitlistUserByVerificationToken retrieves a user by verification token
func (s *Store) GetWaitlistUserByVerificationToken(ctx context.Context, token string) (WaitlistUser, error) {
	var user WaitlistUser
	err := s.db.GetContext(ctx, &user, sqlGetWaitlistUserByVerificationToken, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WaitlistUser{}, ErrNotFound
		}
		return WaitlistUser{}, fmt.Errorf("failed to get waitlist user by verification token: %w", err)
	}
	return user, nil
}

// SearchWaitlistUsersParams represents parameters for searching waitlist users
type SearchWaitlistUsersParams struct {
	CampaignID    uuid.UUID
	Query         *string
	Statuses      []string
	Verified      *bool
	MinReferrals  *int
	DateFrom      *string
	DateTo        *string
	SortBy        string
	SortOrder     string
	Limit         int
	Offset        int
}

// SearchWaitlistUsers performs advanced search with filters
func (s *Store) SearchWaitlistUsers(ctx context.Context, params SearchWaitlistUsersParams) ([]WaitlistUser, error) {
	query := `SELECT ` + waitlistUserColumns + `
	FROM waitlist_users
	WHERE campaign_id = $1 AND deleted_at IS NULL`

	args := []interface{}{params.CampaignID}
	argCount := 1

	// Add search query filter
	if params.Query != nil && *params.Query != "" {
		argCount++
		query += fmt.Sprintf(" AND (email ILIKE $%d OR first_name ILIKE $%d OR last_name ILIKE $%d)", argCount, argCount, argCount)
		searchPattern := "%" + *params.Query + "%"
		args = append(args, searchPattern)
	}

	// Add status filter
	if len(params.Statuses) > 0 {
		argCount++
		query += fmt.Sprintf(" AND status = ANY($%d)", argCount)
		args = append(args, params.Statuses)
	}

	// Add verified filter
	if params.Verified != nil {
		argCount++
		query += fmt.Sprintf(" AND email_verified = $%d", argCount)
		args = append(args, *params.Verified)
	}

	// Add min referrals filter
	if params.MinReferrals != nil {
		argCount++
		query += fmt.Sprintf(" AND verified_referral_count >= $%d", argCount)
		args = append(args, *params.MinReferrals)
	}

	// Add date range filters
	if params.DateFrom != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args = append(args, *params.DateFrom)
	}

	if params.DateTo != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args = append(args, *params.DateTo)
	}

	// Add sorting
	sortBy := "position"
	if params.SortBy != "" {
		sortBy = params.SortBy
	}

	sortOrder := "ASC"
	if params.SortOrder == "desc" {
		sortOrder = "DESC"
	}

	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	// Add pagination
	argCount++
	query += fmt.Sprintf(" LIMIT $%d", argCount)
	args = append(args, params.Limit)

	argCount++
	query += fmt.Sprintf(" OFFSET $%d", argCount)
	args = append(args, params.Offset)

	var users []WaitlistUser
	err := s.db.SelectContext(ctx, &users, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search waitlist users: %w", err)
	}

	return users, nil
}

// CountWaitlistUsersByStatus counts users by status for a campaign
func (s *Store) CountWaitlistUsersByStatus(ctx context.Context, campaignID uuid.UUID, status string) (int, error) {
	query := `SELECT COUNT(*) FROM waitlist_users WHERE campaign_id = $1 AND status = $2 AND deleted_at IS NULL`
	var count int
	err := s.db.GetContext(ctx, &count, query, campaignID, status)
	if err != nil {
		return 0, fmt.Errorf("failed to count waitlist users by status: %w", err)
	}
	return count, nil
}

// GetWaitlistUsersByCampaignWithFilters retrieves users with advanced filtering and sorting
type ListWaitlistUsersWithFiltersParams struct {
	CampaignID uuid.UUID
	Status     *string
	Verified   *bool
	SortBy     string
	SortOrder  string
	Limit      int
	Offset     int
}

// GetWaitlistUsersByCampaignWithFilters retrieves waitlist users with filters and sorting
func (s *Store) GetWaitlistUsersByCampaignWithFilters(ctx context.Context, params ListWaitlistUsersWithFiltersParams) ([]WaitlistUser, error) {
	query := `SELECT ` + waitlistUserColumns + `
	FROM waitlist_users
	WHERE campaign_id = $1 AND deleted_at IS NULL`

	args := []interface{}{params.CampaignID}
	argCount := 1

	// Add status filter
	if params.Status != nil {
		argCount++
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, *params.Status)
	}

	// Add verified filter
	if params.Verified != nil {
		argCount++
		query += fmt.Sprintf(" AND email_verified = $%d", argCount)
		args = append(args, *params.Verified)
	}

	// Add sorting
	sortBy := "position"
	validSortFields := map[string]bool{
		"position":       true,
		"created_at":     true,
		"referral_count": true,
	}
	if params.SortBy != "" && validSortFields[params.SortBy] {
		sortBy = params.SortBy
	}

	sortOrder := "ASC"
	if params.SortOrder == "desc" {
		sortOrder = "DESC"
	}

	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	// Add pagination
	argCount++
	query += fmt.Sprintf(" LIMIT $%d", argCount)
	args = append(args, params.Limit)

	argCount++
	query += fmt.Sprintf(" OFFSET $%d", argCount)
	args = append(args, params.Offset)

	var users []WaitlistUser
	err := s.db.SelectContext(ctx, &users, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get waitlist users with filters: %w", err)
	}

	return users, nil
}

// CountWaitlistUsersWithFilters counts users matching the filter criteria
func (s *Store) CountWaitlistUsersWithFilters(ctx context.Context, campaignID uuid.UUID, status *string, verified *bool) (int, error) {
	query := `SELECT COUNT(*) FROM waitlist_users WHERE campaign_id = $1 AND deleted_at IS NULL`
	args := []interface{}{campaignID}
	argCount := 1

	if status != nil {
		argCount++
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, *status)
	}

	if verified != nil {
		argCount++
		query += fmt.Sprintf(" AND email_verified = $%d", argCount)
		args = append(args, *verified)
	}

	var count int
	err := s.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to count waitlist users with filters: %w", err)
	}
	return count, nil
}

const sqlGetAllWaitlistUsersForPositionCalculation = `
SELECT ` + waitlistUserColumns + `
FROM waitlist_users
WHERE campaign_id = $1 AND deleted_at IS NULL
ORDER BY created_at ASC, id ASC
`

// GetAllWaitlistUsersForPositionCalculation retrieves all waitlist users for a campaign for position calculation
// Returns users ordered by created_at ASC, id ASC (no pagination)
func (s *Store) GetAllWaitlistUsersForPositionCalculation(ctx context.Context, campaignID uuid.UUID) ([]WaitlistUser, error) {
	var users []WaitlistUser
	err := s.db.SelectContext(ctx, &users, sqlGetAllWaitlistUsersForPositionCalculation, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to get all waitlist users for position calculation: %w", err)
	}
	return users, nil
}

const sqlBulkUpdateWaitlistUserPositions = `
UPDATE waitlist_users
SET position = data.new_position,
    updated_at = CURRENT_TIMESTAMP
FROM (SELECT unnest($1::uuid[]) AS user_id, unnest($2::int[]) AS new_position) AS data
WHERE waitlist_users.id = data.user_id
`

// BulkUpdateWaitlistUserPositions updates positions for multiple users in a single query
func (s *Store) BulkUpdateWaitlistUserPositions(ctx context.Context, userIDs []uuid.UUID, positions []int) error {
	if len(userIDs) != len(positions) {
		return fmt.Errorf("userIDs and positions must have same length")
	}
	if len(userIDs) == 0 {
		return nil
	}

	// Convert UUIDs to strings for the array
	userIDStrings := make([]string, len(userIDs))
	for i, id := range userIDs {
		userIDStrings[i] = id.String()
	}

	_, err := s.db.ExecContext(ctx, sqlBulkUpdateWaitlistUserPositions, userIDStrings, positions)
	if err != nil {
		return fmt.Errorf("failed to bulk update waitlist user positions: %w", err)
	}
	return nil
}

const sqlBlockWaitlistUser = `
UPDATE waitlist_users
SET status = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// BlockWaitlistUser updates a user's status to blocked
func (s *Store) BlockWaitlistUser(ctx context.Context, userID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, sqlBlockWaitlistUser, userID, WaitlistUserStatusBlocked)
	if err != nil {
		return fmt.Errorf("failed to block waitlist user: %w", err)
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

const sqlCountRecentSignupsByIP = `
SELECT COUNT(*)
FROM waitlist_users
WHERE campaign_id = $1 AND ip_address = $2 AND created_at > $3 AND deleted_at IS NULL
`

// CountRecentSignupsByIP counts signups from an IP address within a time window
func (s *Store) CountRecentSignupsByIP(ctx context.Context, campaignID uuid.UUID, ip string, since time.Time) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count, sqlCountRecentSignupsByIP, campaignID, ip, since)
	if err != nil {
		return 0, fmt.Errorf("failed to count recent signups by IP: %w", err)
	}
	return count, nil
}

// ExtendedListWaitlistUsersParams represents comprehensive filtering parameters
type ExtendedListWaitlistUsersParams struct {
	CampaignID   uuid.UUID
	Statuses     []string          // Multiple status values (OR condition)
	Sources      []string          // Multiple source values (OR condition)
	HasReferrals *bool             // Filter for users with referrals > 0
	MinPosition  *int              // Position range
	MaxPosition  *int
	DateFrom     *time.Time        // Date range
	DateTo       *time.Time
	CustomFields map[string]string // JSONB metadata filters (AND condition)
	SortBy       string
	SortOrder    string
	Limit        int
	Offset       int
}

// ListWaitlistUsersWithExtendedFilters retrieves users with comprehensive filtering
func (s *Store) ListWaitlistUsersWithExtendedFilters(ctx context.Context, params ExtendedListWaitlistUsersParams) ([]WaitlistUser, error) {
	query := `SELECT ` + waitlistUserColumns + `
	FROM waitlist_users
	WHERE campaign_id = $1 AND deleted_at IS NULL`

	args := []interface{}{params.CampaignID}
	argCount := 1

	// Add status filter (multiple values with OR)
	if len(params.Statuses) > 0 {
		argCount++
		query += fmt.Sprintf(" AND status = ANY($%d)", argCount)
		args = append(args, params.Statuses)
	}

	// Add source filter (multiple values with OR)
	if len(params.Sources) > 0 {
		argCount++
		query += fmt.Sprintf(" AND source = ANY($%d)", argCount)
		args = append(args, params.Sources)
	}

	// Add has referrals filter
	if params.HasReferrals != nil && *params.HasReferrals {
		query += " AND referral_count > 0"
	}

	// Add position range filters
	if params.MinPosition != nil {
		argCount++
		query += fmt.Sprintf(" AND position >= $%d", argCount)
		args = append(args, *params.MinPosition)
	}

	if params.MaxPosition != nil {
		argCount++
		query += fmt.Sprintf(" AND position <= $%d", argCount)
		args = append(args, *params.MaxPosition)
	}

	// Add date range filters
	if params.DateFrom != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args = append(args, *params.DateFrom)
	}

	if params.DateTo != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args = append(args, *params.DateTo)
	}

	// Add custom field filters (case-insensitive text match)
	for key, value := range params.CustomFields {
		argCount++
		// Use ILIKE for case-insensitive partial match on JSONB text value
		query += fmt.Sprintf(" AND metadata ->> $%d ILIKE $%d", argCount, argCount+1)
		args = append(args, key, "%"+value+"%")
		argCount++
	}

	// Add sorting
	sortBy := "position"
	validSortFields := map[string]bool{
		"position":       true,
		"created_at":     true,
		"referral_count": true,
		"email":          true,
		"status":         true,
		"source":         true,
	}
	if params.SortBy != "" && validSortFields[params.SortBy] {
		sortBy = params.SortBy
	}

	sortOrder := "ASC"
	if params.SortOrder == "desc" {
		sortOrder = "DESC"
	}

	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	// Add pagination
	argCount++
	query += fmt.Sprintf(" LIMIT $%d", argCount)
	args = append(args, params.Limit)

	argCount++
	query += fmt.Sprintf(" OFFSET $%d", argCount)
	args = append(args, params.Offset)

	var users []WaitlistUser
	err := s.db.SelectContext(ctx, &users, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list waitlist users with extended filters: %w", err)
	}

	return users, nil
}

// CountWaitlistUsersWithExtendedFilters counts users matching extended filter criteria
func (s *Store) CountWaitlistUsersWithExtendedFilters(ctx context.Context, params ExtendedListWaitlistUsersParams) (int, error) {
	query := `SELECT COUNT(*) FROM waitlist_users WHERE campaign_id = $1 AND deleted_at IS NULL`
	args := []interface{}{params.CampaignID}
	argCount := 1

	// Add status filter
	if len(params.Statuses) > 0 {
		argCount++
		query += fmt.Sprintf(" AND status = ANY($%d)", argCount)
		args = append(args, params.Statuses)
	}

	// Add source filter
	if len(params.Sources) > 0 {
		argCount++
		query += fmt.Sprintf(" AND source = ANY($%d)", argCount)
		args = append(args, params.Sources)
	}

	// Add has referrals filter
	if params.HasReferrals != nil && *params.HasReferrals {
		query += " AND referral_count > 0"
	}

	// Add position range filters
	if params.MinPosition != nil {
		argCount++
		query += fmt.Sprintf(" AND position >= $%d", argCount)
		args = append(args, *params.MinPosition)
	}

	if params.MaxPosition != nil {
		argCount++
		query += fmt.Sprintf(" AND position <= $%d", argCount)
		args = append(args, *params.MaxPosition)
	}

	// Add date range filters
	if params.DateFrom != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args = append(args, *params.DateFrom)
	}

	if params.DateTo != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args = append(args, *params.DateTo)
	}

	// Add custom field filters (case-insensitive text match)
	for key, value := range params.CustomFields {
		argCount++
		// Use ILIKE for case-insensitive partial match on JSONB text value
		query += fmt.Sprintf(" AND metadata ->> $%d ILIKE $%d", argCount, argCount+1)
		args = append(args, key, "%"+value+"%")
		argCount++
	}

	var count int
	err := s.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to count waitlist users with extended filters: %w", err)
	}
	return count, nil
}
