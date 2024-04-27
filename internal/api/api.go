package api

import (
	authHandler "base-server/internal/auth/handler"
	"net/http"

	"github.com/gin-gonic/gin"
)

type API struct {
	router      *gin.RouterGroup
	authHandler authHandler.Handler
}

func New(router *gin.RouterGroup, authHandler authHandler.Handler) API {
	return API{
		router:      router,
		authHandler: authHandler,
	}
}

func (a *API) RegisterRoutes() {
	a.Health()
	apiGroup := a.router.Group("/api")
	{
		authGroup := apiGroup.Group("/auth")
		authGroup.POST("/login/email", a.authHandler.HandleEmailLogin)
		authGroup.POST("/login/oauth", a.authHandler.HandleOAuthLogin)
		authGroup.POST("/signup/email", a.authHandler.HandleEmailSignup)
		authGroup.GET("/google/callback", a.authHandler.HandleGoogleOauthCallback)
	}
	protectedGroup := apiGroup.Group("/protected", a.authHandler.HandleJWTMiddleware)
	{
		protectedGroup.GET("/user", a.authHandler.GetUserInfo)
	}
}

func (a *API) Health() {
	a.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})
}
