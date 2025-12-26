package processor

//go:generate go run go.uber.org/mock/mockgen@latest -source=processor.go -destination=mocks_test.go -package=processor

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

// WaitlistStore defines the database operations required by WaitlistProcessor
type WaitlistStore interface {
	GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (store.Campaign, error)
	GetWaitlistUserByEmail(ctx context.Context, campaignID uuid.UUID, email string) (store.WaitlistUser, error)
	GetWaitlistUserByReferralCode(ctx context.Context, referralCode string) (store.WaitlistUser, error)
	GetWaitlistUserByVerificationToken(ctx context.Context, token string) (store.WaitlistUser, error)
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
	// Position calculation methods
	GetAllWaitlistUsersForPositionCalculation(ctx context.Context, campaignID uuid.UUID) ([]store.WaitlistUser, error)
	BulkUpdateWaitlistUserPositions(ctx context.Context, userIDs []uuid.UUID, positions []int) error
	// Channel code methods
	CreateUserChannelCodes(ctx context.Context, userID uuid.UUID, codes map[string]string) ([]store.UserChannelCode, error)
	GetUserByChannelCode(ctx context.Context, code string) (*store.WaitlistUser, string, error)
}

// EventDispatcher defines the event operations required by WaitlistProcessor
type EventDispatcher interface {
	DispatchUserCreated(ctx context.Context, accountID, campaignID uuid.UUID, userData map[string]interface{})
	DispatchUserVerified(ctx context.Context, accountID, campaignID uuid.UUID, userData map[string]interface{})
}

// CaptchaVerifier defines the captcha verification operations
type CaptchaVerifier interface {
	Verify(ctx context.Context, token string, remoteIP string) error
	IsEnabled() bool
}

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
	ErrCaptchaRequired          = errors.New("captcha verification required")
	ErrCaptchaFailed            = errors.New("captcha verification failed")
	ErrCampaignNotActive        = errors.New("campaign is not accepting signups")
)

type WaitlistProcessor struct {
	store           WaitlistStore
	logger          *observability.Logger
	eventDispatcher EventDispatcher
	captchaVerifier CaptchaVerifier
}

func New(store WaitlistStore, logger *observability.Logger, eventDispatcher EventDispatcher, captchaVerifier CaptchaVerifier) WaitlistProcessor {
	return WaitlistProcessor{
		store:           store,
		logger:          logger,
		eventDispatcher: eventDispatcher,
		captchaVerifier: captchaVerifier,
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
	CaptchaToken     *string
}

// SignupUserResponse represents the response after signing up a user
type SignupUserResponse struct {
	User          store.WaitlistUser `json:"user"`
	Position      int                `json:"position"`
	ReferralLink  string             `json:"referral_link"`
	ReferralCodes map[string]string  `json:"referral_codes,omitempty"`
	Message       string             `json:"message"`
}

// SignupUser handles the complete signup process for a waitlist user
func (p *WaitlistProcessor) SignupUser(ctx context.Context, campaignID uuid.UUID, req SignupUserRequest, baseURL string) (SignupUserResponse, error) {
	ctx = observability.WithFields(ctx,
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
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: campaign.AccountID.String()})

	// Check campaign status - only active campaigns accept signups
	if campaign.Status != store.CampaignStatusActive {
		return SignupUserResponse{}, ErrCampaignNotActive
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

	// Check captcha if enabled for this campaign
	if p.captchaVerifier != nil && p.captchaVerifier.IsEnabled() && campaign.FormSettings != nil && campaign.FormSettings.CaptchaEnabled {
		// Captcha is required for this campaign
		if req.CaptchaToken == nil || *req.CaptchaToken == "" {
			return SignupUserResponse{}, ErrCaptchaRequired
		}
		ipAddress := ""
		if req.IPAddress != nil {
			ipAddress = *req.IPAddress
		}
		if err := p.captchaVerifier.Verify(ctx, *req.CaptchaToken, ipAddress); err != nil {
			p.logger.Info(ctx, "captcha verification failed")
			return SignupUserResponse{}, ErrCaptchaFailed
		}
	}

	// Handle referral code if provided
	var referredByID *uuid.UUID
	var source *string
	defaultSource := "direct"

	if req.ReferralCode != nil && *req.ReferralCode != "" {
		// First, try to find referrer by channel code (new format: ABC123_TW)
		referrer, channel, err := p.store.GetUserByChannelCode(ctx, *req.ReferralCode)
		if err != nil {
			p.logger.Error(ctx, "failed to get referrer by channel code", err)
			return SignupUserResponse{}, err
		}

		if referrer != nil && referrer.CampaignID == campaignID {
			// Valid channel code - use channel as source
			referredByID = &referrer.ID
			source = &channel
		} else {
			// Fall back to legacy referral code lookup
			legacyReferrer, err := p.store.GetWaitlistUserByReferralCode(ctx, *req.ReferralCode)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					// Invalid referral code - silently ignore and treat as direct signup
					p.logger.Info(ctx, "invalid referral code provided, treating as direct signup")
					source = &defaultSource
				} else {
					p.logger.Error(ctx, "failed to get referrer by code", err)
					return SignupUserResponse{}, err
				}
			} else if legacyReferrer.CampaignID != campaignID {
				// Referrer is from different campaign - silently ignore
				p.logger.Info(ctx, "referral code from different campaign, treating as direct signup")
				source = &defaultSource
			} else {
				// Valid legacy referral code
				referredByID = &legacyReferrer.ID
				referralSource := "referral"
				source = &referralSource
			}
		}
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

	// Position will be calculated asynchronously by the position calculation worker
	// Set to -1 to indicate "calculating" or "not yet calculated"
	position := -1

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

	// Generate channel-specific referral codes if campaign has sharing channels
	var referralCodes map[string]string
	if campaign.ReferralSettings != nil && len(campaign.ReferralSettings.SharingChannels) > 0 {
		// Convert SharingChannel slice to string slice
		channels := make([]string, 0, len(campaign.ReferralSettings.SharingChannels))
		for _, ch := range campaign.ReferralSettings.SharingChannels {
			channels = append(channels, string(ch))
		}

		// Generate channel codes
		codes, err := utils.GenerateChannelCodes(channels)
		if err != nil {
			p.logger.Error(ctx, "failed to generate channel codes", err)
			// Don't fail the signup, just log the error
		} else if len(codes) > 0 {
			// Store channel codes in database
			_, err := p.store.CreateUserChannelCodes(ctx, user.ID, codes)
			if err != nil {
				p.logger.Error(ctx, "failed to store channel codes", err)
				// Don't fail the signup, just log the error
			} else {
				referralCodes = codes
			}
		}
	}

	// Dispatch user.created event for email notifications
	if p.eventDispatcher != nil {
		userData := map[string]interface{}{
			"id":                 user.ID.String(),
			"email":              user.Email,
			"first_name":         user.FirstName,
			"last_name":          user.LastName,
			"position":           user.Position,
			"referral_code":      user.ReferralCode,
			"referral_link":      referralLink,
			"verification_token": user.VerificationToken,
			"email_verified":     user.EmailVerified,
			"campaign_name":      campaign.Name,
			"campaign_slug":      campaign.Slug,
		}
		p.eventDispatcher.DispatchUserCreated(ctx, campaign.AccountID, campaign.ID, userData)
	}

	p.logger.Info(ctx, "user signed up successfully")

	return SignupUserResponse{
		User:          user,
		Position:      position,
		ReferralLink:  referralLink,
		ReferralCodes: referralCodes,
		Message:       "Successfully joined the waitlist! Please check your email to verify your address.",
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

	// Ensure users is never null - return empty array instead
	if users == nil {
		users = []store.WaitlistUser{}
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

// VerifyCampaignOwnership verifies that a campaign belongs to an account (exported for handler use)
func (p *WaitlistProcessor) VerifyCampaignOwnership(ctx context.Context, accountID, campaignID uuid.UUID) error {
	return p.verifyCampaignAccess(ctx, accountID, campaignID)
}

// VerifyUserByToken verifies a user's email using the verification token (public endpoint)
func (p *WaitlistProcessor) VerifyUserByToken(ctx context.Context, campaignID uuid.UUID, token string) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	// Look up user by verification token
	user, err := p.store.GetWaitlistUserByVerificationToken(ctx, token)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrInvalidVerificationToken
		}
		p.logger.Error(ctx, "failed to get user by verification token", err)
		return err
	}

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "user_id", Value: user.ID.String()},
	)

	// Verify user belongs to the specified campaign
	if user.CampaignID != campaignID {
		return ErrInvalidVerificationToken
	}

	// Check if already verified
	if user.EmailVerified {
		return ErrEmailAlreadyVerified
	}

	// Verify the email
	err = p.store.VerifyWaitlistUserEmail(ctx, user.ID)
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

	// Get campaign for event dispatch
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		p.logger.Error(ctx, "failed to get campaign for event dispatch", err)
		// Don't fail the verification, just log
	} else if p.eventDispatcher != nil {
		// Dispatch user.verified event
		userData := map[string]interface{}{
			"id":            user.ID.String(),
			"email":         user.Email,
			"first_name":    user.FirstName,
			"last_name":     user.LastName,
			"position":      user.Position,
			"referral_code": user.ReferralCode,
			"campaign_name": campaign.Name,
			"campaign_slug": campaign.Slug,
		}
		p.eventDispatcher.DispatchUserVerified(ctx, campaign.AccountID, campaign.ID, userData)
	}

	p.logger.Info(ctx, "user email verified successfully via token")
	return nil
}
