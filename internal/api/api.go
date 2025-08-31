package api

import (
	aiHandler "base-server/internal/ai-capabilities/handler"
	authHandler "base-server/internal/auth/handler"
	billingHandler "base-server/internal/money/billing/handler"
	voiceCallHandler "base-server/internal/voicecall/handler"
	"net/http"

	"github.com/gin-gonic/gin"
)

type API struct {
	router           *gin.RouterGroup
	authHandler      authHandler.Handler
	billingHandler   billingHandler.Handler
	aiHandler        aiHandler.Handler
	voicecallHandler voiceCallHandler.Handler
}

func New(router *gin.RouterGroup, authHandler authHandler.Handler, handler billingHandler.Handler,
	aiHandler aiHandler.Handler, voicecallHandler voiceCallHandler.Handler) API {
	return API{
		router:           router,
		authHandler:      authHandler,
		billingHandler:   handler,
		aiHandler:        aiHandler,
		voicecallHandler: voicecallHandler,
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
	}
	apiGroup.GET("billing/plans", a.billingHandler.ListPrices)
	apiGroup.POST("billing/webhook", a.billingHandler.HandleWebhook)
	apiGroup.POST("phone/answer", a.voicecallHandler.HandleAnswerPhone)
	apiGroup.GET("audio/transcribe", a.voicecallHandler.HandleVoice)               // WebSocket requires GET
	apiGroup.POST("phone/answer-agent", a.voicecallHandler.HandleAnswerVoiceAgent) // TwiML for voice agent
	apiGroup.GET("phone/voice-agent", a.voicecallHandler.HandleVoiceAgent)         // WebSocket for voice agent
}

func (a *API) Health() {
	a.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})
}
