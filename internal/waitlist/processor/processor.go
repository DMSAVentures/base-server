package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"base-server/internal/waitlist/utils"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound             = errors.New("user not found")
	ErrCampaignNotFound         = errors.New("campaign not found")
	ErrEmailAlreadyExists       = errors.New("email already exists for this campaign")
	ErrInvalidReferralCode      = errors.New("invalid referral code")
	ErrInvalidStatus            = errors.New("invalid user status")
	ErrUnauthorized             = errors.New("unauthorized access to campaign")
	ErrInvalidVerificationToken = errors.New("invalid verification token")
	ErrEmailAlreadyVerified     = errors.New("email already verified")
	ErrMaxSignupsReached        = errors.New("campaign has reached maximum signups")
)

type WaitlistProcessor struct {
	store  store.Store
	logger *observability.Logger
}

func New(store store.Store, logger *observability.Logger) WaitlistProcessor {
	return WaitlistProcessor{
		store:  store,
		logger: logger,
	}
}

// SignupUserRequest represents a request to sign up a user to a waitlist
type SignupUserRequest struct {
	Email            string
	FirstName        *string
	LastName         *string
	ReferralCode     *string
	CustomFields     map[string]string
	UTMSource        *string
	UTMMedium        *string
	UTMCampaign      *string
	UTMTerm          *string
	UTMContent       *string
	MarketingConsent bool
	TermsAccepted    bool
	IPAddress        *string
	UserAgent        *string
}

// SignupUserResponse represents the response after signing up a user
type SignupUserResponse struct {
	User         store.WaitlistUser `json:"user"`
	Position     int                `json:"position"`
	ReferralLink string             `json:"referral_link"`
	Message      string             `json:"message"`
}

// SignupUser handles the complete signup process for a waitlist user
func (p *WaitlistProcessor) SignupUser(ctx context.Context, accountID, campaignID uuid.UUID, req SignupUserRequest, baseURL string) (SignupUserResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "email", Value: req.Email},
	)

	// Validate campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return SignupUserResponse{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return SignupUserResponse{}, err
	}

	if campaign.AccountID != accountID {
		return SignupUserResponse{}, ErrUnauthorized
	}

	// Check if email already exists for this campaign
	existingUser, err := p.store.GetWaitlistUserByEmail(ctx, campaignID, req.Email)
	if err == nil && existingUser.ID != uuid.Nil {
		return SignupUserResponse{}, ErrEmailAlreadyExists
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		p.logger.Error(ctx, "failed to check email existence", err)
		return SignupUserResponse{}, err
	}

	// Check max signups limit
	if campaign.MaxSignups != nil && campaign.TotalSignups >= *campaign.MaxSignups {
		return SignupUserResponse{}, ErrMaxSignupsReached
	}

	// Handle referral code if provided
	var referredByID *uuid.UUID
	var source *string
	defaultSource := "direct"

	if req.ReferralCode != nil && *req.ReferralCode != "" {
		referrer, err := p.store.GetWaitlistUserByReferralCode(ctx, *req.ReferralCode)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return SignupUserResponse{}, ErrInvalidReferralCode
			}
			p.logger.Error(ctx, "failed to get referrer by code", err)
			return SignupUserResponse{}, err
		}

		// Verify referrer is from same campaign
		if referrer.CampaignID != campaignID {
			return SignupUserResponse{}, ErrInvalidReferralCode
		}

		referredByID = &referrer.ID
		referralSource := "referral"
		source = &referralSource
	} else {
		source = &defaultSource
	}

	// Generate unique referral code
	referralCode, err := utils.GenerateReferralCode(8)
	if err != nil {
		p.logger.Error(ctx, "failed to generate referral code", err)
		return SignupUserResponse{}, fmt.Errorf("failed to generate referral code: %w", err)
	}

	// Generate verification token
	verificationToken, err := utils.GenerateVerificationToken()
	if err != nil {
		p.logger.Error(ctx, "failed to generate verification token", err)
		return SignupUserResponse{}, fmt.Errorf("failed to generate verification token: %w", err)
	}

	// Calculate position (total current users + 1)
	currentCount, err := p.store.CountWaitlistUsersByCampaign(ctx, campaignID)
	if err != nil {
		p.logger.Error(ctx, "failed to count users", err)
		return SignupUserResponse{}, err
	}
	position := currentCount + 1

	// Convert custom fields to JSONB metadata
	metadata := store.JSONB{}
	if req.CustomFields != nil {
		for k, v := range req.CustomFields {
			metadata[k] = v
		}
	}

	// Create user
	createParams := store.CreateWaitlistUserParams{
		CampaignID:        campaignID,
		Email:             strings.ToLower(strings.TrimSpace(req.Email)),
		FirstName:         req.FirstName,
		LastName:          req.LastName,
		ReferralCode:      referralCode,
		ReferredByID:      referredByID,
		Position:          position,
		OriginalPosition:  position,
		Source:            source,
		UTMSource:         req.UTMSource,
		UTMMedium:         req.UTMMedium,
		UTMCampaign:       req.UTMCampaign,
		UTMTerm:           req.UTMTerm,
		UTMContent:        req.UTMContent,
		IPAddress:         req.IPAddress,
		UserAgent:         req.UserAgent,
		Metadata:          metadata,
		MarketingConsent:  req.MarketingConsent,
		TermsAccepted:     req.TermsAccepted,
		VerificationToken: &verificationToken,
	}

	user, err := p.store.CreateWaitlistUser(ctx, createParams)
	if err != nil {
		p.logger.Error(ctx, "failed to create waitlist user", err)
		return SignupUserResponse{}, err
	}

	// If user was referred, increment referrer's count
	if referredByID != nil {
		if err := p.store.IncrementReferralCount(ctx, *referredByID); err != nil {
			p.logger.Error(ctx, "failed to increment referral count", err)
			// Don't fail the signup, just log the error
		}
	}

	// Build referral link
	referralLink := utils.BuildReferralLink(baseURL, campaign.Slug, referralCode)

	p.logger.Info(ctx, "user signed up successfully")

	return SignupUserResponse{
		User:         user,
		Position:     position,
		ReferralLink: referralLink,
		Message:      "Successfully joined the waitlist! Please check your email to verify your address.",
	}, nil
}

// ListUsersRequest represents parameters for listing users
type ListUsersRequest struct {
	Status    *string
	Verified  *bool
	SortBy    string
	SortOrder string
	Page      int
	Limit     int
}

// ListUsersResponse represents the paginated response
type ListUsersResponse struct {
	Users      []store.WaitlistUser `json:"users"`
	TotalCount int                  `json:"total_count"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"page_size"`
	TotalPages int                  `json:"total_pages"`
}

// ListUsers retrieves waitlist users with filters and pagination
func (p *WaitlistProcessor) ListUsers(ctx context.Context, accountID, campaignID uuid.UUID, req ListUsersRequest) (ListUsersResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	// Verify campaign belongs to account
	if err := p.verifyCampaignAccess(ctx, accountID, campaignID); err != nil {
		return ListUsersResponse{}, err
	}

	// Validate and set defaults
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 || req.Limit > 100 {
		req.Limit = 20
	}

	// Validate status if provided
	if req.Status != nil && !isValidUserStatus(*req.Status) {
		return ListUsersResponse{}, ErrInvalidStatus
	}

	offset := (req.Page - 1) * req.Limit

	params := store.ListWaitlistUsersWithFiltersParams{
		CampaignID: campaignID,
		Status:     req.Status,
		Verified:   req.Verified,
		SortBy:     req.SortBy,
		SortOrder:  req.SortOrder,
		Limit:      req.Limit,
		Offset:     offset,
	}

	users, err := p.store.GetWaitlistUsersByCampaignWithFilters(ctx, params)
	if err != nil {
		p.logger.Error(ctx, "failed to list users", err)
		return ListUsersResponse{}, err
	}

	// Get total count
	totalCount, err := p.store.CountWaitlistUsersWithFilters(ctx, campaignID, req.Status, req.Verified)
	if err != nil {
		p.logger.Error(ctx, "failed to count users", err)
		return ListUsersResponse{}, err
	}

	totalPages := (totalCount + req.Limit - 1) / req.Limit

	return ListUsersResponse{
		Users:      users,
		TotalCount: totalCount,
		Page:       req.Page,
		PageSize:   req.Limit,
		TotalPages: totalPages,
	}, nil
}

// GetUser retrieves a single user by ID
func (p *WaitlistProcessor) GetUser(ctx context.Context, accountID, campaignID, userID uuid.UUID) (store.WaitlistUser, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
	)

	// Verify campaign belongs to account
	if err := p.verifyCampaignAccess(ctx, accountID, campaignID); err != nil {
		return store.WaitlistUser{}, err
	}

	user, err := p.store.GetWaitlistUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.WaitlistUser{}, ErrUserNotFound
		}
		p.logger.Error(ctx, "failed to get user", err)
		return store.WaitlistUser{}, err
	}

	// Verify user belongs to campaign
	if user.CampaignID != campaignID {
		return store.WaitlistUser{}, ErrUserNotFound
	}

	return user, nil
}

// UpdateUserRequest represents parameters for updating a user
type UpdateUserRequest struct {
	FirstName *string
	LastName  *string
	Status    *string
	Metadata  map[string]interface{}
}

// UpdateUser updates a waitlist user
func (p *WaitlistProcessor) UpdateUser(ctx context.Context, accountID, campaignID, userID uuid.UUID, req UpdateUserRequest) (store.WaitlistUser, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
	)

	// Verify campaign belongs to account
	if err := p.verifyCampaignAccess(ctx, accountID, campaignID); err != nil {
		return store.WaitlistUser{}, err
	}

	// Verify user exists and belongs to campaign
	existingUser, err := p.store.GetWaitlistUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.WaitlistUser{}, ErrUserNotFound
		}
		p.logger.Error(ctx, "failed to get user", err)
		return store.WaitlistUser{}, err
	}

	if existingUser.CampaignID != campaignID {
		return store.WaitlistUser{}, ErrUserNotFound
	}

	// Validate status if provided
	if req.Status != nil && !isValidUserStatus(*req.Status) {
		return store.WaitlistUser{}, ErrInvalidStatus
	}

	// Convert metadata
	var metadata store.JSONB
	if req.Metadata != nil {
		metadata = store.JSONB(req.Metadata)
	}

	params := store.UpdateWaitlistUserParams{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Status:    req.Status,
		Metadata:  metadata,
	}

	user, err := p.store.UpdateWaitlistUser(ctx, userID, params)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.WaitlistUser{}, ErrUserNotFound
		}
		p.logger.Error(ctx, "failed to update user", err)
		return store.WaitlistUser{}, err
	}

	p.logger.Info(ctx, "user updated successfully")
	return user, nil
}

// DeleteUser removes a user from the waitlist
func (p *WaitlistProcessor) DeleteUser(ctx context.Context, accountID, campaignID, userID uuid.UUID) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
	)

	// Verify campaign belongs to account
	if err := p.verifyCampaignAccess(ctx, accountID, campaignID); err != nil {
		return err
	}

	// Verify user exists and belongs to campaign
	user, err := p.store.GetWaitlistUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrUserNotFound
		}
		p.logger.Error(ctx, "failed to get user", err)
		return err
	}

	if user.CampaignID != campaignID {
		return ErrUserNotFound
	}

	err = p.store.DeleteWaitlistUser(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrUserNotFound
		}
		p.logger.Error(ctx, "failed to delete user", err)
		return err
	}

	p.logger.Info(ctx, "user deleted successfully")
	return nil
}

// SearchUsersRequest represents parameters for advanced search
type SearchUsersRequest struct {
	Query        *string
	Statuses     []string
	Verified     *bool
	MinReferrals *int
	DateFrom     *string
	DateTo       *string
	Page         int
	Limit        int
}

// SearchUsers performs advanced search with multiple filters
func (p *WaitlistProcessor) SearchUsers(ctx context.Context, accountID, campaignID uuid.UUID, req SearchUsersRequest) (ListUsersResponse, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	// Verify campaign belongs to account
	if err := p.verifyCampaignAccess(ctx, accountID, campaignID); err != nil {
		return ListUsersResponse{}, err
	}

	// Validate and set defaults
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 || req.Limit > 100 {
		req.Limit = 20
	}

	// Validate statuses if provided
	for _, status := range req.Statuses {
		if !isValidUserStatus(status) {
			return ListUsersResponse{}, ErrInvalidStatus
		}
	}

	offset := (req.Page - 1) * req.Limit

	params := store.SearchWaitlistUsersParams{
		CampaignID:   campaignID,
		Query:        req.Query,
		Statuses:     req.Statuses,
		Verified:     req.Verified,
		MinReferrals: req.MinReferrals,
		DateFrom:     req.DateFrom,
		DateTo:       req.DateTo,
		SortBy:       "position",
		SortOrder:    "asc",
		Limit:        req.Limit,
		Offset:       offset,
	}

	users, err := p.store.SearchWaitlistUsers(ctx, params)
	if err != nil {
		p.logger.Error(ctx, "failed to search users", err)
		return ListUsersResponse{}, err
	}

	// For search, we'll return the current page results
	// In a production system, you might want to count total results
	totalCount := len(users)
	if len(users) == req.Limit {
		totalCount = req.Page * req.Limit // Estimate
	}

	return ListUsersResponse{
		Users:      users,
		TotalCount: totalCount,
		Page:       req.Page,
		PageSize:   req.Limit,
		TotalPages: (totalCount + req.Limit - 1) / req.Limit,
	}, nil
}

// VerifyUser manually verifies a user's email
func (p *WaitlistProcessor) VerifyUser(ctx context.Context, accountID, campaignID, userID uuid.UUID) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
	)

	// Verify campaign belongs to account
	if err := p.verifyCampaignAccess(ctx, accountID, campaignID); err != nil {
		return err
	}

	// Verify user exists and belongs to campaign
	user, err := p.store.GetWaitlistUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrUserNotFound
		}
		p.logger.Error(ctx, "failed to get user", err)
		return err
	}

	if user.CampaignID != campaignID {
		return ErrUserNotFound
	}

	if user.EmailVerified {
		return ErrEmailAlreadyVerified
	}

	err = p.store.VerifyWaitlistUserEmail(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrUserNotFound
		}
		p.logger.Error(ctx, "failed to verify user email", err)
		return err
	}

	// If user was referred, increment verified referral count for referrer
	if user.ReferredByID != nil {
		if err := p.store.IncrementVerifiedReferralCount(ctx, *user.ReferredByID); err != nil {
			p.logger.Error(ctx, "failed to increment verified referral count", err)
			// Don't fail the verification, just log
		}
	}

	p.logger.Info(ctx, "user email verified successfully")
	return nil
}

// ResendVerificationToken generates a new verification token and updates the user
func (p *WaitlistProcessor) ResendVerificationToken(ctx context.Context, accountID, campaignID, userID uuid.UUID) (string, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "user_id", Value: userID.String()},
	)

	// Verify campaign belongs to account
	if err := p.verifyCampaignAccess(ctx, accountID, campaignID); err != nil {
		return "", err
	}

	// Verify user exists and belongs to campaign
	user, err := p.store.GetWaitlistUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", ErrUserNotFound
		}
		p.logger.Error(ctx, "failed to get user", err)
		return "", err
	}

	if user.CampaignID != campaignID {
		return "", ErrUserNotFound
	}

	if user.EmailVerified {
		return "", ErrEmailAlreadyVerified
	}

	// Generate new verification token
	token, err := utils.GenerateVerificationToken()
	if err != nil {
		p.logger.Error(ctx, "failed to generate verification token", err)
		return "", fmt.Errorf("failed to generate verification token: %w", err)
	}

	err = p.store.UpdateVerificationToken(ctx, userID, token)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", ErrUserNotFound
		}
		p.logger.Error(ctx, "failed to update verification token", err)
		return "", err
	}

	p.logger.Info(ctx, "verification token updated successfully")
	return token, nil
}

// Helper functions

func (p *WaitlistProcessor) verifyCampaignAccess(ctx context.Context, accountID, campaignID uuid.UUID) error {
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

func isValidUserStatus(status string) bool {
	validStatuses := map[string]bool{
		store.WaitlistUserStatusPending:   true,
		store.WaitlistUserStatusVerified:  true,
		store.WaitlistUserStatusConverted: true,
		store.WaitlistUserStatusRemoved:   true,
		store.WaitlistUserStatusBlocked:   true,
	}
	return validStatuses[status]
}
