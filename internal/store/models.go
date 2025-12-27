package store

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// JSONB is a custom type for JSONB fields
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("incompatible type for JSONB")
	}

	// Handle empty or null JSON
	if len(bytes) == 0 || string(bytes) == "null" {
		*j = make(JSONB)
		return nil
	}

	result := make(JSONB)
	err := json.Unmarshal(bytes, &result)
	if err != nil {
		return err
	}
	*j = result
	return nil
}

// StringArray is a custom type for PostgreSQL text[] arrays
type StringArray []string

// Value implements the driver.Valuer interface for StringArray
func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	if len(a) == 0 {
		return "{}", nil
	}
	// PostgreSQL array format: {item1,item2,item3}
	return "{" + strings.Join(a, ",") + "}", nil
}

// Scan implements the sql.Scanner interface for StringArray
func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}

	var str string
	switch v := value.(type) {
	case []byte:
		str = string(v)
	case string:
		str = v
	default:
		return fmt.Errorf("unsupported type for StringArray: %T", value)
	}

	// Handle empty array
	if str == "" || str == "{}" {
		*a = []string{}
		return nil
	}

	// Remove curly braces and split
	str = strings.Trim(str, "{}")
	if str == "" {
		*a = []string{}
		return nil
	}

	// Split by comma
	*a = strings.Split(str, ",")
	return nil
}

// ============================================================================
// Campaign Settings Types
// ============================================================================

// FormFieldType represents form field types
type FormFieldType string

const (
	FormFieldTypeEmail    FormFieldType = "email"
	FormFieldTypeText     FormFieldType = "text"
	FormFieldTypeTextarea FormFieldType = "textarea"
	FormFieldTypeSelect   FormFieldType = "select"
	FormFieldTypeCheckbox FormFieldType = "checkbox"
	FormFieldTypeRadio    FormFieldType = "radio"
	FormFieldTypePhone    FormFieldType = "phone"
	FormFieldTypeURL      FormFieldType = "url"
	FormFieldTypeDate     FormFieldType = "date"
	FormFieldTypeNumber   FormFieldType = "number"
)

// CaptchaProvider represents captcha provider types
type CaptchaProvider string

const (
	CaptchaProviderTurnstile CaptchaProvider = "turnstile"
	CaptchaProviderRecaptcha CaptchaProvider = "recaptcha"
	CaptchaProviderHCaptcha  CaptchaProvider = "hcaptcha"
)

// SharingChannel represents referral sharing channels
type SharingChannel string

const (
	SharingChannelEmail    SharingChannel = "email"
	SharingChannelTwitter  SharingChannel = "twitter"
	SharingChannelFacebook SharingChannel = "facebook"
	SharingChannelLinkedIn SharingChannel = "linkedin"
	SharingChannelWhatsApp SharingChannel = "whatsapp"
)

// TrackingIntegrationType represents tracking integration types
type TrackingIntegrationType string

const (
	TrackingIntegrationGoogleAnalytics TrackingIntegrationType = "google_analytics"
	TrackingIntegrationMetaPixel       TrackingIntegrationType = "meta_pixel"
	TrackingIntegrationGoogleAds       TrackingIntegrationType = "google_ads"
	TrackingIntegrationTikTokPixel     TrackingIntegrationType = "tiktok_pixel"
	TrackingIntegrationLinkedInInsight TrackingIntegrationType = "linkedin_insight"
)

// SharingChannelArray is a custom type for PostgreSQL sharing_channel[] arrays
type SharingChannelArray []SharingChannel

// Value implements the driver.Valuer interface for SharingChannelArray
func (a SharingChannelArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	if len(a) == 0 {
		return "{}", nil
	}
	// PostgreSQL array format: {item1,item2,item3}
	strVals := make([]string, len(a))
	for i, v := range a {
		strVals[i] = string(v)
	}
	return "{" + strings.Join(strVals, ",") + "}", nil
}

// Scan implements the sql.Scanner interface for SharingChannelArray
func (a *SharingChannelArray) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}

	var str string
	switch v := value.(type) {
	case []byte:
		str = string(v)
	case string:
		str = v
	default:
		return fmt.Errorf("unsupported type for SharingChannelArray: %T", value)
	}

	// Handle empty array
	if str == "" || str == "{}" {
		*a = []SharingChannel{}
		return nil
	}

	// Remove curly braces and split
	str = strings.Trim(str, "{}")
	if str == "" {
		*a = []SharingChannel{}
		return nil
	}

	// Split by comma and convert to SharingChannel
	parts := strings.Split(str, ",")
	*a = make([]SharingChannel, len(parts))
	for i, p := range parts {
		(*a)[i] = SharingChannel(p)
	}
	return nil
}

// Account represents a customer account
type Account struct {
	ID               uuid.UUID  `db:"id" json:"id"`
	Name             string     `db:"name" json:"name"`
	Slug             string     `db:"slug" json:"slug"`
	OwnerUserID      uuid.UUID  `db:"owner_user_id" json:"owner_user_id"`
	Plan             string     `db:"plan" json:"plan"`
	Status           string     `db:"status" json:"status"`
	StripeCustomerID *string    `db:"stripe_customer_id" json:"stripe_customer_id,omitempty"`
	TrialEndsAt      *time.Time `db:"trial_ends_at" json:"trial_ends_at,omitempty"`
	Settings         JSONB      `db:"settings" json:"settings"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt        *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// TeamMember represents a member of an account team
type TeamMember struct {
	ID          uuid.UUID  `db:"id" json:"id"`
	AccountID   uuid.UUID  `db:"account_id" json:"account_id"`
	UserID      uuid.UUID  `db:"user_id" json:"user_id"`
	Role        string     `db:"role" json:"role"`
	Permissions JSONB      `db:"permissions" json:"permissions"`
	InvitedBy   *uuid.UUID `db:"invited_by" json:"invited_by,omitempty"`
	InvitedAt   *time.Time `db:"invited_at" json:"invited_at,omitempty"`
	JoinedAt    *time.Time `db:"joined_at" json:"joined_at,omitempty"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
}

// Campaign represents a waitlist campaign
type Campaign struct {
	ID          uuid.UUID  `db:"id" json:"id"`
	AccountID   uuid.UUID  `db:"account_id" json:"account_id"`
	Name        string     `db:"name" json:"name"`
	Slug        string     `db:"slug" json:"slug"`
	Description *string    `db:"description" json:"description,omitempty"`
	Status      string     `db:"status" json:"status"`
	Type        string     `db:"type" json:"type"`
	LaunchDate  *time.Time `db:"launch_date" json:"launch_date,omitempty"`
	EndDate     *time.Time `db:"end_date" json:"end_date,omitempty"`

	PrivacyPolicyURL *string `db:"privacy_policy_url" json:"privacy_policy_url,omitempty"`
	TermsURL         *string `db:"terms_url" json:"terms_url,omitempty"`
	MaxSignups       *int    `db:"max_signups" json:"max_signups,omitempty"`

	TotalSignups   int `db:"total_signups" json:"total_signups"`
	TotalVerified  int `db:"total_verified" json:"total_verified"`
	TotalReferrals int `db:"total_referrals" json:"total_referrals"`

	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`

	// Relationships (loaded separately, not from DB)
	EmailSettings        *CampaignEmailSettings        `db:"-" json:"email_settings,omitempty"`
	BrandingSettings     *CampaignBrandingSettings     `db:"-" json:"branding_settings,omitempty"`
	FormSettings         *CampaignFormSettings         `db:"-" json:"form_settings,omitempty"`
	ReferralSettings     *CampaignReferralSettings     `db:"-" json:"referral_settings,omitempty"`
	FormFields           []CampaignFormField           `db:"-" json:"form_fields,omitempty"`
	ShareMessages        []CampaignShareMessage        `db:"-" json:"share_messages,omitempty"`
	TrackingIntegrations []CampaignTrackingIntegration `db:"-" json:"tracking_integrations,omitempty"`
}

// ============================================================================
// Campaign Settings Models (1:1 relationships)
// ============================================================================

// CampaignEmailSettings represents email configuration for a campaign
type CampaignEmailSettings struct {
	ID                   uuid.UUID `db:"id" json:"id"`
	CampaignID           uuid.UUID `db:"campaign_id" json:"campaign_id"`
	FromName             *string   `db:"from_name" json:"from_name,omitempty"`
	FromEmail            *string   `db:"from_email" json:"from_email,omitempty"`
	ReplyTo              *string   `db:"reply_to" json:"reply_to,omitempty"`
	VerificationRequired bool      `db:"verification_required" json:"verification_required"`
	SendWelcomeEmail     bool      `db:"send_welcome_email" json:"send_welcome_email"`
	CreatedAt            time.Time `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time `db:"updated_at" json:"updated_at"`
}

// CampaignBrandingSettings represents branding configuration for a campaign
type CampaignBrandingSettings struct {
	ID           uuid.UUID `db:"id" json:"id"`
	CampaignID   uuid.UUID `db:"campaign_id" json:"campaign_id"`
	LogoURL      *string   `db:"logo_url" json:"logo_url,omitempty"`
	PrimaryColor *string   `db:"primary_color" json:"primary_color,omitempty"`
	FontFamily   *string   `db:"font_family" json:"font_family,omitempty"`
	CustomDomain *string   `db:"custom_domain" json:"custom_domain,omitempty"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

// CampaignFormSettings represents form configuration for a campaign
type CampaignFormSettings struct {
	ID              uuid.UUID        `db:"id" json:"id"`
	CampaignID      uuid.UUID        `db:"campaign_id" json:"campaign_id"`
	CaptchaEnabled  bool             `db:"captcha_enabled" json:"captcha_enabled"`
	CaptchaProvider *CaptchaProvider `db:"captcha_provider" json:"captcha_provider,omitempty"`
	CaptchaSiteKey  *string          `db:"captcha_site_key" json:"captcha_site_key,omitempty"`
	DoubleOptIn     bool             `db:"double_opt_in" json:"double_opt_in"`
	Design          JSONB            `db:"design" json:"design"` // Stores layout, colors, typography, spacing, borderRadius, submitButtonText
	SuccessTitle    *string          `db:"success_title" json:"success_title,omitempty"`
	SuccessMessage  *string          `db:"success_message" json:"success_message,omitempty"`
	CreatedAt       time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time        `db:"updated_at" json:"updated_at"`
}

// CampaignReferralSettings represents referral configuration for a campaign
type CampaignReferralSettings struct {
	ID                      uuid.UUID           `db:"id" json:"id"`
	CampaignID              uuid.UUID           `db:"campaign_id" json:"campaign_id"`
	Enabled                 bool                `db:"enabled" json:"enabled"`
	PointsPerReferral       int                 `db:"points_per_referral" json:"points_per_referral"`
	VerifiedOnly            bool                `db:"verified_only" json:"verified_only"`
	PositionsToJump         int                 `db:"positions_to_jump" json:"positions_to_jump"`
	ReferrerPositionsToJump int                 `db:"referrer_positions_to_jump" json:"referrer_positions_to_jump"`
	SharingChannels         SharingChannelArray `db:"sharing_channels" json:"sharing_channels"`
	CreatedAt               time.Time           `db:"created_at" json:"created_at"`
	UpdatedAt               time.Time           `db:"updated_at" json:"updated_at"`
}

// ============================================================================
// Campaign Settings Models (1:N relationships)
// ============================================================================

// CampaignFormField represents a form field definition
type CampaignFormField struct {
	ID                uuid.UUID     `db:"id" json:"id"`
	CampaignID        uuid.UUID     `db:"campaign_id" json:"campaign_id"`
	Name              string        `db:"name" json:"name"`
	FieldType         FormFieldType `db:"field_type" json:"field_type"`
	Label             string        `db:"label" json:"label"`
	Placeholder       *string       `db:"placeholder" json:"placeholder,omitempty"`
	Required          bool          `db:"required" json:"required"`
	ValidationPattern *string       `db:"validation_pattern" json:"validation_pattern,omitempty"`
	Options           StringArray   `db:"options" json:"options,omitempty"`
	DisplayOrder      int           `db:"display_order" json:"display_order"`
	CreatedAt         time.Time     `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time     `db:"updated_at" json:"updated_at"`
}

// CampaignShareMessage represents a custom share message for a referral channel
type CampaignShareMessage struct {
	ID         uuid.UUID      `db:"id" json:"id"`
	CampaignID uuid.UUID      `db:"campaign_id" json:"campaign_id"`
	Channel    SharingChannel `db:"channel" json:"channel"`
	Message    string         `db:"message" json:"message"`
	CreatedAt  time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time      `db:"updated_at" json:"updated_at"`
}

// CampaignTrackingIntegration represents a tracking integration configuration
type CampaignTrackingIntegration struct {
	ID              uuid.UUID               `db:"id" json:"id"`
	CampaignID      uuid.UUID               `db:"campaign_id" json:"campaign_id"`
	IntegrationType TrackingIntegrationType `db:"integration_type" json:"type"`
	Enabled         bool                    `db:"enabled" json:"enabled"`
	TrackingID      string                  `db:"tracking_id" json:"tracking_id"`
	TrackingLabel   *string                 `db:"tracking_label" json:"label,omitempty"`
	CreatedAt       time.Time               `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time               `db:"updated_at" json:"updated_at"`
}

// WaitlistUser represents a user on a waitlist
type WaitlistUser struct {
	ID         uuid.UUID `db:"id" json:"id"`
	CampaignID uuid.UUID `db:"campaign_id" json:"campaign_id"`
	Email      string    `db:"email" json:"email"`
	FirstName  *string   `db:"first_name" json:"first_name,omitempty"`
	LastName   *string   `db:"last_name" json:"last_name,omitempty"`
	Status     string    `db:"status" json:"status"`

	Position         int `db:"position" json:"position"`
	OriginalPosition int `db:"original_position" json:"original_position"`

	ReferralCode          string     `db:"referral_code" json:"referral_code"`
	ReferredByID          *uuid.UUID `db:"referred_by_id" json:"referred_by_id,omitempty"`
	ReferralCount         int        `db:"referral_count" json:"referral_count"`
	VerifiedReferralCount int        `db:"verified_referral_count" json:"verified_referral_count"`
	Points                int        `db:"points" json:"points"`

	EmailVerified      bool       `db:"email_verified" json:"email_verified"`
	VerificationToken  *string    `db:"verification_token" json:"-"`
	VerificationSentAt *time.Time `db:"verification_sent_at" json:"verification_sent_at,omitempty"`
	VerifiedAt         *time.Time `db:"verified_at" json:"verified_at,omitempty"`

	Source      *string `db:"source" json:"source,omitempty"`
	UTMSource   *string `db:"utm_source" json:"utm_source,omitempty"`
	UTMMedium   *string `db:"utm_medium" json:"utm_medium,omitempty"`
	UTMCampaign *string `db:"utm_campaign" json:"utm_campaign,omitempty"`
	UTMTerm     *string `db:"utm_term" json:"utm_term,omitempty"`
	UTMContent  *string `db:"utm_content" json:"utm_content,omitempty"`

	IPAddress         *string `db:"ip_address" json:"-"`
	UserAgent         *string `db:"user_agent" json:"-"`
	CountryCode       *string `db:"country_code" json:"country_code,omitempty"`
	City              *string `db:"city" json:"city,omitempty"`
	DeviceFingerprint *string `db:"device_fingerprint" json:"-"`

	// CloudFront geographic data
	Country      *string  `db:"country" json:"country,omitempty"`
	Region       *string  `db:"region" json:"region,omitempty"`
	RegionCode   *string  `db:"region_code" json:"region_code,omitempty"`
	PostalCode   *string  `db:"postal_code" json:"postal_code,omitempty"`
	UserTimezone *string  `db:"user_timezone" json:"user_timezone,omitempty"`
	Latitude     *float64 `db:"latitude" json:"latitude,omitempty"`
	Longitude    *float64 `db:"longitude" json:"longitude,omitempty"`
	MetroCode    *string  `db:"metro_code" json:"metro_code,omitempty"`

	// CloudFront device detection (enum values: desktop, mobile, tablet, smarttv, unknown)
	DeviceType *string `db:"device_type" json:"device_type,omitempty"`
	// DeviceOS enum values: android, ios, other
	DeviceOS *string `db:"device_os" json:"device_os,omitempty"`

	Metadata JSONB `db:"metadata" json:"metadata,omitempty"`

	MarketingConsent   bool       `db:"marketing_consent" json:"marketing_consent"`
	MarketingConsentAt *time.Time `db:"marketing_consent_at" json:"marketing_consent_at,omitempty"`
	TermsAccepted      bool       `db:"terms_accepted" json:"terms_accepted"`
	TermsAcceptedAt    *time.Time `db:"terms_accepted_at" json:"terms_accepted_at,omitempty"`

	LastActivityAt *time.Time `db:"last_activity_at" json:"last_activity_at,omitempty"`
	ShareCount     int        `db:"share_count" json:"share_count"`

	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// Referral represents a referral relationship
type Referral struct {
	ID         uuid.UUID `db:"id" json:"id"`
	CampaignID uuid.UUID `db:"campaign_id" json:"campaign_id"`
	ReferrerID uuid.UUID `db:"referrer_id" json:"referrer_id"`
	ReferredID uuid.UUID `db:"referred_id" json:"referred_id"`
	Status     string    `db:"status" json:"status"`
	Source     *string   `db:"source" json:"source,omitempty"`
	IPAddress  *string   `db:"ip_address" json:"-"`

	VerifiedAt  *time.Time `db:"verified_at" json:"verified_at,omitempty"`
	ConvertedAt *time.Time `db:"converted_at" json:"converted_at,omitempty"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// Reward represents a reward definition
type Reward struct {
	ID          uuid.UUID `db:"id" json:"id"`
	CampaignID  uuid.UUID `db:"campaign_id" json:"campaign_id"`
	Name        string    `db:"name" json:"name"`
	Description *string   `db:"description" json:"description,omitempty"`
	Type        string    `db:"type" json:"type"`

	Config         JSONB  `db:"config" json:"config"`
	TriggerType    string `db:"trigger_type" json:"trigger_type"`
	TriggerConfig  JSONB  `db:"trigger_config" json:"trigger_config"`
	DeliveryMethod string `db:"delivery_method" json:"delivery_method"`
	DeliveryConfig JSONB  `db:"delivery_config" json:"delivery_config"`

	TotalAvailable *int `db:"total_available" json:"total_available,omitempty"`
	TotalClaimed   int  `db:"total_claimed" json:"total_claimed"`
	UserLimit      int  `db:"user_limit" json:"user_limit"`

	Status string `db:"status" json:"status"`

	StartsAt  *time.Time `db:"starts_at" json:"starts_at,omitempty"`
	ExpiresAt *time.Time `db:"expires_at" json:"expires_at,omitempty"`

	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// UserReward represents a reward earned by a user
type UserReward struct {
	ID         uuid.UUID `db:"id" json:"id"`
	UserID     uuid.UUID `db:"user_id" json:"user_id"`
	RewardID   uuid.UUID `db:"reward_id" json:"reward_id"`
	CampaignID uuid.UUID `db:"campaign_id" json:"campaign_id"`
	Status     string    `db:"status" json:"status"`

	RewardData JSONB `db:"reward_data" json:"reward_data"`

	EarnedAt    time.Time  `db:"earned_at" json:"earned_at"`
	DeliveredAt *time.Time `db:"delivered_at" json:"delivered_at,omitempty"`
	RedeemedAt  *time.Time `db:"redeemed_at" json:"redeemed_at,omitempty"`
	RevokedAt   *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`
	ExpiresAt   *time.Time `db:"expires_at" json:"expires_at,omitempty"`

	DeliveryAttempts      int        `db:"delivery_attempts" json:"delivery_attempts"`
	LastDeliveryAttemptAt *time.Time `db:"last_delivery_attempt_at" json:"last_delivery_attempt_at,omitempty"`
	DeliveryError         *string    `db:"delivery_error" json:"delivery_error,omitempty"`

	RevokedReason *string    `db:"revoked_reason" json:"revoked_reason,omitempty"`
	RevokedBy     *uuid.UUID `db:"revoked_by" json:"revoked_by,omitempty"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// EmailTemplate represents an email template
type EmailTemplate struct {
	ID         uuid.UUID `db:"id" json:"id"`
	CampaignID uuid.UUID `db:"campaign_id" json:"campaign_id"`
	Name       string    `db:"name" json:"name"`
	Type       string    `db:"type" json:"type"`
	Subject    string    `db:"subject" json:"subject"`

	HTMLBody   string `db:"html_body" json:"html_body"`
	BlocksJSON *JSONB `db:"blocks_json" json:"blocks_json,omitempty"`

	Enabled           bool `db:"enabled" json:"enabled"`
	SendAutomatically bool `db:"send_automatically" json:"send_automatically"`

	VariantName   *string `db:"variant_name" json:"variant_name,omitempty"`
	VariantWeight *int    `db:"variant_weight" json:"variant_weight,omitempty"`

	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// EmailLog represents an email log entry
type EmailLog struct {
	ID         uuid.UUID  `db:"id" json:"id"`
	CampaignID uuid.UUID  `db:"campaign_id" json:"campaign_id"`
	UserID     *uuid.UUID `db:"user_id" json:"user_id,omitempty"`
	TemplateID *uuid.UUID `db:"template_id" json:"template_id,omitempty"`
	BlastID    *uuid.UUID `db:"blast_id" json:"blast_id,omitempty"`

	RecipientEmail string `db:"recipient_email" json:"recipient_email"`
	Subject        string `db:"subject" json:"subject"`
	Type           string `db:"type" json:"type"`

	Status string `db:"status" json:"status"`

	ProviderMessageID *string `db:"provider_message_id" json:"provider_message_id,omitempty"`

	SentAt      *time.Time `db:"sent_at" json:"sent_at,omitempty"`
	DeliveredAt *time.Time `db:"delivered_at" json:"delivered_at,omitempty"`
	OpenedAt    *time.Time `db:"opened_at" json:"opened_at,omitempty"`
	ClickedAt   *time.Time `db:"clicked_at" json:"clicked_at,omitempty"`
	BouncedAt   *time.Time `db:"bounced_at" json:"bounced_at,omitempty"`
	FailedAt    *time.Time `db:"failed_at" json:"failed_at,omitempty"`

	ErrorMessage *string `db:"error_message" json:"error_message,omitempty"`
	BounceReason *string `db:"bounce_reason" json:"bounce_reason,omitempty"`

	OpenCount  int `db:"open_count" json:"open_count"`
	ClickCount int `db:"click_count" json:"click_count"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// CampaignAnalytics represents time-series analytics data
type CampaignAnalytics struct {
	Time       time.Time `db:"time" json:"time"`
	CampaignID uuid.UUID `db:"campaign_id" json:"campaign_id"`

	NewSignups     int `db:"new_signups" json:"new_signups"`
	NewVerified    int `db:"new_verified" json:"new_verified"`
	NewReferrals   int `db:"new_referrals" json:"new_referrals"`
	NewConversions int `db:"new_conversions" json:"new_conversions"`

	EmailsSent    int `db:"emails_sent" json:"emails_sent"`
	EmailsOpened  int `db:"emails_opened" json:"emails_opened"`
	EmailsClicked int `db:"emails_clicked" json:"emails_clicked"`

	RewardsEarned    int `db:"rewards_earned" json:"rewards_earned"`
	RewardsDelivered int `db:"rewards_delivered" json:"rewards_delivered"`

	TotalSignups   int `db:"total_signups" json:"total_signups"`
	TotalVerified  int `db:"total_verified" json:"total_verified"`
	TotalReferrals int `db:"total_referrals" json:"total_referrals"`
}

// UserActivityLog represents a user activity log entry
type UserActivityLog struct {
	ID         uuid.UUID  `db:"id" json:"id"`
	CampaignID uuid.UUID  `db:"campaign_id" json:"campaign_id"`
	UserID     *uuid.UUID `db:"user_id" json:"user_id,omitempty"`

	EventType string `db:"event_type" json:"event_type"`
	EventData JSONB  `db:"event_data" json:"event_data,omitempty"`

	IPAddress *string `db:"ip_address" json:"-"`
	UserAgent *string `db:"user_agent" json:"-"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// Webhook represents a webhook configuration
type Webhook struct {
	ID         uuid.UUID  `db:"id" json:"id"`
	AccountID  uuid.UUID  `db:"account_id" json:"account_id"`
	CampaignID *uuid.UUID `db:"campaign_id" json:"campaign_id,omitempty"`

	URL    string `db:"url" json:"url"`
	Secret string `db:"secret" json:"-"`

	Events StringArray `db:"events" json:"events"`

	Status string `db:"status" json:"status"`

	RetryEnabled bool `db:"retry_enabled" json:"retry_enabled"`
	MaxRetries   int  `db:"max_retries" json:"max_retries"`

	TotalSent     int        `db:"total_sent" json:"total_sent"`
	TotalFailed   int        `db:"total_failed" json:"total_failed"`
	LastSuccessAt *time.Time `db:"last_success_at" json:"last_success_at,omitempty"`
	LastFailureAt *time.Time `db:"last_failure_at" json:"last_failure_at,omitempty"`

	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// WebhookDelivery represents a webhook delivery attempt
type WebhookDelivery struct {
	ID        uuid.UUID `db:"id" json:"id"`
	WebhookID uuid.UUID `db:"webhook_id" json:"webhook_id"`

	EventType string `db:"event_type" json:"event_type"`
	Payload   JSONB  `db:"payload" json:"payload"`

	Status string `db:"status" json:"status"`

	RequestHeaders  JSONB   `db:"request_headers" json:"request_headers,omitempty"`
	ResponseStatus  *int    `db:"response_status" json:"response_status,omitempty"`
	ResponseBody    *string `db:"response_body" json:"response_body,omitempty"`
	ResponseHeaders JSONB   `db:"response_headers" json:"response_headers,omitempty"`

	DurationMs *int `db:"duration_ms" json:"duration_ms,omitempty"`

	AttemptNumber int        `db:"attempt_number" json:"attempt_number"`
	NextRetryAt   *time.Time `db:"next_retry_at" json:"next_retry_at,omitempty"`

	ErrorMessage *string `db:"error_message" json:"error_message,omitempty"`

	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	DeliveredAt *time.Time `db:"delivered_at" json:"delivered_at,omitempty"`
}

// APIKey represents an API key
type APIKey struct {
	ID        uuid.UUID `db:"id" json:"id"`
	AccountID uuid.UUID `db:"account_id" json:"account_id"`

	Name      string `db:"name" json:"name"`
	KeyHash   string `db:"key_hash" json:"-"`
	KeyPrefix string `db:"key_prefix" json:"key_prefix"`

	Scopes StringArray `db:"scopes" json:"scopes"`

	RateLimitTier string `db:"rate_limit_tier" json:"rate_limit_tier"`

	Status string `db:"status" json:"status"`

	LastUsedAt    *time.Time `db:"last_used_at" json:"last_used_at,omitempty"`
	TotalRequests int        `db:"total_requests" json:"total_requests"`

	ExpiresAt *time.Time `db:"expires_at" json:"expires_at,omitempty"`

	CreatedBy *uuid.UUID `db:"created_by" json:"created_by,omitempty"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	RevokedAt *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`
	RevokedBy *uuid.UUID `db:"revoked_by" json:"revoked_by,omitempty"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID        uuid.UUID  `db:"id" json:"id"`
	AccountID *uuid.UUID `db:"account_id" json:"account_id,omitempty"`

	ActorUserID     *uuid.UUID `db:"actor_user_id" json:"actor_user_id,omitempty"`
	ActorType       string     `db:"actor_type" json:"actor_type"`
	ActorIdentifier *string    `db:"actor_identifier" json:"actor_identifier,omitempty"`

	Action       string     `db:"action" json:"action"`
	ResourceType string     `db:"resource_type" json:"resource_type"`
	ResourceID   *uuid.UUID `db:"resource_id" json:"resource_id,omitempty"`

	Changes JSONB `db:"changes" json:"changes,omitempty"`

	IPAddress *string `db:"ip_address" json:"-"`
	UserAgent *string `db:"user_agent" json:"-"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// UserChannelCode represents a channel-specific referral code for a user
type UserChannelCode struct {
	ID        uuid.UUID `db:"id" json:"id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	Channel   string    `db:"channel" json:"channel"`
	Code      string    `db:"code" json:"code"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// FraudDetection represents a fraud detection record
type FraudDetection struct {
	ID         uuid.UUID  `db:"id" json:"id"`
	CampaignID uuid.UUID  `db:"campaign_id" json:"campaign_id"`
	UserID     *uuid.UUID `db:"user_id" json:"user_id,omitempty"`

	DetectionType   string  `db:"detection_type" json:"detection_type"`
	ConfidenceScore float64 `db:"confidence_score" json:"confidence_score"`

	Details JSONB `db:"details" json:"details"`

	Status string `db:"status" json:"status"`

	ReviewedBy  *uuid.UUID `db:"reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt  *time.Time `db:"reviewed_at" json:"reviewed_at,omitempty"`
	ReviewNotes *string    `db:"review_notes" json:"review_notes,omitempty"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ============================================================================
// Segments and Email Blasts
// ============================================================================

// SegmentStatus represents the status of a segment
type SegmentStatus string

const (
	SegmentStatusActive   SegmentStatus = "active"
	SegmentStatusArchived SegmentStatus = "archived"
)

// EmailBlastStatus represents the status of an email blast
type EmailBlastStatus string

const (
	EmailBlastStatusDraft      EmailBlastStatus = "draft"
	EmailBlastStatusScheduled  EmailBlastStatus = "scheduled"
	EmailBlastStatusProcessing EmailBlastStatus = "processing"
	EmailBlastStatusSending    EmailBlastStatus = "sending"
	EmailBlastStatusCompleted  EmailBlastStatus = "completed"
	EmailBlastStatusPaused     EmailBlastStatus = "paused"
	EmailBlastStatusCancelled  EmailBlastStatus = "cancelled"
	EmailBlastStatusFailed     EmailBlastStatus = "failed"
)

// BlastRecipientStatus represents the status of a blast recipient
type BlastRecipientStatus string

const (
	BlastRecipientStatusPending   BlastRecipientStatus = "pending"
	BlastRecipientStatusQueued    BlastRecipientStatus = "queued"
	BlastRecipientStatusSending   BlastRecipientStatus = "sending"
	BlastRecipientStatusSent      BlastRecipientStatus = "sent"
	BlastRecipientStatusDelivered BlastRecipientStatus = "delivered"
	BlastRecipientStatusOpened    BlastRecipientStatus = "opened"
	BlastRecipientStatusClicked   BlastRecipientStatus = "clicked"
	BlastRecipientStatusBounced   BlastRecipientStatus = "bounced"
	BlastRecipientStatusFailed    BlastRecipientStatus = "failed"
)

// SegmentFilterCriteria represents the filter criteria for a segment
type SegmentFilterCriteria struct {
	Statuses      []string          `json:"statuses,omitempty"`
	Sources       []string          `json:"sources,omitempty"`
	EmailVerified *bool             `json:"email_verified,omitempty"`
	HasReferrals  *bool             `json:"has_referrals,omitempty"`
	MinReferrals  *int              `json:"min_referrals,omitempty"`
	MinPosition   *int              `json:"min_position,omitempty"`
	MaxPosition   *int              `json:"max_position,omitempty"`
	DateFrom      *time.Time        `json:"date_from,omitempty"`
	DateTo        *time.Time        `json:"date_to,omitempty"`
	CustomFields  map[string]string `json:"custom_fields,omitempty"`
}

// Segment represents a reusable segment definition for targeting waitlist users
type Segment struct {
	ID         uuid.UUID `db:"id" json:"id"`
	CampaignID uuid.UUID `db:"campaign_id" json:"campaign_id"`

	Name        string  `db:"name" json:"name"`
	Description *string `db:"description" json:"description,omitempty"`

	FilterCriteria JSONB `db:"filter_criteria" json:"filter_criteria"`

	CachedUserCount int        `db:"cached_user_count" json:"cached_user_count"`
	CachedAt        *time.Time `db:"cached_at" json:"cached_at,omitempty"`

	Status string `db:"status" json:"status"`

	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// EmailBlast represents an email blast campaign sent to a segment
type EmailBlast struct {
	ID         uuid.UUID `db:"id" json:"id"`
	CampaignID uuid.UUID `db:"campaign_id" json:"campaign_id"`
	SegmentID  uuid.UUID `db:"segment_id" json:"segment_id"`
	TemplateID uuid.UUID `db:"template_id" json:"template_id"`

	Name    string `db:"name" json:"name"`
	Subject string `db:"subject" json:"subject"`

	ScheduledAt *time.Time `db:"scheduled_at" json:"scheduled_at,omitempty"`
	StartedAt   *time.Time `db:"started_at" json:"started_at,omitempty"`
	CompletedAt *time.Time `db:"completed_at" json:"completed_at,omitempty"`

	Status string `db:"status" json:"status"`

	TotalRecipients int `db:"total_recipients" json:"total_recipients"`
	SentCount       int `db:"sent_count" json:"sent_count"`
	DeliveredCount  int `db:"delivered_count" json:"delivered_count"`
	OpenedCount     int `db:"opened_count" json:"opened_count"`
	ClickedCount    int `db:"clicked_count" json:"clicked_count"`
	BouncedCount    int `db:"bounced_count" json:"bounced_count"`
	FailedCount     int `db:"failed_count" json:"failed_count"`

	BatchSize    int        `db:"batch_size" json:"batch_size"`
	CurrentBatch int        `db:"current_batch" json:"current_batch"`
	LastBatchAt  *time.Time `db:"last_batch_at" json:"last_batch_at,omitempty"`
	ErrorMessage *string    `db:"error_message" json:"error_message,omitempty"`

	SendThrottlePerSecond *int `db:"send_throttle_per_second" json:"send_throttle_per_second,omitempty"`

	CreatedBy *uuid.UUID `db:"created_by" json:"created_by,omitempty"`

	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// BlastRecipient represents an individual recipient within an email blast
type BlastRecipient struct {
	ID      uuid.UUID `db:"id" json:"id"`
	BlastID uuid.UUID `db:"blast_id" json:"blast_id"`
	UserID  uuid.UUID `db:"user_id" json:"user_id"`

	Email string `db:"email" json:"email"`

	Status string `db:"status" json:"status"`

	EmailLogID *uuid.UUID `db:"email_log_id" json:"email_log_id,omitempty"`

	QueuedAt    *time.Time `db:"queued_at" json:"queued_at,omitempty"`
	SentAt      *time.Time `db:"sent_at" json:"sent_at,omitempty"`
	DeliveredAt *time.Time `db:"delivered_at" json:"delivered_at,omitempty"`
	OpenedAt    *time.Time `db:"opened_at" json:"opened_at,omitempty"`
	ClickedAt   *time.Time `db:"clicked_at" json:"clicked_at,omitempty"`
	BouncedAt   *time.Time `db:"bounced_at" json:"bounced_at,omitempty"`
	FailedAt    *time.Time `db:"failed_at" json:"failed_at,omitempty"`

	ErrorMessage *string `db:"error_message" json:"error_message,omitempty"`

	BatchNumber *int `db:"batch_number" json:"batch_number,omitempty"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
