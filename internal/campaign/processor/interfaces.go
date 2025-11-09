package processor

import (
	"base-server/internal/store"
	"context"

	"github.com/google/uuid"
)

// CampaignStore defines the database operations required by CampaignProcessor
type CampaignStore interface {
	GetCampaignBySlug(ctx context.Context, accountID uuid.UUID, slug string) (store.Campaign, error)
	CreateCampaign(ctx context.Context, params store.CreateCampaignParams) (store.Campaign, error)
	GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (store.Campaign, error)
	ListCampaigns(ctx context.Context, params store.ListCampaignsParams) (store.ListCampaignsResult, error)
	UpdateCampaign(ctx context.Context, accountID, campaignID uuid.UUID, params store.UpdateCampaignParams) (store.Campaign, error)
	UpdateCampaignStatus(ctx context.Context, accountID, campaignID uuid.UUID, status string) (store.Campaign, error)
	DeleteCampaign(ctx context.Context, accountID, campaignID uuid.UUID) error
}
