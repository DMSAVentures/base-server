package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CreateSegmentParams represents parameters for creating a segment
type CreateSegmentParams struct {
	CampaignID     uuid.UUID
	Name           string
	Description    *string
	FilterCriteria JSONB
}

const sqlCreateSegment = `
INSERT INTO segments (campaign_id, name, description, filter_criteria)
VALUES ($1, $2, $3, $4)
RETURNING id, campaign_id, name, description, filter_criteria, cached_user_count, cached_at, status, created_at, updated_at, deleted_at
`

// CreateSegment creates a new segment
func (s *Store) CreateSegment(ctx context.Context, params CreateSegmentParams) (Segment, error) {
	var segment Segment
	err := s.db.GetContext(ctx, &segment, sqlCreateSegment,
		params.CampaignID,
		params.Name,
		params.Description,
		params.FilterCriteria)
	if err != nil {
		return Segment{}, fmt.Errorf("failed to create segment: %w", err)
	}
	return segment, nil
}

const sqlGetSegmentByID = `
SELECT id, campaign_id, name, description, filter_criteria, cached_user_count, cached_at, status, created_at, updated_at, deleted_at
FROM segments
WHERE id = $1 AND deleted_at IS NULL
`

// GetSegmentByID retrieves a segment by ID
func (s *Store) GetSegmentByID(ctx context.Context, segmentID uuid.UUID) (Segment, error) {
	var segment Segment
	err := s.db.GetContext(ctx, &segment, sqlGetSegmentByID, segmentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Segment{}, ErrNotFound
		}
		return Segment{}, fmt.Errorf("failed to get segment: %w", err)
	}
	return segment, nil
}

const sqlGetSegmentsByCampaign = `
SELECT id, campaign_id, name, description, filter_criteria, cached_user_count, cached_at, status, created_at, updated_at, deleted_at
FROM segments
WHERE campaign_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
`

// GetSegmentsByCampaign retrieves all segments for a campaign
func (s *Store) GetSegmentsByCampaign(ctx context.Context, campaignID uuid.UUID) ([]Segment, error) {
	var segments []Segment
	err := s.db.SelectContext(ctx, &segments, sqlGetSegmentsByCampaign, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to get segments: %w", err)
	}
	return segments, nil
}

const sqlGetActiveSegmentsByCampaign = `
SELECT id, campaign_id, name, description, filter_criteria, cached_user_count, cached_at, status, created_at, updated_at, deleted_at
FROM segments
WHERE campaign_id = $1 AND status = 'active' AND deleted_at IS NULL
ORDER BY created_at DESC
`

// GetActiveSegmentsByCampaign retrieves all active segments for a campaign
func (s *Store) GetActiveSegmentsByCampaign(ctx context.Context, campaignID uuid.UUID) ([]Segment, error) {
	var segments []Segment
	err := s.db.SelectContext(ctx, &segments, sqlGetActiveSegmentsByCampaign, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active segments: %w", err)
	}
	return segments, nil
}

// UpdateSegmentParams represents parameters for updating a segment
type UpdateSegmentParams struct {
	Name           *string
	Description    *string
	FilterCriteria *JSONB
	Status         *string
}

const sqlUpdateSegment = `
UPDATE segments
SET name = COALESCE($2, name),
    description = COALESCE($3, description),
    filter_criteria = COALESCE($4, filter_criteria),
    status = COALESCE($5, status),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, campaign_id, name, description, filter_criteria, cached_user_count, cached_at, status, created_at, updated_at, deleted_at
`

// UpdateSegment updates a segment
func (s *Store) UpdateSegment(ctx context.Context, segmentID uuid.UUID, params UpdateSegmentParams) (Segment, error) {
	var segment Segment
	err := s.db.GetContext(ctx, &segment, sqlUpdateSegment,
		segmentID,
		params.Name,
		params.Description,
		params.FilterCriteria,
		params.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Segment{}, ErrNotFound
		}
		return Segment{}, fmt.Errorf("failed to update segment: %w", err)
	}
	return segment, nil
}

const sqlDeleteSegment = `
UPDATE segments
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// DeleteSegment soft deletes a segment
func (s *Store) DeleteSegment(ctx context.Context, segmentID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, sqlDeleteSegment, segmentID)
	if err != nil {
		return fmt.Errorf("failed to delete segment: %w", err)
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

const sqlUpdateSegmentCachedCount = `
UPDATE segments
SET cached_user_count = $2,
    cached_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// UpdateSegmentCachedCount updates the cached user count for a segment
func (s *Store) UpdateSegmentCachedCount(ctx context.Context, segmentID uuid.UUID, count int) error {
	res, err := s.db.ExecContext(ctx, sqlUpdateSegmentCachedCount, segmentID, count)
	if err != nil {
		return fmt.Errorf("failed to update segment cached count: %w", err)
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

// CountUsersMatchingCriteria counts users matching the given filter criteria
func (s *Store) CountUsersMatchingCriteria(ctx context.Context, campaignID uuid.UUID, criteria SegmentFilterCriteria) (int, error) {
	query, args := buildSegmentFilterQuery(campaignID, criteria, true, 0, 0)

	var count int
	err := s.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to count users matching criteria: %w", err)
	}

	return count, nil
}

// GetUsersMatchingCriteria retrieves users matching the given filter criteria
func (s *Store) GetUsersMatchingCriteria(ctx context.Context, campaignID uuid.UUID, criteria SegmentFilterCriteria, limit, offset int) ([]WaitlistUser, error) {
	query, args := buildSegmentFilterQuery(campaignID, criteria, false, limit, offset)

	var users []WaitlistUser
	err := s.db.SelectContext(ctx, &users, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get users matching criteria: %w", err)
	}

	return users, nil
}

// buildSegmentFilterQuery builds the SQL query for filtering users based on segment criteria
func buildSegmentFilterQuery(campaignID uuid.UUID, criteria SegmentFilterCriteria, countOnly bool, limit, offset int) (string, []interface{}) {
	var query strings.Builder
	args := []interface{}{campaignID}
	argCount := 1

	if countOnly {
		query.WriteString("SELECT COUNT(*) FROM waitlist_users WHERE campaign_id = $1 AND deleted_at IS NULL")
	} else {
		query.WriteString("SELECT id, campaign_id, email, first_name, last_name, status, position, original_position, referral_code, referred_by_id, referral_count, verified_referral_count, points, email_verified, verification_token, verification_sent_at, verified_at, source, utm_source, utm_medium, utm_campaign, utm_term, utm_content, ip_address, user_agent, country_code, city, device_fingerprint, metadata, marketing_consent, marketing_consent_at, terms_accepted, terms_accepted_at, last_activity_at, share_count, created_at, updated_at, deleted_at FROM waitlist_users WHERE campaign_id = $1 AND deleted_at IS NULL")
	}

	// Status filter
	if len(criteria.Statuses) > 0 {
		argCount++
		placeholders := make([]string, len(criteria.Statuses))
		for i := range criteria.Statuses {
			placeholders[i] = fmt.Sprintf("$%d", argCount+i)
		}
		query.WriteString(fmt.Sprintf(" AND status IN (%s)", strings.Join(placeholders, ", ")))
		for _, status := range criteria.Statuses {
			args = append(args, status)
		}
		argCount += len(criteria.Statuses) - 1
	}

	// Source filter
	if len(criteria.Sources) > 0 {
		argCount++
		placeholders := make([]string, len(criteria.Sources))
		for i := range criteria.Sources {
			placeholders[i] = fmt.Sprintf("$%d", argCount+i)
		}
		query.WriteString(fmt.Sprintf(" AND source IN (%s)", strings.Join(placeholders, ", ")))
		for _, source := range criteria.Sources {
			args = append(args, source)
		}
		argCount += len(criteria.Sources) - 1
	}

	// Email verified filter
	if criteria.EmailVerified != nil {
		argCount++
		query.WriteString(fmt.Sprintf(" AND email_verified = $%d", argCount))
		args = append(args, *criteria.EmailVerified)
	}

	// Has referrals filter
	if criteria.HasReferrals != nil {
		if *criteria.HasReferrals {
			query.WriteString(" AND referral_count > 0")
		} else {
			query.WriteString(" AND referral_count = 0")
		}
	}

	// Min referrals filter
	if criteria.MinReferrals != nil {
		argCount++
		query.WriteString(fmt.Sprintf(" AND referral_count >= $%d", argCount))
		args = append(args, *criteria.MinReferrals)
	}

	// Position range filters
	if criteria.MinPosition != nil {
		argCount++
		query.WriteString(fmt.Sprintf(" AND position >= $%d", argCount))
		args = append(args, *criteria.MinPosition)
	}

	if criteria.MaxPosition != nil {
		argCount++
		query.WriteString(fmt.Sprintf(" AND position <= $%d", argCount))
		args = append(args, *criteria.MaxPosition)
	}

	// Date range filters
	if criteria.DateFrom != nil {
		argCount++
		query.WriteString(fmt.Sprintf(" AND created_at >= $%d", argCount))
		args = append(args, *criteria.DateFrom)
	}

	if criteria.DateTo != nil {
		argCount++
		query.WriteString(fmt.Sprintf(" AND created_at <= $%d", argCount))
		args = append(args, *criteria.DateTo)
	}

	// Custom fields filter (JSONB metadata)
	for key, value := range criteria.CustomFields {
		argCount++
		query.WriteString(fmt.Sprintf(" AND metadata ->> $%d ILIKE $%d", argCount, argCount+1))
		args = append(args, key, "%"+value+"%")
		argCount++
	}

	// Add ordering and pagination for non-count queries
	if !countOnly {
		query.WriteString(" ORDER BY position ASC")
		if limit > 0 {
			argCount++
			query.WriteString(fmt.Sprintf(" LIMIT $%d", argCount))
			args = append(args, limit)
		}
		if offset > 0 {
			argCount++
			query.WriteString(fmt.Sprintf(" OFFSET $%d", argCount))
			args = append(args, offset)
		}
	}

	return query.String(), args
}

// GetUsersForBlast retrieves all users matching segment criteria for a blast (no pagination)
func (s *Store) GetUsersForBlast(ctx context.Context, campaignID uuid.UUID, criteria SegmentFilterCriteria) ([]WaitlistUser, error) {
	query, args := buildSegmentFilterQuery(campaignID, criteria, false, 0, 0)

	var users []WaitlistUser
	err := s.db.SelectContext(ctx, &users, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get users for blast: %w", err)
	}

	return users, nil
}

// ParseFilterCriteria parses JSONB filter criteria into SegmentFilterCriteria struct
func ParseFilterCriteria(jsonb JSONB) (SegmentFilterCriteria, error) {
	criteria := SegmentFilterCriteria{}

	if statuses, ok := jsonb["statuses"].([]interface{}); ok {
		for _, s := range statuses {
			if str, ok := s.(string); ok {
				criteria.Statuses = append(criteria.Statuses, str)
			}
		}
	}

	if sources, ok := jsonb["sources"].([]interface{}); ok {
		for _, s := range sources {
			if str, ok := s.(string); ok {
				criteria.Sources = append(criteria.Sources, str)
			}
		}
	}

	if emailVerified, ok := jsonb["email_verified"].(bool); ok {
		criteria.EmailVerified = &emailVerified
	}

	if hasReferrals, ok := jsonb["has_referrals"].(bool); ok {
		criteria.HasReferrals = &hasReferrals
	}

	if minReferrals, ok := jsonb["min_referrals"].(float64); ok {
		val := int(minReferrals)
		criteria.MinReferrals = &val
	}

	if minPosition, ok := jsonb["min_position"].(float64); ok {
		val := int(minPosition)
		criteria.MinPosition = &val
	}

	if maxPosition, ok := jsonb["max_position"].(float64); ok {
		val := int(maxPosition)
		criteria.MaxPosition = &val
	}

	if dateFrom, ok := jsonb["date_from"].(string); ok {
		if t, err := time.Parse(time.RFC3339, dateFrom); err == nil {
			criteria.DateFrom = &t
		}
	}

	if dateTo, ok := jsonb["date_to"].(string); ok {
		if t, err := time.Parse(time.RFC3339, dateTo); err == nil {
			criteria.DateTo = &t
		}
	}

	if customFields, ok := jsonb["custom_fields"].(map[string]interface{}); ok {
		criteria.CustomFields = make(map[string]string)
		for k, v := range customFields {
			if str, ok := v.(string); ok {
				criteria.CustomFields[k] = str
			}
		}
	}

	return criteria, nil
}
