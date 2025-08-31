package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *Handler) HandleTranscribe(c *gin.Context) {
	ctx := c.Request.Context()
	h.aiCapabilities.ConnectToWS(ctx)
	time.Sleep(40 * time.Second)
}
