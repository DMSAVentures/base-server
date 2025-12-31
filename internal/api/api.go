package api

import (
	"net/http"

	aiHandler "base-server/internal/ai-capabilities/handler"
	analyticsHandler "base-server/internal/analytics/handler"
	apikeysHandler "base-server/internal/apikeys/handler"
	authHandler "base-server/internal/auth/handler"
	campaignHandler "base-server/internal/campaign/handler"
	emailblastsHandler "base-server/internal/emailblasts/handler"
	emailTemplateHandler "base-server/internal/emailtemplates/handler"
	zapierHandler "base-server/internal/integrations/zapier"
	billingHandler "base-server/internal/money/billing/handler"
	referralHandler "base-server/internal/referral/handler"
	rewardHandler "base-server/internal/rewards/handler"
	segmentsHandler "base-server/internal/segments/handler"
	voiceCallHandler "base-server/internal/voicecall/handler"
	waitlistHandler "base-server/internal/waitlist/handler"
	webhookHandler "base-server/internal/webhooks/handler"

	"github.com/gin-gonic/gin"
)

type API struct {
	router               *gin.RouterGroup
	authHandler          authHandler.Handler
	campaignHandler      campaignHandler.Handler
	waitlistHandler      waitlistHandler.Handler
	analyticsHandler     analyticsHandler.Handler
	referralHandler      referralHandler.Handler
	rewardHandler        rewardHandler.Handler
	emailTemplateHandler emailTemplateHandler.Handler
	billingHandler       billingHandler.Handler
	aiHandler            aiHandler.Handler
	voicecallHandler     voiceCallHandler.Handler
	webhookHandler       *webhookHandler.Handler
	zapierHandler        *zapierHandler.Handler
	apikeysHandler       *apikeysHandler.Handler
	segmentsHandler      segmentsHandler.Handler
	emailblastsHandler   emailblastsHandler.Handler
}

func New(router *gin.RouterGroup, authHandler authHandler.Handler, campaignHandler campaignHandler.Handler,
	waitlistHandler waitlistHandler.Handler, analyticsHandler analyticsHandler.Handler, referralHandler referralHandler.Handler, rewardHandler rewardHandler.Handler, emailTemplateHandler emailTemplateHandler.Handler, handler billingHandler.Handler, aiHandler aiHandler.Handler, voicecallHandler voiceCallHandler.Handler, webhookHandler *webhookHandler.Handler, zapierHandler *zapierHandler.Handler, apikeysHandler *apikeysHandler.Handler, segmentsHandler segmentsHandler.Handler, emailblastsHandler emailblastsHandler.Handler) API {
	return API{
		router:               router,
		authHandler:          authHandler,
		campaignHandler:      campaignHandler,
		waitlistHandler:      waitlistHandler,
		analyticsHandler:     analyticsHandler,
		referralHandler:      referralHandler,
		rewardHandler:        rewardHandler,
		emailTemplateHandler: emailTemplateHandler,
		billingHandler:       handler,
		aiHandler:            aiHandler,
		voicecallHandler:     voicecallHandler,
		webhookHandler:       webhookHandler,
		zapierHandler:        zapierHandler,
		apikeysHandler:       apikeysHandler,
		segmentsHandler:      segmentsHandler,
		emailblastsHandler:   emailblastsHandler,
	}
}

func (a *API) RegisterRoutes() {
	a.Health()
	apiGroup := a.router.Group("/api")
	{
		authGroup := apiGroup.Group("/auth")
		authGroup.POST("/login/email", a.authHandler.HandleEmailLogin)
		authGroup.POST("/signup/email", a.authHandler.HandleEmailSignup)
		authGroup.GET("/google/callback", a.authHandler.HandleGoogleOauthCallback)
	}
	protectedGroup := apiGroup.Group("/protected", a.authHandler.HandleJWTMiddleware)
	{
		protectedGroup.GET("/user", a.authHandler.GetUserInfo)
		protectedGroup.POST("billing/create-payment-intent", a.billingHandler.HandleCreatePaymentIntent)
		protectedGroup.POST("billing/create-subscription-intent", a.billingHandler.HandleCreateSubscriptionIntent)
		protectedGroup.GET("billing/subscription", a.billingHandler.HandleGetSubscription)
		protectedGroup.POST("billing/update-subscription", a.billingHandler.HandleUpdateSubscription)
		protectedGroup.DELETE("billing/cancel-subscription", a.billingHandler.HandleCancelSubscription)
		protectedGroup.POST("billing/payment-method-update-intent", a.billingHandler.HandleUpdatePaymentMethod)
		protectedGroup.POST("billing/get-payment-method", a.billingHandler.HandleGetPaymentMethod)
		protectedGroup.POST("billing/create-customer-portal", a.billingHandler.HandleCreateCustomerPortal)
		protectedGroup.POST("billing/create-checkout-session", a.billingHandler.HandleCreateCheckoutSession)
		protectedGroup.GET("billing/checkout-session", a.billingHandler.GetCheckoutSession)
		protectedGroup.POST("ai/conversation", a.aiHandler.HandleConversation)
		protectedGroup.POST("ai/image/generate", a.aiHandler.HandleGenerateImage)

		// Webhook management routes
		webhookGroup := protectedGroup.Group("/webhooks")
		{
			webhookGroup.POST("", a.webhookHandler.HandleCreateWebhook)
			webhookGroup.GET("", a.webhookHandler.HandleListWebhooks)
			webhookGroup.GET("/:webhook_id", a.webhookHandler.HandleGetWebhook)
			webhookGroup.PUT("/:webhook_id", a.webhookHandler.HandleUpdateWebhook)
			webhookGroup.DELETE("/:webhook_id", a.webhookHandler.HandleDeleteWebhook)
			webhookGroup.GET("/:webhook_id/deliveries", a.webhookHandler.HandleListWebhookDeliveries)
			webhookGroup.POST("/:webhook_id/test", a.webhookHandler.HandleTestWebhook)
		}

		// API Keys management routes
		apiKeysGroup := protectedGroup.Group("/api-keys")
		{
			apiKeysGroup.POST("", a.apikeysHandler.HandleCreateAPIKey)
			apiKeysGroup.GET("", a.apikeysHandler.HandleListAPIKeys)
			apiKeysGroup.GET("/scopes", a.apikeysHandler.HandleGetScopes)
			apiKeysGroup.GET("/:id", a.apikeysHandler.HandleGetAPIKey)
			apiKeysGroup.PUT("/:id", a.apikeysHandler.HandleUpdateAPIKey)
			apiKeysGroup.DELETE("/:id", a.apikeysHandler.HandleRevokeAPIKey)
		}
	}

	// Campaign API routes (v1)
	v1Group := apiGroup.Group("/v1", a.authHandler.HandleJWTMiddleware)
	{
		// Email Templates routes (account-level)
		v1Group.GET("/email-templates", a.emailTemplateHandler.HandleListAllEmailTemplates)

		campaignsGroup := v1Group.Group("/campaigns")
		{
			campaignsGroup.POST("", a.campaignHandler.HandleCreateCampaign)
			campaignsGroup.GET("", a.campaignHandler.HandleListCampaigns)
			campaignsGroup.GET("/:campaign_id", a.campaignHandler.HandleGetCampaign)
			campaignsGroup.PUT("/:campaign_id", a.campaignHandler.HandleUpdateCampaign)
			campaignsGroup.DELETE("/:campaign_id", a.campaignHandler.HandleDeleteCampaign)
			campaignsGroup.PATCH("/:campaign_id/status", a.campaignHandler.HandleUpdateCampaignStatus)

			// Position calculation admin routes
			campaignsGroup.POST("/:campaign_id/positions/recalculate", a.waitlistHandler.HandleRecalculatePositions)

			// Waitlist Users routes
			usersGroup := campaignsGroup.Group("/:campaign_id/users")
			{
				// Note: POST "" (signup) is now a public endpoint - see publicV1Group below
				usersGroup.GET("", a.waitlistHandler.HandleListUsers)
				usersGroup.POST("/search", a.waitlistHandler.HandleSearchUsers)
				usersGroup.POST("/import", a.waitlistHandler.HandleImportUsers)
				usersGroup.POST("/export", a.waitlistHandler.HandleExportUsers)

				usersGroup.GET("/:user_id", a.waitlistHandler.HandleGetUser)
				usersGroup.PUT("/:user_id", a.waitlistHandler.HandleUpdateUser)
				usersGroup.DELETE("/:user_id", a.waitlistHandler.HandleDeleteUser)
				usersGroup.POST("/:user_id/verify", a.waitlistHandler.HandleVerifyUser)
				usersGroup.POST("/:user_id/resend-verification", a.waitlistHandler.HandleResendVerification)

				// User Rewards routes
				usersGroup.POST("/:user_id/rewards", a.rewardHandler.HandleGrantReward)
				usersGroup.GET("/:user_id/rewards", a.rewardHandler.HandleGetUserRewards)

				// User Referral routes
				usersGroup.GET("/:user_id/referrals", a.referralHandler.HandleGetUserReferrals)
				usersGroup.GET("/:user_id/referral-link", a.referralHandler.HandleGetReferralLink)
			}

			// Rewards routes
			rewardsGroup := campaignsGroup.Group("/:campaign_id/rewards")
			{
				rewardsGroup.POST("", a.rewardHandler.HandleCreateReward)
				rewardsGroup.GET("", a.rewardHandler.HandleListRewards)
				rewardsGroup.GET("/:reward_id", a.rewardHandler.HandleGetReward)
				rewardsGroup.PUT("/:reward_id", a.rewardHandler.HandleUpdateReward)
				rewardsGroup.DELETE("/:reward_id", a.rewardHandler.HandleDeleteReward)
			}

			// Email Templates routes
			emailTemplatesGroup := campaignsGroup.Group("/:campaign_id/email-templates")
			{
				emailTemplatesGroup.POST("", a.emailTemplateHandler.HandleCreateEmailTemplate)
				emailTemplatesGroup.GET("", a.emailTemplateHandler.HandleListEmailTemplates)
				emailTemplatesGroup.GET("/:template_id", a.emailTemplateHandler.HandleGetEmailTemplate)
				emailTemplatesGroup.PUT("/:template_id", a.emailTemplateHandler.HandleUpdateEmailTemplate)
				emailTemplatesGroup.DELETE("/:template_id", a.emailTemplateHandler.HandleDeleteEmailTemplate)
				emailTemplatesGroup.POST("/:template_id/send-test", a.emailTemplateHandler.HandleSendTestEmail)
			}

			// Analytics routes
			analyticsGroup := campaignsGroup.Group("/:campaign_id/analytics")
			{
				analyticsGroup.GET("/overview", a.analyticsHandler.HandleGetAnalyticsOverview)
				analyticsGroup.GET("/conversions", a.analyticsHandler.HandleGetConversionAnalytics)
				analyticsGroup.GET("/referrals", a.analyticsHandler.HandleGetReferralAnalytics)
				analyticsGroup.GET("/signups-over-time", a.analyticsHandler.HandleGetSignupsOverTime)
				analyticsGroup.GET("/signups-by-source", a.analyticsHandler.HandleGetSignupsBySource)
				analyticsGroup.GET("/sources", a.analyticsHandler.HandleGetSourceAnalytics)
				analyticsGroup.GET("/funnel", a.analyticsHandler.HandleGetFunnelAnalytics)
			}

			// Referral routes
			referralsGroup := campaignsGroup.Group("/:campaign_id/referrals")
			{
				referralsGroup.GET("", a.referralHandler.HandleListReferrals)
				referralsGroup.POST("/track", a.referralHandler.HandleTrackReferral)
			}

			// Segments routes
			segmentsGroup := campaignsGroup.Group("/:campaign_id/segments")
			{
				segmentsGroup.POST("", a.segmentsHandler.HandleCreateSegment)
				segmentsGroup.GET("", a.segmentsHandler.HandleListSegments)
				segmentsGroup.POST("/preview", a.segmentsHandler.HandlePreviewSegment)
				segmentsGroup.GET("/:segment_id", a.segmentsHandler.HandleGetSegment)
				segmentsGroup.PUT("/:segment_id", a.segmentsHandler.HandleUpdateSegment)
				segmentsGroup.DELETE("/:segment_id", a.segmentsHandler.HandleDeleteSegment)
				segmentsGroup.POST("/:segment_id/refresh", a.segmentsHandler.HandleRefreshSegmentCount)
			}
		}

		// Email Blasts routes (account-scoped, not campaign-nested)
		blastsGroup := v1Group.Group("/blasts")
		{
			blastsGroup.POST("", a.emailblastsHandler.HandleCreateEmailBlast)
			blastsGroup.GET("", a.emailblastsHandler.HandleListEmailBlasts)
			blastsGroup.GET("/:blast_id", a.emailblastsHandler.HandleGetEmailBlast)
			blastsGroup.PUT("/:blast_id", a.emailblastsHandler.HandleUpdateEmailBlast)
			blastsGroup.DELETE("/:blast_id", a.emailblastsHandler.HandleDeleteEmailBlast)
			blastsGroup.POST("/:blast_id/send", a.emailblastsHandler.HandleSendBlastNow)
			blastsGroup.POST("/:blast_id/schedule", a.emailblastsHandler.HandleScheduleBlast)
			blastsGroup.POST("/:blast_id/pause", a.emailblastsHandler.HandlePauseBlast)
			blastsGroup.POST("/:blast_id/resume", a.emailblastsHandler.HandleResumeBlast)
			blastsGroup.POST("/:blast_id/cancel", a.emailblastsHandler.HandleCancelBlast)
			blastsGroup.GET("/:blast_id/analytics", a.emailblastsHandler.HandleGetBlastAnalytics)
			blastsGroup.GET("/:blast_id/recipients", a.emailblastsHandler.HandleListBlastRecipients)
		}
	}

	// Public waitlist endpoints (no authentication required)
	publicV1Group := apiGroup.Group("/v1")
	{
		publicV1Group.GET("/:campaign_id", a.campaignHandler.HandleGetPublicCampaign)
		publicV1Group.POST("/campaigns/:campaign_id/users", a.waitlistHandler.HandleSignupUser)
		publicV1Group.GET("/campaigns/:campaign_id/verify", a.waitlistHandler.HandleVerifyEmail)
	}

	apiGroup.GET("billing/plans", a.billingHandler.ListPrices)
	apiGroup.POST("billing/webhook", a.billingHandler.HandleWebhook)
	apiGroup.POST("phone/answer", a.voicecallHandler.HandleAnswerPhone)
	apiGroup.GET("audio/transcribe", a.voicecallHandler.HandleVoice)               // WebSocket requires GET
	apiGroup.POST("phone/answer-agent", a.voicecallHandler.HandleAnswerVoiceAgent) // TwiML for voice agent
	apiGroup.GET("phone/voice-agent", a.voicecallHandler.HandleVoiceAgent)         // WebSocket for voice agent

	// Zapier API routes (API Key authenticated)
	zapierGroup := apiGroup.Group("/v1/zapier", a.zapierHandler.APIKeyMiddleware())
	{
		zapierGroup.GET("/me", a.zapierHandler.HandleMe)
		zapierGroup.POST("/subscribe", a.zapierHandler.HandleSubscribe)
		zapierGroup.DELETE("/subscribe/:id", a.zapierHandler.HandleUnsubscribe)
		zapierGroup.GET("/sample/:event", a.zapierHandler.HandleSampleData)
		zapierGroup.GET("/campaigns", a.zapierHandler.HandleListCampaigns)
	}

	// Integration management routes (JWT protected, for the UI)
	integrationsGroup := protectedGroup.Group("/integrations")
	{
		// Zapier management
		zapierMgmtGroup := integrationsGroup.Group("/zapier")
		{
			zapierMgmtGroup.GET("/status", a.zapierHandler.HandleStatus)
			zapierMgmtGroup.GET("/subscriptions", a.zapierHandler.HandleSubscriptions)
			zapierMgmtGroup.POST("/disconnect", a.zapierHandler.HandleDisconnect)
		}
	}
}

func (a *API) Health() {
	a.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})
}
