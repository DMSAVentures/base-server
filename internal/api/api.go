package api

import (
	authHandler "base-server/internal/auth/handler"

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

func (a *API) Handler() gin.IRoutes {
	authGroup := a.router.Group("/auth")
	authGroup.GET("/", a.authHandler.HandleLogin)
	return authGroup
}
