package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// CreateFraudDetectionParams represents parameters for creating a fraud detection record
type CreateFraudDetectionParams struct {
	CampaignID      uuid.UUID
	UserID          *uuid.UUID
	DetectionType   string
	ConfidenceScore float64
	Details         JSONB
}

const sqlCreateFraudDetection = `
INSERT INTO fraud_detections (campaign_id, user_id, detection_type, confidence_score, details)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, campaign_id, user_id, detection_type, confidence_score, details, status, reviewed_by, reviewed_at, review_notes, created_at
`

// CreateFraudDetection creates a new fraud detection record
func (s *Store) CreateFraudDetection(ctx context.Context, params CreateFraudDetectionParams) (FraudDetection, error) {
	var detection FraudDetection
	err := s.db.GetContext(ctx, &detection, sqlCreateFraudDetection,
		params.CampaignID,
		params.UserID,
		params.DetectionType,
		params.ConfidenceScore,
		params.Details)
	if err != nil {
		s.logger.Error(ctx, "failed to create fraud detection", err)
		return FraudDetection{}, fmt.Errorf("failed to create fraud detection: %w", err)
	}
	return detection, nil
}

const sqlGetFraudDetectionsByUser = `
SELECT id, campaign_id, user_id, detection_type, confidence_score, details, status, reviewed_by, reviewed_at, review_notes, created_at
FROM fraud_detections
WHERE user_id = $1
ORDER BY created_at DESC
`

// GetFraudDetectionsByUser retrieves fraud detections for a user
func (s *Store) GetFraudDetectionsByUser(ctx context.Context, userID uuid.UUID) ([]FraudDetection, error) {
	var detections []FraudDetection
	err := s.db.SelectContext(ctx, &detections, sqlGetFraudDetectionsByUser, userID)
	if err != nil {
		s.logger.Error(ctx, "failed to get fraud detections by user", err)
		return nil, fmt.Errorf("failed to get fraud detections by user: %w", err)
	}
	return detections, nil
}

const sqlGetFraudDetectionsByCampaign = `
SELECT id, campaign_id, user_id, detection_type, confidence_score, details, status, reviewed_by, reviewed_at, review_notes, created_at
FROM fraud_detections
WHERE campaign_id = $1 AND status = 'pending'
ORDER BY confidence_score DESC, created_at DESC
LIMIT $2
`

// GetFraudDetectionsByCampaign retrieves pending fraud detections for a campaign
func (s *Store) GetFraudDetectionsByCampaign(ctx context.Context, campaignID uuid.UUID, limit int) ([]FraudDetection, error) {
	var detections []FraudDetection
	err := s.db.SelectContext(ctx, &detections, sqlGetFraudDetectionsByCampaign, campaignID, limit)
	if err != nil {
		s.logger.Error(ctx, "failed to get fraud detections by campaign", err)
		return nil, fmt.Errorf("failed to get fraud detections by campaign: %w", err)
	}
	return detections, nil
}

const sqlGetUsersByIPAddress = `
SELECT id, campaign_id, email, first_name, last_name, status, position, original_position, referral_code, referred_by_id, referral_count, verified_referral_count, points, email_verified, verification_token, verification_sent_at, verified_at, source, utm_source, utm_medium, utm_campaign, utm_term, utm_content, ip_address, user_agent, country_code, city, device_fingerprint, metadata, marketing_consent, marketing_consent_at, terms_accepted, terms_accepted_at, last_activity_at, share_count, created_at, updated_at, deleted_at
FROM waitlist_users
WHERE campaign_id = $1 AND ip_address = $2 AND deleted_at IS NULL
`

// GetUsersByIPAddress retrieves users by IP address (for fraud detection)
func (s *Store) GetUsersByIPAddress(ctx context.Context, campaignID uuid.UUID, ipAddress string) ([]WaitlistUser, error) {
	var users []WaitlistUser
	err := s.db.SelectContext(ctx, &users, sqlGetUsersByIPAddress, campaignID, ipAddress)
	if err != nil {
		s.logger.Error(ctx, "failed to get users by IP address", err)
		return nil, fmt.Errorf("failed to get users by IP address: %w", err)
	}
	return users, nil
}

const sqlGetUsersByDeviceFingerprint = `
SELECT id, campaign_id, email, first_name, last_name, status, position, original_position, referral_code, referred_by_id, referral_count, verified_referral_count, points, email_verified, verification_token, verification_sent_at, verified_at, source, utm_source, utm_medium, utm_campaign, utm_term, utm_content, ip_address, user_agent, country_code, city, device_fingerprint, metadata, marketing_consent, marketing_consent_at, terms_accepted, terms_accepted_at, last_activity_at, share_count, created_at, updated_at, deleted_at
FROM waitlist_users
WHERE campaign_id = $1 AND device_fingerprint = $2 AND deleted_at IS NULL
`

// GetUsersByDeviceFingerprint retrieves users by device fingerprint (for fraud detection)
func (s *Store) GetUsersByDeviceFingerprint(ctx context.Context, campaignID uuid.UUID, fingerprint string) ([]WaitlistUser, error) {
	var users []WaitlistUser
	err := s.db.SelectContext(ctx, &users, sqlGetUsersByDeviceFingerprint, campaignID, fingerprint)
	if err != nil {
		s.logger.Error(ctx, "failed to get users by device fingerprint", err)
		return nil, fmt.Errorf("failed to get users by device fingerprint: %w", err)
	}
	return users, nil
}

const sqlCountRecentReferralsByUser = `
SELECT COUNT(*)
FROM referrals
WHERE referrer_id = $1 AND created_at >= NOW() - INTERVAL '1 hour'
`

// CountRecentReferralsByUser counts referrals made by a user in the last hour (for velocity checks)
func (s *Store) CountRecentReferralsByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count, sqlCountRecentReferralsByUser, userID)
	if err != nil {
		s.logger.Error(ctx, "failed to count recent referrals", err)
		return 0, fmt.Errorf("failed to count recent referrals: %w", err)
	}
	return count, nil
}
