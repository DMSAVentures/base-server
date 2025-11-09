package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Storer defines all public methods available on the Store
type Storer interface {
	// Database
	GetDB() *sqlx.DB

	// Account operations
	CreateAccount(ctx context.Context, params CreateAccountParams) (Account, error)
	GetAccountByID(ctx context.Context, accountID uuid.UUID) (Account, error)
	GetAccountBySlug(ctx context.Context, slug string) (Account, error)
	GetAccountsByOwnerUserID(ctx context.Context, userID uuid.UUID) ([]Account, error)
	UpdateAccount(ctx context.Context, accountID uuid.UUID, params UpdateAccountParams) (Account, error)
	DeleteAccount(ctx context.Context, accountID uuid.UUID) error
	UpdateAccountStripeCustomerID(ctx context.Context, accountID uuid.UUID, stripeCustomerID string) error

	// Team Member operations
	CreateTeamMember(ctx context.Context, params CreateTeamMemberParams) (TeamMember, error)
	GetTeamMembersByAccountID(ctx context.Context, accountID uuid.UUID) ([]TeamMember, error)
	GetTeamMemberByAccountAndUserID(ctx context.Context, accountID, userID uuid.UUID) (TeamMember, error)
	UpdateTeamMemberRole(ctx context.Context, accountID, userID uuid.UUID, role string) error
	DeleteTeamMember(ctx context.Context, accountID, userID uuid.UUID) error

	// API Key operations
	CreateAPIKey(ctx context.Context, params CreateAPIKeyParams) (APIKey, error)
	GetAPIKeyByHash(ctx context.Context, keyHash string) (APIKey, error)
	GetAPIKeyByID(ctx context.Context, keyID uuid.UUID) (APIKey, error)
	GetAPIKeysByAccount(ctx context.Context, accountID uuid.UUID) ([]APIKey, error)
	UpdateAPIKeyUsage(ctx context.Context, keyID uuid.UUID) error
	RevokeAPIKey(ctx context.Context, keyID uuid.UUID, revokedBy uuid.UUID) error
	UpdateAPIKeyName(ctx context.Context, keyID uuid.UUID, name string) error

	// Audit Log operations
	CreateAuditLog(ctx context.Context, params CreateAuditLogParams) (AuditLog, error)
	GetAuditLogsByAccount(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]AuditLog, error)
	GetAuditLogsByResource(ctx context.Context, resourceType string, resourceID uuid.UUID, limit, offset int) ([]AuditLog, error)

	// Fraud Detection operations
	CreateFraudDetection(ctx context.Context, params CreateFraudDetectionParams) (FraudDetection, error)
	GetFraudDetectionsByCampaign(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]FraudDetection, error)
	GetFraudDetectionsByUser(ctx context.Context, userID uuid.UUID) ([]FraudDetection, error)
	UpdateFraudDetectionStatus(ctx context.Context, detectionID, reviewedBy uuid.UUID, status string, reviewNotes *string) error
	GetPendingFraudDetections(ctx context.Context, minConfidence float64, limit int) ([]FraudDetection, error)

	// Campaign operations
	CreateCampaign(ctx context.Context, params CreateCampaignParams) (Campaign, error)
	GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (Campaign, error)
	GetCampaignBySlug(ctx context.Context, accountID uuid.UUID, slug string) (Campaign, error)
	GetCampaignsByAccountID(ctx context.Context, accountID uuid.UUID) ([]Campaign, error)
	GetCampaignsByStatus(ctx context.Context, accountID uuid.UUID, status string) ([]Campaign, error)
	ListCampaigns(ctx context.Context, params ListCampaignsParams) (ListCampaignsResult, error)
	UpdateCampaign(ctx context.Context, accountID, campaignID uuid.UUID, params UpdateCampaignParams) (Campaign, error)
	UpdateCampaignStatus(ctx context.Context, accountID, campaignID uuid.UUID, status string) (Campaign, error)
	DeleteCampaign(ctx context.Context, accountID, campaignID uuid.UUID) error
	IncrementCampaignSignups(ctx context.Context, campaignID uuid.UUID) error
	IncrementCampaignVerified(ctx context.Context, campaignID uuid.UUID) error
	IncrementCampaignReferrals(ctx context.Context, campaignID uuid.UUID) error

	// Analytics operations
	GetAnalyticsOverview(ctx context.Context, campaignID uuid.UUID) (AnalyticsOverviewResult, error)
	GetTopReferrers(ctx context.Context, campaignID uuid.UUID, limit int) ([]TopReferrerResult, error)
	GetConversionAnalytics(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) (ConversionAnalyticsResult, error)
	GetReferralAnalytics(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) (ReferralAnalyticsResult, error)
	GetReferralSourceBreakdown(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) ([]SourceBreakdownResult, error)
	GetTimeSeriesAnalytics(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time, granularity string) ([]TimeSeriesDataPoint, error)
	GetSignupSourceBreakdown(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) ([]SourceBreakdownResult, error)
	GetUTMCampaignBreakdown(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) ([]map[string]interface{}, error)
	GetUTMSourceBreakdown(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) ([]map[string]interface{}, error)
	GetFunnelAnalytics(ctx context.Context, campaignID uuid.UUID, dateFrom, dateTo *time.Time) ([]FunnelStepResult, error)

	// Conversation operations
	GetConversation(ctx context.Context, id uuid.UUID) (*Conversation, error)
	CreateConversation(ctx context.Context, userID uuid.UUID) (*Conversation, error)
	GetAllConversationsByUserID(ctx context.Context, userID uuid.UUID) ([]Conversation, error)
	GetAllMessagesByConversationID(ctx context.Context, conversationID uuid.UUID) ([]Message, error)
	CreateMessage(ctx context.Context, conversationID uuid.UUID, role, content string) (*Message, error)
	UpdateConversationTitleByConversationID(ctx context.Context, conversationID uuid.UUID, title string) error

	// Email Auth operations
	CheckIfEmailExists(ctx context.Context, email string) (bool, error)
	CreateUserOnEmailSignup(ctx context.Context, firstName string, lastName string, email string, hashedPassword string) (User, error)
	GetCredentialsByEmail(ctx context.Context, email string) (EmailAuth, error)
	GetUserByAuthID(ctx context.Context, authID uuid.UUID) (AuthenticatedUser, error)

	// OAuth operations
	CreateUserOnGoogleSignIn(ctx context.Context, googleUserId string, email string, firstName string, lastName string) (User, error)
	GetOauthUserByEmail(ctx context.Context, email string) (OauthAuth, error)

	// Email Template operations
	CreateEmailTemplate(ctx context.Context, params CreateEmailTemplateParams) (EmailTemplate, error)
	GetEmailTemplateByID(ctx context.Context, templateID uuid.UUID) (EmailTemplate, error)
	GetEmailTemplatesByCampaign(ctx context.Context, campaignID uuid.UUID) ([]EmailTemplate, error)
	GetEmailTemplateByType(ctx context.Context, campaignID uuid.UUID, templateType string) (EmailTemplate, error)
	UpdateEmailTemplate(ctx context.Context, templateID uuid.UUID, params UpdateEmailTemplateParams) (EmailTemplate, error)
	DeleteEmailTemplate(ctx context.Context, templateID uuid.UUID) error

	// Email Log operations
	CreateEmailLog(ctx context.Context, params CreateEmailLogParams) (EmailLog, error)
	GetEmailLogByID(ctx context.Context, logID uuid.UUID) (EmailLog, error)
	GetEmailLogsByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]EmailLog, error)
	GetEmailLogsByCampaign(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]EmailLog, error)
	UpdateEmailLogStatus(ctx context.Context, logID uuid.UUID, status string) error
	IncrementEmailOpenCount(ctx context.Context, logID uuid.UUID) error
	IncrementEmailClickCount(ctx context.Context, logID uuid.UUID) error
	GetEmailLogByProviderMessageID(ctx context.Context, providerMessageID string) (EmailLog, error)

	// Payment Method operations
	CreatePaymentMethod(ctx context.Context, params CreatePaymentMethodParams) (*PaymentMethod, error)
	GetPaymentMethodByUserID(ctx context.Context, userID uuid.UUID) (*PaymentMethod, error)
	UpdatePaymentMethodByUserID(ctx context.Context, userID uuid.UUID, stripeID string, cardBrand, cardLast4 string, cardExpMonth, cardExpYear int64) error

	// Product operations
	CreateProduct(ctx context.Context, productID, name, description string) (Product, error)
	GetProductByStripeID(ctx context.Context, stripeID string) (Product, error)

	// Price operations
	CreatePrice(ctx context.Context, price Price) error
	UpdatePriceByStripeID(ctx context.Context, productID uuid.UUID, description, stripeID string) error
	DeletePriceByStripeID(ctx context.Context, stripeID string) error
	GetPriceByStripeID(ctx context.Context, stripeID string) (Price, error)
	ListPrices(ctx context.Context) ([]Price, error)
	GetPriceByID(ctx context.Context, priceID string) (Price, error)

	// Referral operations
	CreateReferral(ctx context.Context, params CreateReferralParams) (Referral, error)
	GetReferralByID(ctx context.Context, referralID uuid.UUID) (Referral, error)
	GetReferralsByReferrer(ctx context.Context, referrerID uuid.UUID) ([]Referral, error)
	GetReferralsByCampaign(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]Referral, error)
	CountReferralsByCampaign(ctx context.Context, campaignID uuid.UUID) (int, error)
	UpdateReferralStatus(ctx context.Context, referralID uuid.UUID, status string) error
	GetReferralByReferrerAndReferred(ctx context.Context, referrerID, referredID uuid.UUID) (Referral, error)
	GetVerifiedReferralCountByReferrer(ctx context.Context, referrerID uuid.UUID) (int, error)
	GetReferralsByStatus(ctx context.Context, campaignID uuid.UUID, status string) ([]Referral, error)
	GetReferralsByCampaignWithStatusFilter(ctx context.Context, campaignID uuid.UUID, status *string, limit, offset int) ([]Referral, error)
	CountReferralsByCampaignWithStatusFilter(ctx context.Context, campaignID uuid.UUID, status *string) (int, error)
	GetReferralsByReferrerWithPagination(ctx context.Context, referrerID uuid.UUID, limit, offset int) ([]Referral, error)
	CountReferralsByReferrer(ctx context.Context, referrerID uuid.UUID) (int, error)

	// Reward operations
	CreateReward(ctx context.Context, params CreateRewardParams) (Reward, error)
	GetRewardByID(ctx context.Context, rewardID uuid.UUID) (Reward, error)
	GetRewardsByCampaign(ctx context.Context, campaignID uuid.UUID) ([]Reward, error)
	GetActiveRewardsByCampaign(ctx context.Context, campaignID uuid.UUID) ([]Reward, error)
	UpdateReward(ctx context.Context, rewardID uuid.UUID, params UpdateRewardParams) (Reward, error)
	DeleteReward(ctx context.Context, rewardID uuid.UUID) error
	IncrementRewardClaimed(ctx context.Context, rewardID uuid.UUID) error

	// User Reward operations
	CreateUserReward(ctx context.Context, params CreateUserRewardParams) (UserReward, error)
	GetUserRewardsByUser(ctx context.Context, userID uuid.UUID) ([]UserReward, error)
	GetUserRewardsByCampaign(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]UserReward, error)
	UpdateUserRewardStatus(ctx context.Context, userRewardID uuid.UUID, status string) error
	IncrementDeliveryAttempts(ctx context.Context, userRewardID uuid.UUID, errorMsg string) error
	GetPendingUserRewards(ctx context.Context, limit int) ([]UserReward, error)

	// Subscription operations
	CreateSubscription(ctx context.Context, subscriptionCreated CreateSubscriptionsParams) error
	UpdateSubscription(ctx context.Context, subscriptionUpdated UpdateSubscriptionParams) error
	CancelSubscription(ctx context.Context, subscriptionID string, cancelAt time.Time) error
	GetSubscription(ctx context.Context, subscriptionID string) (Subscription, error)
	GetSubscriptionByUserID(ctx context.Context, userID uuid.UUID) (Subscription, error)

	// Usage Log operations
	InsertUsageLog(ctx context.Context, usageLog UsageLog) (UsageLog, error)
	GetUsageLogsByUserIDForPeriod(ctx context.Context, userID uuid.UUID, startDate, endDate time.Time) ([]UsageLog, error)
	UpdateUsageTokensByConversationID(ctx context.Context, conversationID uuid.UUID, delta int) error

	// User operations
	GetUserByExternalID(ctx context.Context, externalID uuid.UUID) (User, error)
	UpdateStripeCustomerIDByUserID(ctx context.Context, userID uuid.UUID, stripeCustomerID string) error
	GetStripeCustomerIDByUserExternalID(ctx context.Context, ID uuid.UUID) (string, error)
	GetUserByStripeCustomerID(ctx context.Context, stripeID string) (User, error)

	// Waitlist User operations
	CreateWaitlistUser(ctx context.Context, params CreateWaitlistUserParams) (WaitlistUser, error)
	GetWaitlistUserByID(ctx context.Context, userID uuid.UUID) (WaitlistUser, error)
	GetWaitlistUserByEmail(ctx context.Context, campaignID uuid.UUID, email string) (WaitlistUser, error)
	GetWaitlistUserByReferralCode(ctx context.Context, referralCode string) (WaitlistUser, error)
	GetWaitlistUsersByCampaign(ctx context.Context, campaignID uuid.UUID, params ListWaitlistUsersParams) ([]WaitlistUser, error)
	GetWaitlistUsersByStatus(ctx context.Context, campaignID uuid.UUID, status string) ([]WaitlistUser, error)
	CountWaitlistUsersByCampaign(ctx context.Context, campaignID uuid.UUID) (int, error)
	UpdateWaitlistUser(ctx context.Context, userID uuid.UUID, params UpdateWaitlistUserParams) (WaitlistUser, error)
	VerifyWaitlistUserEmail(ctx context.Context, userID uuid.UUID) error
	IncrementReferralCount(ctx context.Context, userID uuid.UUID) error
	IncrementVerifiedReferralCount(ctx context.Context, userID uuid.UUID) error
	UpdateWaitlistUserPosition(ctx context.Context, userID uuid.UUID, position int) error
	UpdateLastActivity(ctx context.Context, userID uuid.UUID) error
	DeleteWaitlistUser(ctx context.Context, userID uuid.UUID) error
	UpdateVerificationToken(ctx context.Context, userID uuid.UUID, token string) error
	GetWaitlistUserByVerificationToken(ctx context.Context, token string) (WaitlistUser, error)
	SearchWaitlistUsers(ctx context.Context, params SearchWaitlistUsersParams) ([]WaitlistUser, error)
	CountWaitlistUsersByStatus(ctx context.Context, campaignID uuid.UUID, status string) (int, error)
	GetWaitlistUsersByCampaignWithFilters(ctx context.Context, params ListWaitlistUsersWithFiltersParams) ([]WaitlistUser, error)
	CountWaitlistUsersWithFilters(ctx context.Context, campaignID uuid.UUID, status *string, verified *bool) (int, error)

	// Webhook operations
	CreateWebhook(ctx context.Context, params CreateWebhookParams) (Webhook, error)
	GetWebhookByID(ctx context.Context, webhookID uuid.UUID) (Webhook, error)
	GetWebhooksByAccount(ctx context.Context, accountID uuid.UUID) ([]Webhook, error)
	GetWebhooksByCampaign(ctx context.Context, campaignID uuid.UUID) ([]Webhook, error)
	UpdateWebhook(ctx context.Context, webhookID uuid.UUID, params UpdateWebhookParams) (Webhook, error)
	DeleteWebhook(ctx context.Context, webhookID uuid.UUID) error
	IncrementWebhookSent(ctx context.Context, webhookID uuid.UUID) error
	IncrementWebhookFailed(ctx context.Context, webhookID uuid.UUID) error

	// Webhook Delivery operations
	CreateWebhookDelivery(ctx context.Context, params CreateWebhookDeliveryParams) (WebhookDelivery, error)
	GetWebhookDeliveryByID(ctx context.Context, deliveryID uuid.UUID) (WebhookDelivery, error)
	GetWebhookDeliveriesByWebhook(ctx context.Context, webhookID uuid.UUID, limit, offset int) ([]WebhookDelivery, error)
	UpdateWebhookDeliveryStatus(ctx context.Context, deliveryID uuid.UUID, params UpdateWebhookDeliveryStatusParams) error
	GetPendingWebhookDeliveries(ctx context.Context, limit int) ([]WebhookDelivery, error)
	IncrementDeliveryAttempt(ctx context.Context, deliveryID uuid.UUID, nextRetryAt *time.Time) error
}
