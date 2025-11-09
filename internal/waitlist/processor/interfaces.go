package processor

import (
	"base-server/internal/store"
	"context"

	"github.com/google/uuid"
)

// Store defines the database operations required by WaitlistProcessor
type Store interface {
	GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (store.Campaign, error)
	GetWaitlistUserByEmail(ctx context.Context, campaignID uuid.UUID, email string) (store.WaitlistUser, error)
	GetWaitlistUserByReferralCode(ctx context.Context, referralCode string) (store.WaitlistUser, error)
	CountWaitlistUsersByCampaign(ctx context.Context, campaignID uuid.UUID) (int, error)
	CreateWaitlistUser(ctx context.Context, params store.CreateWaitlistUserParams) (store.WaitlistUser, error)
	IncrementReferralCount(ctx context.Context, userID uuid.UUID) error
	GetWaitlistUsersByCampaignWithFilters(ctx context.Context, params store.ListWaitlistUsersWithFiltersParams) ([]store.WaitlistUser, error)
	CountWaitlistUsersWithFilters(ctx context.Context, campaignID uuid.UUID, status *string, verified *bool) (int, error)
	GetWaitlistUserByID(ctx context.Context, userID uuid.UUID) (store.WaitlistUser, error)
	UpdateWaitlistUser(ctx context.Context, userID uuid.UUID, params store.UpdateWaitlistUserParams) (store.WaitlistUser, error)
	DeleteWaitlistUser(ctx context.Context, userID uuid.UUID) error
	SearchWaitlistUsers(ctx context.Context, params store.SearchWaitlistUsersParams) ([]store.WaitlistUser, error)
	VerifyWaitlistUserEmail(ctx context.Context, userID uuid.UUID) error
	IncrementVerifiedReferralCount(ctx context.Context, userID uuid.UUID) error
	UpdateVerificationToken(ctx context.Context, userID uuid.UUID, token string) error
}

// EventDispatcher defines the event operations required by WaitlistProcessor
type EventDispatcher interface {
	DispatchUserCreated(ctx context.Context, accountID, campaignID uuid.UUID, userData map[string]interface{})
}
