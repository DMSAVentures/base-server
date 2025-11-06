package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateReferralParams represents parameters for creating a referral
type CreateReferralParams struct {
	CampaignID uuid.UUID
	ReferrerID uuid.UUID
	ReferredID uuid.UUID
	Source     *string
	IPAddress  *string
}

const sqlCreateReferral = `
INSERT INTO referrals (campaign_id, referrer_id, referred_id, source, ip_address)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, campaign_id, referrer_id, referred_id, status, source, ip_address, verified_at, converted_at, created_at, updated_at
`

// CreateReferral creates a new referral
func (s *Store) CreateReferral(ctx context.Context, params CreateReferralParams) (Referral, error) {
	var referral Referral
	err := s.db.GetContext(ctx, &referral, sqlCreateReferral,
		params.CampaignID,
		params.ReferrerID,
		params.ReferredID,
		params.Source,
		params.IPAddress)
	if err != nil {
		s.logger.Error(ctx, "failed to create referral", err)
		return Referral{}, fmt.Errorf("failed to create referral: %w", err)
	}
	return referral, nil
}

const sqlGetReferralByID = `
SELECT id, campaign_id, referrer_id, referred_id, status, source, ip_address, verified_at, converted_at, created_at, updated_at
FROM referrals
WHERE id = $1
`

// GetReferralByID retrieves a referral by ID
func (s *Store) GetReferralByID(ctx context.Context, referralID uuid.UUID) (Referral, error) {
	var referral Referral
	err := s.db.GetContext(ctx, &referral, sqlGetReferralByID, referralID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Referral{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get referral by id", err)
		return Referral{}, fmt.Errorf("failed to get referral by id: %w", err)
	}
	return referral, nil
}

const sqlGetReferralsByReferrer = `
SELECT id, campaign_id, referrer_id, referred_id, status, source, ip_address, verified_at, converted_at, created_at, updated_at
FROM referrals
WHERE referrer_id = $1
ORDER BY created_at DESC
`

// GetReferralsByReferrer retrieves all referrals made by a specific user
func (s *Store) GetReferralsByReferrer(ctx context.Context, referrerID uuid.UUID) ([]Referral, error) {
	var referrals []Referral
	err := s.db.SelectContext(ctx, &referrals, sqlGetReferralsByReferrer, referrerID)
	if err != nil {
		s.logger.Error(ctx, "failed to get referrals by referrer", err)
		return nil, fmt.Errorf("failed to get referrals by referrer: %w", err)
	}
	return referrals, nil
}

const sqlGetReferralsByCampaign = `
SELECT id, campaign_id, referrer_id, referred_id, status, source, ip_address, verified_at, converted_at, created_at, updated_at
FROM referrals
WHERE campaign_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
`

// GetReferralsByCampaign retrieves referrals for a campaign with pagination
func (s *Store) GetReferralsByCampaign(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]Referral, error) {
	var referrals []Referral
	err := s.db.SelectContext(ctx, &referrals, sqlGetReferralsByCampaign, campaignID, limit, offset)
	if err != nil {
		s.logger.Error(ctx, "failed to get referrals by campaign", err)
		return nil, fmt.Errorf("failed to get referrals by campaign: %w", err)
	}
	return referrals, nil
}

const sqlCountReferralsByCampaign = `
SELECT COUNT(*)
FROM referrals
WHERE campaign_id = $1
`

// CountReferralsByCampaign counts total referrals for a campaign
func (s *Store) CountReferralsByCampaign(ctx context.Context, campaignID uuid.UUID) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count, sqlCountReferralsByCampaign, campaignID)
	if err != nil {
		s.logger.Error(ctx, "failed to count referrals", err)
		return 0, fmt.Errorf("failed to count referrals: %w", err)
	}
	return count, nil
}

const sqlUpdateReferralStatus = `
UPDATE referrals
SET status = $2,
    verified_at = CASE WHEN $2 = 'verified' THEN CURRENT_TIMESTAMP ELSE verified_at END,
    converted_at = CASE WHEN $2 = 'converted' THEN CURRENT_TIMESTAMP ELSE converted_at END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// UpdateReferralStatus updates a referral's status
func (s *Store) UpdateReferralStatus(ctx context.Context, referralID uuid.UUID, status string) error {
	res, err := s.db.ExecContext(ctx, sqlUpdateReferralStatus, referralID, status)
	if err != nil {
		s.logger.Error(ctx, "failed to update referral status", err)
		return fmt.Errorf("failed to update referral status: %w", err)
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

const sqlGetReferralByReferrerAndReferred = `
SELECT id, campaign_id, referrer_id, referred_id, status, source, ip_address, verified_at, converted_at, created_at, updated_at
FROM referrals
WHERE referrer_id = $1 AND referred_id = $2
`

// GetReferralByReferrerAndReferred retrieves a referral by referrer and referred user IDs
func (s *Store) GetReferralByReferrerAndReferred(ctx context.Context, referrerID, referredID uuid.UUID) (Referral, error) {
	var referral Referral
	err := s.db.GetContext(ctx, &referral, sqlGetReferralByReferrerAndReferred, referrerID, referredID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Referral{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get referral", err)
		return Referral{}, fmt.Errorf("failed to get referral: %w", err)
	}
	return referral, nil
}

const sqlGetVerifiedReferralCountByReferrer = `
SELECT COUNT(*)
FROM referrals
WHERE referrer_id = $1 AND status = 'verified'
`

// GetVerifiedReferralCountByReferrer counts verified referrals for a user
func (s *Store) GetVerifiedReferralCountByReferrer(ctx context.Context, referrerID uuid.UUID) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count, sqlGetVerifiedReferralCountByReferrer, referrerID)
	if err != nil {
		s.logger.Error(ctx, "failed to count verified referrals", err)
		return 0, fmt.Errorf("failed to count verified referrals: %w", err)
	}
	return count, nil
}

const sqlGetReferralsByStatus = `
SELECT id, campaign_id, referrer_id, referred_id, status, source, ip_address, verified_at, converted_at, created_at, updated_at
FROM referrals
WHERE campaign_id = $1 AND status = $2
ORDER BY created_at DESC
`

// GetReferralsByStatus retrieves referrals by campaign and status
func (s *Store) GetReferralsByStatus(ctx context.Context, campaignID uuid.UUID, status string) ([]Referral, error) {
	var referrals []Referral
	err := s.db.SelectContext(ctx, &referrals, sqlGetReferralsByStatus, campaignID, status)
	if err != nil {
		s.logger.Error(ctx, "failed to get referrals by status", err)
		return nil, fmt.Errorf("failed to get referrals by status: %w", err)
	}
	return referrals, nil
}
