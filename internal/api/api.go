package api

import (
	authHandler "base-server/internal/auth/handler"
	billingHandler "base-server/internal/billing/handler"
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
		protectedGroup.POST("pay/create-payment-intent", a.billingHandler.HandleCreatePaymentIntent)
		protectedGroup.POST("pay/create-subscription-intent", a.billingHandler.HandleCreateSubscriptionIntent)
	}
	a.router.POST("billing/webhook", a.billingHandler.HandleWebhook)
}

func (a *API) Health() {
	a.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})
}
