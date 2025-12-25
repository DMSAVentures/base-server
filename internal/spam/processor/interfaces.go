package processor

import (
	"context"
	"time"

	"base-server/internal/store"

	"github.com/google/uuid"
)

// SpamStore defines the store interface required by the spam processor
type SpamStore interface {
	// GetWaitlistUserByID retrieves a waitlist user by ID
	GetWaitlistUserByID(ctx context.Context, userID uuid.UUID) (store.WaitlistUser, error)

	// CountRecentSignupsByIP counts signups from an IP within a time window
	CountRecentSignupsByIP(ctx context.Context, campaignID uuid.UUID, ip string, since time.Time) (int, error)

	// CreateFraudDetection creates a new fraud detection record
	CreateFraudDetection(ctx context.Context, params store.CreateFraudDetectionParams) (store.FraudDetection, error)

	// BlockWaitlistUser blocks a waitlist user
	BlockWaitlistUser(ctx context.Context, userID uuid.UUID) error
}
