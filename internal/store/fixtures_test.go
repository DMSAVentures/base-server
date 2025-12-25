package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Fixtures provides factory functions for creating test data.
// All factory methods use testify/require to fail fast on errors.
type Fixtures struct {
	t      *testing.T
	testDB *TestDB
	ctx    context.Context
}

// NewFixtures creates a new Fixtures instance for test data generation.
func NewFixtures(t *testing.T, testDB *TestDB) *Fixtures {
	t.Helper()
	return &Fixtures{
		t:      t,
		testDB: testDB,
		ctx:    context.Background(),
	}
}

// --- User Fixtures ---

// UserOpts customizes user creation.
type UserOpts struct {
	FirstName string
	LastName  string
}

// DefaultUserOpts returns sensible defaults for user creation.
func DefaultUserOpts() UserOpts {
	return UserOpts{
		FirstName: "Test",
		LastName:  "User",
	}
}

// CreateUser creates a test user with optional customization.
// Uses raw SQL since there's no direct CreateUser store method.
func (f *Fixtures) CreateUser(opts ...func(*UserOpts)) User {
	f.t.Helper()
	o := DefaultUserOpts()
	for _, fn := range opts {
		fn(&o)
	}

	var user User
	query := `INSERT INTO users (first_name, last_name) VALUES ($1, $2) RETURNING id, first_name, last_name`
	err := f.testDB.GetDB().GetContext(f.ctx, &user, query, o.FirstName, o.LastName)
	require.NoError(f.t, err, "failed to create test user")
	return user
}

// --- Backward Compatible Helpers ---
// These functions maintain compatibility with existing test files.
// Note: createTestUser is already defined in emailauth_test.go

// createTestAccount creates a test account (backward compatible helper).
func createTestAccount(t *testing.T, testDB *TestDB) Account {
	t.Helper()
	f := NewFixtures(t, testDB)
	return f.CreateAccount()
}

// createTestCampaign creates a test campaign (backward compatible helper).
func createTestCampaign(t *testing.T, testDB *TestDB, accountID uuid.UUID, name, slug string) Campaign {
	t.Helper()
	f := NewFixtures(t, testDB)
	return f.CreateCampaign(func(o *CampaignOpts) {
		o.AccountID = &accountID
		o.Name = name
		o.Slug = slug
	})
}

// createTestWaitlistUser creates a test waitlist user (backward compatible helper).
func createTestWaitlistUser(t *testing.T, testDB *TestDB, campaignID uuid.UUID, email string) WaitlistUser {
	t.Helper()
	f := NewFixtures(t, testDB)
	return f.CreateWaitlistUser(func(o *WaitlistUserOpts) {
		o.CampaignID = &campaignID
		o.Email = email
	})
}

// --- Account Fixtures ---

// AccountOpts customizes account creation.
type AccountOpts struct {
	Name        string
	Slug        string
	Plan        string
	OwnerUserID *uuid.UUID
}

// DefaultAccountOpts returns sensible defaults for account creation.
func DefaultAccountOpts() AccountOpts {
	return AccountOpts{
		Name: "Test Account",
		Slug: "test-account-" + uuid.New().String()[:8],
		Plan: "pro",
	}
}

// CreateAccount creates a test account with optional customization.
// If no owner is specified, a new user will be created.
func (f *Fixtures) CreateAccount(opts ...func(*AccountOpts)) Account {
	f.t.Helper()
	o := DefaultAccountOpts()
	for _, fn := range opts {
		fn(&o)
	}

	var ownerID uuid.UUID
	if o.OwnerUserID != nil {
		ownerID = *o.OwnerUserID
	} else {
		user := f.CreateUser()
		ownerID = user.ID
	}

	account, err := f.testDB.Store.CreateAccount(f.ctx, CreateAccountParams{
		Name:        o.Name,
		Slug:        o.Slug,
		OwnerUserID: ownerID,
		Plan:        o.Plan,
	})
	require.NoError(f.t, err, "failed to create test account")
	return account
}

// --- Campaign Fixtures ---

// CampaignOpts customizes campaign creation.
type CampaignOpts struct {
	AccountID        *uuid.UUID
	Name             string
	Slug             string
	Type             string
	Description      *string
	MaxSignups       *int
	PrivacyPolicyURL *string
	TermsURL         *string
}

// DefaultCampaignOpts returns sensible defaults for campaign creation.
func DefaultCampaignOpts() CampaignOpts {
	return CampaignOpts{
		Name: "Test Campaign",
		Slug: "test-campaign-" + uuid.New().String()[:8],
		Type: "waitlist",
	}
}

// CreateCampaign creates a test campaign with optional customization.
// If no account is specified, a new account will be created.
func (f *Fixtures) CreateCampaign(opts ...func(*CampaignOpts)) Campaign {
	f.t.Helper()
	o := DefaultCampaignOpts()
	for _, fn := range opts {
		fn(&o)
	}

	var accountID uuid.UUID
	if o.AccountID != nil {
		accountID = *o.AccountID
	} else {
		account := f.CreateAccount()
		accountID = account.ID
	}

	campaign, err := f.testDB.Store.CreateCampaign(f.ctx, CreateCampaignParams{
		AccountID:        accountID,
		Name:             o.Name,
		Slug:             o.Slug,
		Type:             o.Type,
		Description:      o.Description,
		MaxSignups:       o.MaxSignups,
		PrivacyPolicyURL: o.PrivacyPolicyURL,
		TermsURL:         o.TermsURL,
	})
	require.NoError(f.t, err, "failed to create test campaign")
	return campaign
}

// --- Waitlist User Fixtures ---

// WaitlistUserOpts customizes waitlist user creation.
type WaitlistUserOpts struct {
	CampaignID       *uuid.UUID
	Email            string
	ReferralCode     string
	Position         int
	OriginalPosition int
	TermsAccepted    bool
}

// DefaultWaitlistUserOpts returns sensible defaults for waitlist user creation.
func DefaultWaitlistUserOpts() WaitlistUserOpts {
	return WaitlistUserOpts{
		Email:            "test-" + uuid.New().String()[:8] + "@example.com",
		ReferralCode:     "TEST" + uuid.New().String()[:6],
		Position:         1,
		OriginalPosition: 1,
		TermsAccepted:    true,
	}
}

// CreateWaitlistUser creates a test waitlist user with optional customization.
// If no campaign is specified, a new campaign will be created.
func (f *Fixtures) CreateWaitlistUser(opts ...func(*WaitlistUserOpts)) WaitlistUser {
	f.t.Helper()
	o := DefaultWaitlistUserOpts()
	for _, fn := range opts {
		fn(&o)
	}

	var campaignID uuid.UUID
	if o.CampaignID != nil {
		campaignID = *o.CampaignID
	} else {
		campaign := f.CreateCampaign()
		campaignID = campaign.ID
	}

	user, err := f.testDB.Store.CreateWaitlistUser(f.ctx, CreateWaitlistUserParams{
		CampaignID:       campaignID,
		Email:            o.Email,
		ReferralCode:     o.ReferralCode,
		Position:         o.Position,
		OriginalPosition: o.OriginalPosition,
		TermsAccepted:    o.TermsAccepted,
	})
	require.NoError(f.t, err, "failed to create test waitlist user")
	return user
}

// --- Campaign Settings Fixtures ---

// EmailSettingsOpts customizes email settings creation.
type EmailSettingsOpts struct {
	CampaignID           *uuid.UUID
	FromName             *string
	FromEmail            *string
	ReplyTo              *string
	VerificationRequired bool
	SendWelcomeEmail     bool
}

// CreateEmailSettings creates campaign email settings.
func (f *Fixtures) CreateEmailSettings(campaignID uuid.UUID, opts ...func(*EmailSettingsOpts)) CampaignEmailSettings {
	f.t.Helper()
	o := EmailSettingsOpts{CampaignID: &campaignID}
	for _, fn := range opts {
		fn(&o)
	}

	settings, err := f.testDB.Store.CreateCampaignEmailSettings(f.ctx, CreateCampaignEmailSettingsParams{
		CampaignID:           campaignID,
		FromName:             o.FromName,
		FromEmail:            o.FromEmail,
		ReplyTo:              o.ReplyTo,
		VerificationRequired: o.VerificationRequired,
		SendWelcomeEmail:     o.SendWelcomeEmail,
	})
	require.NoError(f.t, err, "failed to create email settings")
	return settings
}

// FormSettingsOpts customizes form settings creation.
type FormSettingsOpts struct {
	CaptchaEnabled  bool
	CaptchaProvider *CaptchaProvider
	CaptchaSiteKey  *string
	DoubleOptIn     bool
	Design          JSONB
	SuccessTitle    *string
	SuccessMessage  *string
}

// CreateFormSettings creates campaign form settings.
func (f *Fixtures) CreateFormSettings(campaignID uuid.UUID, opts ...func(*FormSettingsOpts)) CampaignFormSettings {
	f.t.Helper()
	o := FormSettingsOpts{Design: JSONB{}}
	for _, fn := range opts {
		fn(&o)
	}

	settings, err := f.testDB.Store.CreateCampaignFormSettings(f.ctx, CreateCampaignFormSettingsParams{
		CampaignID:      campaignID,
		CaptchaEnabled:  o.CaptchaEnabled,
		CaptchaProvider: o.CaptchaProvider,
		CaptchaSiteKey:  o.CaptchaSiteKey,
		DoubleOptIn:     o.DoubleOptIn,
		Design:          o.Design,
		SuccessTitle:    o.SuccessTitle,
		SuccessMessage:  o.SuccessMessage,
	})
	require.NoError(f.t, err, "failed to create form settings")
	return settings
}

// ReferralSettingsOpts customizes referral settings creation.
type ReferralSettingsOpts struct {
	Enabled                 bool
	PointsPerReferral       int
	VerifiedOnly            bool
	PositionsToJump         int
	ReferrerPositionsToJump int
	SharingChannels         []SharingChannel
}

// CreateReferralSettings creates campaign referral settings.
func (f *Fixtures) CreateReferralSettings(campaignID uuid.UUID, opts ...func(*ReferralSettingsOpts)) CampaignReferralSettings {
	f.t.Helper()
	o := ReferralSettingsOpts{SharingChannels: []SharingChannel{"email"}}
	for _, fn := range opts {
		fn(&o)
	}

	settings, err := f.testDB.Store.CreateCampaignReferralSettings(f.ctx, CreateCampaignReferralSettingsParams{
		CampaignID:              campaignID,
		Enabled:                 o.Enabled,
		PointsPerReferral:       o.PointsPerReferral,
		VerifiedOnly:            o.VerifiedOnly,
		PositionsToJump:         o.PositionsToJump,
		ReferrerPositionsToJump: o.ReferrerPositionsToJump,
		SharingChannels:         o.SharingChannels,
	})
	require.NoError(f.t, err, "failed to create referral settings")
	return settings
}

// BrandingSettingsOpts customizes branding settings creation.
type BrandingSettingsOpts struct {
	LogoURL      *string
	PrimaryColor *string
	FontFamily   *string
	CustomDomain *string
}

// CreateBrandingSettings creates campaign branding settings.
func (f *Fixtures) CreateBrandingSettings(campaignID uuid.UUID, opts ...func(*BrandingSettingsOpts)) CampaignBrandingSettings {
	f.t.Helper()
	o := BrandingSettingsOpts{}
	for _, fn := range opts {
		fn(&o)
	}

	settings, err := f.testDB.Store.CreateCampaignBrandingSettings(f.ctx, CreateCampaignBrandingSettingsParams{
		CampaignID:   campaignID,
		LogoURL:      o.LogoURL,
		PrimaryColor: o.PrimaryColor,
		FontFamily:   o.FontFamily,
		CustomDomain: o.CustomDomain,
	})
	require.NoError(f.t, err, "failed to create branding settings")
	return settings
}

// --- Helper Functions ---

// Ptr returns a pointer to the given value.
func Ptr[T any](v T) *T {
	return &v
}
