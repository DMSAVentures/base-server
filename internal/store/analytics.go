package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateCampaignAnalyticsParams represents parameters for creating campaign analytics
type CreateCampaignAnalyticsParams struct {
	Time       time.Time
	CampaignID uuid.UUID

	NewSignups     int
	NewVerified    int
	NewReferrals   int
	NewConversions int

	EmailsSent    int
	EmailsOpened  int
	EmailsClicked int

	RewardsEarned    int
	RewardsDelivered int

	TotalSignups   int
	TotalVerified  int
	TotalReferrals int
}

const sqlCreateCampaignAnalytics = `
INSERT INTO campaign_analytics (
	time, campaign_id,
	new_signups, new_verified, new_referrals, new_conversions,
	emails_sent, emails_opened, emails_clicked,
	rewards_earned, rewards_delivered,
	total_signups, total_verified, total_referrals
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (time, campaign_id) DO UPDATE SET
	new_signups = EXCLUDED.new_signups,
	new_verified = EXCLUDED.new_verified,
	new_referrals = EXCLUDED.new_referrals,
	new_conversions = EXCLUDED.new_conversions,
	emails_sent = EXCLUDED.emails_sent,
	emails_opened = EXCLUDED.emails_opened,
	emails_clicked = EXCLUDED.emails_clicked,
	rewards_earned = EXCLUDED.rewards_earned,
	rewards_delivered = EXCLUDED.rewards_delivered,
	total_signups = EXCLUDED.total_signups,
	total_verified = EXCLUDED.total_verified,
	total_referrals = EXCLUDED.total_referrals
`

// CreateCampaignAnalytics creates or updates campaign analytics for a time bucket
func (s *Store) CreateCampaignAnalytics(ctx context.Context, params CreateCampaignAnalyticsParams) error {
	_, err := s.db.ExecContext(ctx, sqlCreateCampaignAnalytics,
		params.Time,
		params.CampaignID,
		params.NewSignups,
		params.NewVerified,
		params.NewReferrals,
		params.NewConversions,
		params.EmailsSent,
		params.EmailsOpened,
		params.EmailsClicked,
		params.RewardsEarned,
		params.RewardsDelivered,
		params.TotalSignups,
		params.TotalVerified,
		params.TotalReferrals)
	if err != nil {
		s.logger.Error(ctx, "failed to create campaign analytics", err)
		return fmt.Errorf("failed to create campaign analytics: %w", err)
	}
	return nil
}

const sqlGetCampaignAnalyticsByRange = `
SELECT time, campaign_id, new_signups, new_verified, new_referrals, new_conversions,
       emails_sent, emails_opened, emails_clicked,
       rewards_earned, rewards_delivered,
       total_signups, total_verified, total_referrals
FROM campaign_analytics
WHERE campaign_id = $1 AND time >= $2 AND time < $3
ORDER BY time ASC
`

// GetCampaignAnalyticsByRange retrieves campaign analytics for a time range
func (s *Store) GetCampaignAnalyticsByRange(ctx context.Context, campaignID uuid.UUID, startTime, endTime time.Time) ([]CampaignAnalytics, error) {
	var analytics []CampaignAnalytics
	err := s.db.SelectContext(ctx, &analytics, sqlGetCampaignAnalyticsByRange, campaignID, startTime, endTime)
	if err != nil {
		s.logger.Error(ctx, "failed to get campaign analytics by range", err)
		return nil, fmt.Errorf("failed to get campaign analytics by range: %w", err)
	}
	return analytics, nil
}

const sqlCountNewSignupsSince = `
SELECT COUNT(*)
FROM waitlist_users
WHERE campaign_id = $1 AND created_at >= $2 AND created_at < $3 AND deleted_at IS NULL
`

// CountNewSignupsSince counts new signups in a time range
func (s *Store) CountNewSignupsSince(ctx context.Context, campaignID uuid.UUID, startTime, endTime time.Time) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count, sqlCountNewSignupsSince, campaignID, startTime, endTime)
	if err != nil {
		s.logger.Error(ctx, "failed to count new signups", err)
		return 0, fmt.Errorf("failed to count new signups: %w", err)
	}
	return count, nil
}

const sqlCountNewVerifiedSince = `
SELECT COUNT(*)
FROM waitlist_users
WHERE campaign_id = $1 AND verified_at >= $2 AND verified_at < $3 AND deleted_at IS NULL
`

// CountNewVerifiedSince counts new verified users in a time range
func (s *Store) CountNewVerifiedSince(ctx context.Context, campaignID uuid.UUID, startTime, endTime time.Time) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count, sqlCountNewVerifiedSince, campaignID, startTime, endTime)
	if err != nil {
		s.logger.Error(ctx, "failed to count new verified users", err)
		return 0, fmt.Errorf("failed to count new verified users: %w", err)
	}
	return count, nil
}

const sqlCountNewReferralsSince = `
SELECT COUNT(*)
FROM referrals
WHERE campaign_id = $1 AND created_at >= $2 AND created_at < $3
`

// CountNewReferralsSince counts new referrals in a time range
func (s *Store) CountNewReferralsSince(ctx context.Context, campaignID uuid.UUID, startTime, endTime time.Time) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count, sqlCountNewReferralsSince, campaignID, startTime, endTime)
	if err != nil {
		s.logger.Error(ctx, "failed to count new referrals", err)
		return 0, fmt.Errorf("failed to count new referrals: %w", err)
	}
	return count, nil
}

// CreateUserActivityLogParams represents parameters for creating a user activity log
type CreateUserActivityLogParams struct {
	CampaignID uuid.UUID
	UserID     *uuid.UUID
	EventType  string
	EventData  JSONB
	IPAddress  *string
	UserAgent  *string
}

const sqlCreateUserActivityLog = `
INSERT INTO user_activity_logs (campaign_id, user_id, event_type, event_data, ip_address, user_agent)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, campaign_id, user_id, event_type, event_data, ip_address, user_agent, created_at
`

// CreateUserActivityLog creates a user activity log entry
func (s *Store) CreateUserActivityLog(ctx context.Context, params CreateUserActivityLogParams) (UserActivityLog, error) {
	var log UserActivityLog
	err := s.db.GetContext(ctx, &log, sqlCreateUserActivityLog,
		params.CampaignID,
		params.UserID,
		params.EventType,
		params.EventData,
		params.IPAddress,
		params.UserAgent)
	if err != nil {
		s.logger.Error(ctx, "failed to create user activity log", err)
		return UserActivityLog{}, fmt.Errorf("failed to create user activity log: %w", err)
	}
	return log, nil
}
