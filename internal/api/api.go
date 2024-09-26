package api

import (
	authHandler "base-server/internal/auth/handler"
	billingHandler "base-server/internal/money/billing/handler"
	"net/http"

	"github.com/gin-gonic/gin"
)

type API struct {
	router         *gin.RouterGroup
	authHandler    authHandler.Handler
	billingHandler billingHandler.Handler
}

func New(router *gin.RouterGroup, authHandler authHandler.Handler, handler billingHandler.Handler) API {
	return API{
		router:         router,
		authHandler:    authHandler,
		billingHandler: handler,
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
		protectedGroup.POST("billing/update-subscription", a.billingHandler.HandleUpdateSubscription)
		protectedGroup.POST("billing/cancel-subscription", a.billingHandler.HandleCancelSubscription)
		protectedGroup.POST("billing/update-payment-method", a.billingHandler.HandleUpdatePaymentMethod)
		protectedGroup.POST("billing/get-payment-method", a.billingHandler.HandleGetPaymentMethod)
	}
	apiGroup.POST("billing/webhook", a.billingHandler.HandleWebhook)
}

func (a *API) Health() {
	a.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})
}
