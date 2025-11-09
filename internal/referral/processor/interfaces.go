package processor

import (
	"base-server/internal/store"
	"context"

	"github.com/google/uuid"
)

// ReferralStore defines the database operations required by ReferralProcessor
type ReferralStore interface {
	GetReferralsByCampaignWithStatusFilter(ctx context.Context, campaignID uuid.UUID, status *string, limit, offset int) ([]store.Referral, error)
	CountReferralsByCampaignWithStatusFilter(ctx context.Context, campaignID uuid.UUID, status *string) (int, error)
	GetWaitlistUserByReferralCode(ctx context.Context, referralCode string) (store.WaitlistUser, error)
	GetWaitlistUserByID(ctx context.Context, userID uuid.UUID) (store.WaitlistUser, error)
	GetReferralsByReferrerWithPagination(ctx context.Context, referrerID uuid.UUID, limit, offset int) ([]store.Referral, error)
	CountReferralsByReferrer(ctx context.Context, referrerID uuid.UUID) (int, error)
	GetVerifiedReferralCountByReferrer(ctx context.Context, referrerID uuid.UUID) (int, error)
	GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (store.Campaign, error)
}
