package store

// Account ENUMs
const (
	AccountPlanFree       = "free"
	AccountPlanStarter    = "starter"
	AccountPlanPro        = "pro"
	AccountPlanEnterprise = "enterprise"
)

const (
	AccountStatusActive    = "active"
	AccountStatusSuspended = "suspended"
	AccountStatusCanceled  = "canceled"
)

// Team Member ENUMs
const (
	TeamMemberRoleOwner  = "owner"
	TeamMemberRoleAdmin  = "admin"
	TeamMemberRoleEditor = "editor"
	TeamMemberRoleViewer = "viewer"
)

// Campaign ENUMs
const (
	CampaignStatusDraft     = "draft"
	CampaignStatusActive    = "active"
	CampaignStatusPaused    = "paused"
	CampaignStatusCompleted = "completed"
)

const (
	CampaignTypeWaitlist = "waitlist"
	CampaignTypeReferral = "referral"
	CampaignTypeContest  = "contest"
)

// Waitlist User ENUMs
const (
	WaitlistUserStatusPending   = "pending"
	WaitlistUserStatusVerified  = "verified"
	WaitlistUserStatusConverted = "converted"
	WaitlistUserStatusRemoved   = "removed"
	WaitlistUserStatusBlocked   = "blocked"
)

const (
	UserSourceDirect   = "direct"
	UserSourceReferral = "referral"
	UserSourceSocial   = "social"
	UserSourceAd       = "ad"
)

// Referral ENUMs
const (
	ReferralStatusPending   = "pending"
	ReferralStatusVerified  = "verified"
	ReferralStatusConverted = "converted"
	ReferralStatusInvalid   = "invalid"
)

const (
	ReferralSourceEmail    = "email"
	ReferralSourceTwitter  = "twitter"
	ReferralSourceFacebook = "facebook"
	ReferralSourceLinkedIn = "linkedin"
	ReferralSourceWhatsApp = "whatsapp"
	ReferralSourceDirect   = "direct"
)

// Reward ENUMs
const (
	RewardTypeEarlyAccess    = "early_access"
	RewardTypeDiscount       = "discount"
	RewardTypePremiumFeature = "premium_feature"
	RewardTypeMerchandise    = "merchandise"
	RewardTypeCustom         = "custom"
)

const (
	RewardTriggerTypeReferralCount = "referral_count"
	RewardTriggerTypePosition      = "position"
	RewardTriggerTypeMilestone     = "milestone"
	RewardTriggerTypeManual        = "manual"
)

const (
	RewardDeliveryMethodEmail   = "email"
	RewardDeliveryMethodWebhook = "webhook"
	RewardDeliveryMethodManual  = "manual"
)

const (
	RewardStatusActive  = "active"
	RewardStatusPaused  = "paused"
	RewardStatusExpired = "expired"
)

// User Reward ENUMs
const (
	UserRewardStatusPending   = "pending"
	UserRewardStatusEarned    = "earned"
	UserRewardStatusDelivered = "delivered"
	UserRewardStatusRedeemed  = "redeemed"
	UserRewardStatusRevoked   = "revoked"
	UserRewardStatusExpired   = "expired"
)

// Email Template ENUMs
const (
	EmailTemplateTypeVerification  = "verification"
	EmailTemplateTypeWelcome       = "welcome"
	EmailTemplateTypePositionUpdate = "position_update"
	EmailTemplateTypeRewardEarned  = "reward_earned"
	EmailTemplateTypeMilestone     = "milestone"
	EmailTemplateTypeCustom        = "custom"
)

// Email Log ENUMs
const (
	EmailLogStatusPending   = "pending"
	EmailLogStatusSent      = "sent"
	EmailLogStatusDelivered = "delivered"
	EmailLogStatusOpened    = "opened"
	EmailLogStatusClicked   = "clicked"
	EmailLogStatusBounced   = "bounced"
	EmailLogStatusFailed    = "failed"
)

// Webhook ENUMs
const (
	WebhookStatusActive = "active"
	WebhookStatusPaused = "paused"
	WebhookStatusFailed = "failed"
)

const (
	WebhookDeliveryStatusPending = "pending"
	WebhookDeliveryStatusSuccess = "success"
	WebhookDeliveryStatusFailed  = "failed"
)

// API Key ENUMs
const (
	APIKeyRateLimitTierStandard   = "standard"
	APIKeyRateLimitTierPro        = "pro"
	APIKeyRateLimitTierEnterprise = "enterprise"
)

const (
	APIKeyStatusActive  = "active"
	APIKeyStatusRevoked = "revoked"
)

// Audit Log ENUMs
const (
	ActorTypeUser   = "user"
	ActorTypeAPIKey = "api_key"
	ActorTypeSystem = "system"
)

// Fraud Detection ENUMs
const (
	FraudDetectionTypeSelfReferral = "self_referral"
	FraudDetectionTypeFakeEmail    = "fake_email"
	FraudDetectionTypeBot          = "bot"
	FraudDetectionTypeSuspiciousIP = "suspicious_ip"
	FraudDetectionTypeVelocity     = "velocity"
)

const (
	FraudDetectionStatusPending       = "pending"
	FraudDetectionStatusConfirmed     = "confirmed"
	FraudDetectionStatusFalsePositive = "false_positive"
	FraudDetectionStatusResolved      = "resolved"
)
