package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"base-server/internal/waitlist/utils"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrReferralNotFound  = errors.New("referral not found")
	ErrCampaignNotFound  = errors.New("campaign not found")
	ErrUserNotFound      = errors.New("user not found")
	ErrUnauthorized      = errors.New("unauthorized access to campaign")
	ErrInvalidStatus     = errors.New("invalid referral status")
	ErrInvalidReferral   = errors.New("invalid referral code")
	ErrReferralCodeEmpty = errors.New("referral code is required")
)

type ReferralProcessor struct {
	store  store.Store
	logger *observability.Logger
}

func New(store store.Store, logger *observability.Logger) ReferralProcessor {
	return ReferralProcessor{
		store:  store,
		logger: logger,
	}
}

// ListReferralsRequest represents parameters for listing referrals
type ListReferralsRequest struct {
	Status *string
	Page   int
	Limit  int
}

// ListReferralsResponse represents the paginated response for referrals
type ListReferralsResponse struct {
	Referrals []store.Referral `json:"referrals"`
	Pagination Pagination       `json:"pagination"`
}

// Pagination represents pagination metadata
type Pagination struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
	TotalCount int     `json:"total_count"`
}

// ListReferrals retrieves all referrals for a campaign with optional filters
func (p *ReferralProcessor) ListReferrals(ctx context.Context, accountID, campaignID uuid.UUID, req ListReferralsRequest) (ListReferralsResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	// Verify campaign belongs to account
	if err := p.verifyCampaignAccess(ctx, accountID, campaignID); err != nil {
		return ListReferralsResponse{}, err
	}

	// Validate and set defaults
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 || req.Limit > 100 {
		req.Limit = 20
	}

	// Validate status if provided
	if req.Status != nil && !isValidReferralStatus(*req.Status) {
		return ListReferralsResponse{}, ErrInvalidStatus
	}

	offset := (req.Page - 1) * req.Limit

	referrals, err := p.store.GetReferralsByCampaignWithStatusFilter(ctx, campaignID, req.Status, req.Limit, offset)
	if err != nil {
		p.logger.Error(ctx, "failed to list referrals", err)
		return ListReferralsResponse{}, err
	}

	// Ensure referrals is never null - return empty array instead
	if referrals == nil {
		referrals = []store.Referral{}
	}

	// Get total count
	totalCount, err := p.store.CountReferralsByCampaignWithStatusFilter(ctx, campaignID, req.Status)
	if err != nil {
		p.logger.Error(ctx, "failed to count referrals", err)
		return ListReferralsResponse{}, err
	}

	hasMore := (req.Page * req.Limit) < totalCount

	return ListReferralsResponse{
		Referrals: referrals,
		Pagination: Pagination{
			HasMore:    hasMore,
			TotalCount: totalCount,
		},
	}, nil
}

// TrackReferralRequest represents a request to track a referral click
type TrackReferralRequest struct {
	ReferralCode string
	Source       *string
	IPAddress    *string
}

// TrackReferralResponse represents the response after tracking a referral
type TrackReferralResponse struct {
	Message  string          `json:"message"`
	Referrer ReferrerDetails `json:"referrer"`
}

// ReferrerDetails contains basic info about the referrer
type ReferrerDetails struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// TrackReferral tracks when someone clicks on a referral link
func (p *ReferralProcessor) TrackReferral(ctx context.Context, campaignID uuid.UUID, req TrackReferralRequest) (TrackReferralResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "referral_code", Value: req.ReferralCode},
	)

	if req.ReferralCode == "" {
		return TrackReferralResponse{}, ErrReferralCodeEmpty
	}

	// Get referrer by referral code
	referrer, err := p.store.GetWaitlistUserByReferralCode(ctx, req.ReferralCode)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return TrackReferralResponse{}, ErrInvalidReferral
		}
		p.logger.Error(ctx, "failed to get referrer by code", err)
		return TrackReferralResponse{}, err
	}

	// Verify referrer is from same campaign
	if referrer.CampaignID != campaignID {
		return TrackReferralResponse{}, ErrInvalidReferral
	}

	// Build referrer name
	name := referrer.Email
	if referrer.FirstName != nil && referrer.LastName != nil {
		name = fmt.Sprintf("%s %s", *referrer.FirstName, *referrer.LastName)
	} else if referrer.FirstName != nil {
		name = *referrer.FirstName
	}

	p.logger.Info(ctx, "referral tracked successfully")

	return TrackReferralResponse{
		Message: "Referral tracked successfully",
		Referrer: ReferrerDetails{
			ID:   referrer.ID,
			Name: name,
		},
	}, nil
}

// GetUserReferralsRequest represents parameters for getting user's referrals
type GetUserReferralsRequest struct {
	Page  int
	Limit int
}

// GetUserReferralsResponse represents user's referrals
type GetUserReferralsResponse struct {
	Referrals         []store.Referral `json:"referrals"`
	TotalReferrals    int              `json:"total_referrals"`
	VerifiedReferrals int              `json:"verified_referrals"`
	Pagination        Pagination       `json:"pagination"`
}

// GetUserReferrals retrieves all referrals made by a specific user
func (p *ReferralProcessor) GetUserReferrals(ctx context.Context, accountID, campaignID, userID uuid.UUID, req GetUserReferralsRequest) (GetUserReferralsResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
	)

	// Verify campaign belongs to account
	if err := p.verifyCampaignAccess(ctx, accountID, campaignID); err != nil {
		return GetUserReferralsResponse{}, err
	}

	// Verify user exists and belongs to campaign
	user, err := p.store.GetWaitlistUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return GetUserReferralsResponse{}, ErrUserNotFound
		}
		p.logger.Error(ctx, "failed to get user", err)
		return GetUserReferralsResponse{}, err
	}

	if user.CampaignID != campaignID {
		return GetUserReferralsResponse{}, ErrUserNotFound
	}

	// Validate and set defaults
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 || req.Limit > 100 {
		req.Limit = 20
	}

	offset := (req.Page - 1) * req.Limit

	referrals, err := p.store.GetReferralsByReferrerWithPagination(ctx, userID, req.Limit, offset)
	if err != nil {
		p.logger.Error(ctx, "failed to get user referrals", err)
		return GetUserReferralsResponse{}, err
	}

	// Get total count
	totalCount, err := p.store.CountReferralsByReferrer(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to count user referrals", err)
		return GetUserReferralsResponse{}, err
	}

	// Get verified count
	verifiedCount, err := p.store.GetVerifiedReferralCountByReferrer(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get verified referral count", err)
		return GetUserReferralsResponse{}, err
	}

	hasMore := (req.Page * req.Limit) < totalCount

	return GetUserReferralsResponse{
		Referrals:         referrals,
		TotalReferrals:    totalCount,
		VerifiedReferrals: verifiedCount,
		Pagination: Pagination{
			HasMore:    hasMore,
			TotalCount: totalCount,
		},
	}, nil
}

// GetReferralLinkResponse represents user's referral link and share options
type GetReferralLinkResponse struct {
	ReferralLink string     `json:"referral_link"`
	ReferralCode string     `json:"referral_code"`
	ShareLinks   ShareLinks `json:"share_links"`
}

// ShareLinks contains pre-formatted sharing links for various platforms
type ShareLinks struct {
	Twitter  string `json:"twitter"`
	Facebook string `json:"facebook"`
	LinkedIn string `json:"linkedin"`
	WhatsApp string `json:"whatsapp"`
	Email    string `json:"email"`
}

// GetReferralLink retrieves the unique referral link for a user
func (p *ReferralProcessor) GetReferralLink(ctx context.Context, accountID, campaignID, userID uuid.UUID, baseURL string) (GetReferralLinkResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
	)

	// Verify campaign belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return GetReferralLinkResponse{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return GetReferralLinkResponse{}, err
	}

	if campaign.AccountID != accountID {
		return GetReferralLinkResponse{}, ErrUnauthorized
	}

	// Verify user exists and belongs to campaign
	user, err := p.store.GetWaitlistUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return GetReferralLinkResponse{}, ErrUserNotFound
		}
		p.logger.Error(ctx, "failed to get user", err)
		return GetReferralLinkResponse{}, err
	}

	if user.CampaignID != campaignID {
		return GetReferralLinkResponse{}, ErrUserNotFound
	}

	// Build referral link
	referralLink := utils.BuildReferralLink(baseURL, campaign.Slug, user.ReferralCode)

	// Get share message from campaign config
	shareMessage := getShareMessage(campaign, user)

	// Build social media share links
	shareLinks := buildShareLinks(referralLink, shareMessage)

	return GetReferralLinkResponse{
		ReferralLink: referralLink,
		ReferralCode: user.ReferralCode,
		ShareLinks:   shareLinks,
	}, nil
}

// Helper functions

func (p *ReferralProcessor) verifyCampaignAccess(ctx context.Context, accountID, campaignID uuid.UUID) error {
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return err
	}

	if campaign.AccountID != accountID {
		return ErrUnauthorized
	}

	return nil
}

func isValidReferralStatus(status string) bool {
	validStatuses := map[string]bool{
		"pending":   true,
		"verified":  true,
		"converted": true,
		"invalid":   true,
	}
	return validStatuses[status]
}

func getShareMessage(campaign store.Campaign, user store.WaitlistUser) string {
	// Try to get custom share message from campaign config
	if campaign.ReferralConfig != nil {
		if customMessages, ok := campaign.ReferralConfig["custom_share_messages"].(map[string]interface{}); ok {
			if message, ok := customMessages["default"].(string); ok {
				return message
			}
		}
	}

	// Default message
	return fmt.Sprintf("Join me on the %s waitlist!", campaign.Name)
}

func buildShareLinks(referralLink, message string) ShareLinks {
	// URL encode the message and link
	encodedMessage := fmt.Sprintf("%s %s", message, referralLink)

	return ShareLinks{
		Twitter:  fmt.Sprintf("https://twitter.com/intent/tweet?text=%s", encodedMessage),
		Facebook: fmt.Sprintf("https://www.facebook.com/sharer/sharer.php?u=%s", referralLink),
		LinkedIn: fmt.Sprintf("https://www.linkedin.com/sharing/share-offsite/?url=%s", referralLink),
		WhatsApp: fmt.Sprintf("https://wa.me/?text=%s", encodedMessage),
		Email:    fmt.Sprintf("mailto:?subject=%s&body=%s", message, referralLink),
	}
}
